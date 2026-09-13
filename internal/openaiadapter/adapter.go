package openaiadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"

	"github.com/sxamx/modelcairn/internal/chatcompletions"
	"github.com/sxamx/modelcairn/internal/router"
)

const MaxResponseBytes = 8 << 20

type SecretResolver interface {
	Use(context.Context, string, func([]byte) error) error
}

type Adapter struct {
	secrets SecretResolver
	public  *http.Client
	private *http.Client
}

type Error struct {
	Code           string
	RequestWritten bool
	Err            error
}

func (e *Error) Error() string           { return e.Code }
func (e *Error) Unwrap() error           { return e.Err }
func (e *Error) FailureCode() string     { return e.Code }
func (e *Error) WasRequestWritten() bool { return e.RequestWritten }

func New(secrets SecretResolver) *Adapter {
	return &Adapter{
		secrets: secrets,
		public:  noRedirectClient(newDirectTransport(false)),
		private: noRedirectClient(newDirectTransport(true)),
	}
}

func (a *Adapter) CloseIdleConnections() {
	if a == nil {
		return
	}
	if a.public != nil {
		a.public.CloseIdleConnections()
	}
	if a.private != nil {
		a.private.CloseIdleConnections()
	}
}

func noRedirectClient(transport http.RoundTripper) *http.Client {
	return &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func (a *Adapter) Execute(ctx context.Context, destination router.Destination, request chatcompletions.Request, requestedAlias string) (router.UpstreamResult, error) {
	if a == nil || a.secrets == nil || destination.Adapter != "openai-chat-v1" || destination.EgressType != "direct" {
		return router.UpstreamResult{}, &Error{Code: "unsupported_destination"}
	}
	endpoint, err := completionURL(destination.BaseURL, destination.AllowPrivateNetwork)
	if err != nil {
		return router.UpstreamResult{}, &Error{Code: "invalid_destination", Err: err}
	}
	request.Model = destination.ProviderModelID
	payload, err := json.Marshal(request)
	if err != nil {
		return router.UpstreamResult{}, &Error{Code: "encode_request", Err: err}
	}
	defer clear(payload)
	upstreamRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return router.UpstreamResult{}, &Error{Code: "invalid_destination", Err: err}
	}
	upstreamRequest.Header.Set("Content-Type", "application/json")
	upstreamRequest.Header.Set("Accept", "application/json")
	written := false
	trace := &httptrace.ClientTrace{WroteRequest: func(httptrace.WroteRequestInfo) { written = true }}
	upstreamRequest = upstreamRequest.WithContext(httptrace.WithClientTrace(upstreamRequest.Context(), trace))
	client := a.public
	if destination.AllowPrivateNetwork {
		client = a.private
	}
	var result router.UpstreamResult
	err = a.secrets.Use(ctx, destination.SecretName, func(secret []byte) error {
		if len(secret) == 0 || bytes.IndexAny(secret, "\r\n") >= 0 {
			return &Error{Code: "invalid_credential"}
		}
		upstreamRequest.Header.Set("Authorization", "Bearer "+string(secret))
		response, requestErr := client.Do(upstreamRequest)
		upstreamRequest.Header.Del("Authorization")
		if requestErr != nil {
			return &Error{Code: "transport_error", RequestWritten: written, Err: requestErr}
		}
		defer response.Body.Close()
		result.StatusCode = response.StatusCode
		result.ContentType = boundedHeader(response.Header.Get("Content-Type"))
		result.RetryAfter = boundedHeader(response.Header.Get("Retry-After"))
		result.ProviderRequestID = firstHeader(response.Header, "x-request-id", "request-id")
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
			return nil
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, MaxResponseBytes+1))
		if readErr != nil {
			return &Error{Code: "read_response", RequestWritten: true, Err: readErr}
		}
		if len(body) > MaxResponseBytes {
			return &Error{Code: "response_too_large", RequestWritten: true}
		}
		mediaType, _, parseErr := mime.ParseMediaType(result.ContentType)
		if parseErr != nil || mediaType != "application/json" {
			return &Error{Code: "invalid_response", RequestWritten: true}
		}
		normalized, normalizeErr := normalizeResponse(body, requestedAlias)
		if normalizeErr != nil {
			return &Error{Code: "invalid_response", RequestWritten: true, Err: normalizeErr}
		}
		inputTokens, outputTokens, usageErr := responseUsage(body)
		if usageErr != nil {
			return &Error{Code: "invalid_response", RequestWritten: true, Err: usageErr}
		}
		result.Body = normalized
		result.InputTokens = inputTokens
		result.OutputTokens = outputTokens
		return nil
	})
	if err != nil {
		var adapterErr *Error
		if errors.As(err, &adapterErr) {
			return router.UpstreamResult{}, adapterErr
		}
		return router.UpstreamResult{}, &Error{Code: "credential_unavailable", Err: err}
	}
	return result, nil
}

func (a *Adapter) OpenStream(ctx context.Context, destination router.Destination, request chatcompletions.Request) (router.StreamUpstream, error) {
	if a == nil || a.secrets == nil || destination.Adapter != "openai-chat-v1" || destination.EgressType != "direct" || !request.Stream {
		return router.StreamUpstream{}, &Error{Code: "unsupported_destination"}
	}
	endpoint, err := completionURL(destination.BaseURL, destination.AllowPrivateNetwork)
	if err != nil {
		return router.StreamUpstream{}, &Error{Code: "invalid_destination", Err: err}
	}
	request.Model = destination.ProviderModelID
	payload, err := json.Marshal(request)
	if err != nil {
		return router.StreamUpstream{}, &Error{Code: "encode_request", Err: err}
	}
	defer clear(payload)
	upstreamRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return router.StreamUpstream{}, &Error{Code: "invalid_destination", Err: err}
	}
	upstreamRequest.Header.Set("Content-Type", "application/json")
	upstreamRequest.Header.Set("Accept", "text/event-stream")
	written := false
	upstreamRequest = upstreamRequest.WithContext(httptrace.WithClientTrace(upstreamRequest.Context(), &httptrace.ClientTrace{WroteRequest: func(httptrace.WroteRequestInfo) { written = true }}))
	client := a.public
	if destination.AllowPrivateNetwork {
		client = a.private
	}
	var result router.StreamUpstream
	err = a.secrets.Use(ctx, destination.SecretName, func(secret []byte) error {
		if len(secret) == 0 || bytes.IndexAny(secret, "\r\n") >= 0 {
			return &Error{Code: "invalid_credential"}
		}
		upstreamRequest.Header.Set("Authorization", "Bearer "+string(secret))
		response, requestErr := client.Do(upstreamRequest)
		upstreamRequest.Header.Del("Authorization")
		if requestErr != nil {
			return &Error{Code: "transport_error", RequestWritten: written, Err: requestErr}
		}
		result = router.StreamUpstream{StatusCode: response.StatusCode, ContentType: boundedHeader(response.Header.Get("Content-Type")),
			RetryAfter: boundedHeader(response.Header.Get("Retry-After")), ProviderRequestID: firstHeader(response.Header, "x-request-id", "request-id")}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			defer response.Body.Close()
			_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
			return nil
		}
		mediaType, _, parseErr := mime.ParseMediaType(result.ContentType)
		if parseErr != nil || mediaType != "text/event-stream" {
			_ = response.Body.Close()
			return &Error{Code: "invalid_response", RequestWritten: true}
		}
		result.Body = response.Body
		return nil
	})
	if err != nil {
		var adapterErr *Error
		if errors.As(err, &adapterErr) {
			return router.StreamUpstream{}, adapterErr
		}
		return router.StreamUpstream{}, &Error{Code: "credential_unavailable", Err: err}
	}
	return result, nil
}

func completionURL(base string, allowPrivate bool) (*url.URL, error) {
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("invalid base URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("unsupported URL scheme")
	}
	if parsed.Scheme != "https" && !allowPrivate {
		return nil, errors.New("public upstream requires HTTPS")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/chat/completions"
	parsed.RawPath = ""
	return parsed, nil
}

func normalizeResponse(body []byte, requestedAlias string) ([]byte, error) {
	if err := chatcompletions.ValidateJSONDocument(body); err != nil {
		return nil, errors.New("response JSON is ambiguous")
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(body, &object) != nil || object == nil {
		return nil, errors.New("response is not an object")
	}
	var id, kind string
	var choices []json.RawMessage
	if json.Unmarshal(object["id"], &id) != nil || id == "" || json.Unmarshal(object["object"], &kind) != nil || kind != "chat.completion" || json.Unmarshal(object["choices"], &choices) != nil || len(choices) == 0 {
		return nil, errors.New("required response fields are invalid")
	}
	for _, rawChoice := range choices {
		var choice struct {
			Index        *int                       `json:"index"`
			Message      map[string]json.RawMessage `json:"message"`
			FinishReason json.RawMessage            `json:"finish_reason"`
		}
		if json.Unmarshal(rawChoice, &choice) != nil || choice.Index == nil || *choice.Index < 0 || choice.Message == nil {
			return nil, errors.New("choice is invalid")
		}
		var role string
		if json.Unmarshal(choice.Message["role"], &role) != nil || role != "assistant" || !validAssistantResponse(choice.Message) {
			return nil, errors.New("choice message is invalid")
		}
		if len(choice.FinishReason) > 0 && string(choice.FinishReason) != "null" {
			var finish string
			if json.Unmarshal(choice.FinishReason, &finish) != nil {
				return nil, errors.New("finish reason is invalid")
			}
		}
	}
	model, err := json.Marshal(requestedAlias)
	if err != nil {
		return nil, fmt.Errorf("encode response alias: %w", err)
	}
	object["model"] = model
	return json.Marshal(object)
}

func validAssistantResponse(message map[string]json.RawMessage) bool {
	content, hasContent := message["content"]
	toolCalls, hasToolCalls := message["tool_calls"]
	validContent := false
	if hasContent {
		if string(content) == "null" {
			validContent = true
		} else {
			var text string
			validContent = json.Unmarshal(content, &text) == nil
		}
	}
	validTools := false
	if hasToolCalls {
		var calls []struct {
			ID       string `json:"id"`
			Type     string `json:"type"`
			Function struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			} `json:"function"`
		}
		validTools = json.Unmarshal(toolCalls, &calls) == nil && len(calls) > 0
		for _, call := range calls {
			if call.ID == "" || call.Type != "function" || call.Function.Name == "" || len(call.Function.Name) > 64 {
				validTools = false
				break
			}
		}
	}
	if hasContent && !validContent {
		return false
	}
	if hasToolCalls && !validTools {
		return false
	}
	return hasContent || hasToolCalls
}

func responseUsage(body []byte) (*int, *int, error) {
	var object map[string]json.RawMessage
	if json.Unmarshal(body, &object) != nil {
		return nil, nil, errors.New("response is not an object")
	}
	raw, exists := object["usage"]
	if !exists || string(raw) == "null" {
		return nil, nil, nil
	}
	var usage struct {
		PromptTokens     *int `json:"prompt_tokens"`
		CompletionTokens *int `json:"completion_tokens"`
	}
	if json.Unmarshal(raw, &usage) != nil || usage.PromptTokens == nil || usage.CompletionTokens == nil ||
		*usage.PromptTokens < 0 || *usage.CompletionTokens < 0 {
		return nil, nil, errors.New("usage is invalid")
	}
	return usage.PromptTokens, usage.CompletionTokens, nil
}

func firstHeader(header http.Header, names ...string) string {
	for _, name := range names {
		if value := header.Get(name); value != "" {
			return boundedHeader(value)
		}
	}
	return ""
}

func boundedHeader(value string) string {
	if len(value) > 256 {
		return value[:256]
	}
	return value
}
