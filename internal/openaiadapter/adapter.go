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

type Result struct {
	StatusCode        int
	ContentType       string
	RetryAfter        string
	ProviderRequestID string
	Body              []byte
}

type Error struct {
	Code           string
	RequestWritten bool
	Err            error
}

func (e *Error) Error() string { return e.Code }
func (e *Error) Unwrap() error { return e.Err }

func New(secrets SecretResolver) *Adapter {
	return &Adapter{
		secrets: secrets,
		public:  noRedirectClient(newDirectTransport(false)),
		private: noRedirectClient(newDirectTransport(true)),
	}
}

func noRedirectClient(transport http.RoundTripper) *http.Client {
	return &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func (a *Adapter) Execute(ctx context.Context, destination router.Destination, request chatcompletions.Request, requestedAlias string) (Result, error) {
	if a == nil || a.secrets == nil || destination.Adapter != "openai-chat-v1" || destination.EgressType != "direct" {
		return Result{}, &Error{Code: "unsupported_destination"}
	}
	endpoint, err := completionURL(destination.BaseURL)
	if err != nil {
		return Result{}, &Error{Code: "invalid_destination", Err: err}
	}
	request.Model = destination.ProviderModelID
	payload, err := json.Marshal(request)
	if err != nil {
		return Result{}, &Error{Code: "encode_request", Err: err}
	}
	defer clear(payload)
	upstreamRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return Result{}, &Error{Code: "invalid_destination", Err: err}
	}
	upstreamRequest.Header.Set("Content-Type", "application/json")
	upstreamRequest.Header.Set("Accept", "application/json")
	written := false
	trace := &httptrace.ClientTrace{WroteRequest: func(info httptrace.WroteRequestInfo) { written = info.Err == nil }}
	upstreamRequest = upstreamRequest.WithContext(httptrace.WithClientTrace(upstreamRequest.Context(), trace))
	client := a.public
	if destination.AllowPrivateNetwork {
		client = a.private
	}
	var result Result
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
		result.Body = normalized
		return nil
	})
	if err != nil {
		var adapterErr *Error
		if errors.As(err, &adapterErr) {
			return Result{}, adapterErr
		}
		return Result{}, &Error{Code: "credential_unavailable", Err: err}
	}
	return result, nil
}

func completionURL(base string) (*url.URL, error) {
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("invalid base URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("unsupported URL scheme")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/chat/completions"
	parsed.RawPath = ""
	return parsed, nil
}

func normalizeResponse(body []byte, requestedAlias string) ([]byte, error) {
	var object map[string]json.RawMessage
	if json.Unmarshal(body, &object) != nil || object == nil {
		return nil, errors.New("response is not an object")
	}
	var id, kind string
	var choices []json.RawMessage
	if json.Unmarshal(object["id"], &id) != nil || id == "" || json.Unmarshal(object["object"], &kind) != nil || kind != "chat.completion" || json.Unmarshal(object["choices"], &choices) != nil {
		return nil, errors.New("required response fields are invalid")
	}
	model, err := json.Marshal(requestedAlias)
	if err != nil {
		return nil, fmt.Errorf("encode response alias: %w", err)
	}
	object["model"] = model
	return json.Marshal(object)
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
