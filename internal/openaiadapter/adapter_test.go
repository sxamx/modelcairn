package openaiadapter

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sxamx/modelcairn/internal/chatcompletions"
	"github.com/sxamx/modelcairn/internal/router"
)

type fixtureSecrets struct {
	name  string
	value []byte
}

func (s fixtureSecrets) Use(ctx context.Context, name string, callback func([]byte) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if name != s.name {
		return errors.New("secret not found")
	}
	value := append([]byte(nil), s.value...)
	defer clear(value)
	return callback(value)
}

func testRequest(t *testing.T) chatcompletions.Request {
	t.Helper()
	parsed, err := chatcompletions.Parse([]byte(`{"model":"assistant","messages":[{"role":"user","content":"hello"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	return parsed.Request
}

func testDestination(server *httptest.Server) router.Destination {
	return router.Destination{BaseURL: server.URL + "/v1/", Adapter: "openai-chat-v1", EgressType: "direct", AllowPrivateNetwork: true, ProviderModelID: "physical-model", SecretName: "provider-key"}
}

func TestExecuteRewritesModelInjectsCredentialAndRestoresAlias(t *testing.T) {
	var receivedModel, authorization, path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, authorization = r.URL.Path, r.Header.Get("Authorization")
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		receivedModel, _ = request["model"].(string)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-Request-ID", "upstream-request")
		_, _ = w.Write([]byte(`{"id":"chat-1","object":"chat.completion","model":"physical-model","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":12,"completion_tokens":4,"total_tokens":16}}`))
	}))
	defer server.Close()
	result, err := New(fixtureSecrets{name: "provider-key", value: []byte("fixture")}).Execute(context.Background(), testDestination(server), testRequest(t), "assistant")
	if err != nil {
		t.Fatal(err)
	}
	if path != "/v1/chat/completions" || receivedModel != "physical-model" || authorization != "Bearer fixture" {
		t.Fatalf("path=%q model=%q authorization=%q", path, receivedModel, authorization)
	}
	var response map[string]any
	if err := json.Unmarshal(result.Body, &response); err != nil {
		t.Fatal(err)
	}
	if result.StatusCode != http.StatusOK || result.ProviderRequestID != "upstream-request" || response["model"] != "assistant" ||
		result.InputTokens == nil || *result.InputTokens != 12 || result.OutputTokens == nil || *result.OutputTokens != 4 {
		t.Fatalf("result=%+v response=%v", result, response)
	}
}

func TestExecuteRejectsMalformedUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chat-1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":-1,"completion_tokens":2}}`))
	}))
	defer server.Close()
	_, err := New(fixtureSecrets{name: "provider-key", value: []byte("fixture")}).Execute(context.Background(), testDestination(server), chatcompletions.Request{}, "alias")
	var adapterErr *Error
	if !errors.As(err, &adapterErr) || adapterErr.Code != "invalid_response" {
		t.Fatalf("err=%v", err)
	}
}

func TestExecuteReturnsSafeNonSuccessMetadataWithoutBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "17")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"private":"upstream details"}`))
	}))
	defer server.Close()
	result, err := New(fixtureSecrets{name: "provider-key", value: []byte("x")}).Execute(context.Background(), testDestination(server), testRequest(t), "assistant")
	if err != nil {
		t.Fatal(err)
	}
	if result.StatusCode != http.StatusTooManyRequests || result.RetryAfter != "17" || result.Body != nil {
		t.Fatalf("result=%+v", result)
	}
}

func TestExecuteRejectsRedirectInvalidResponseAndPublicLoopback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			http.Redirect(w, r, "/elsewhere", http.StatusTemporaryRedirect)
			return
		}
		t.Fatal("redirect followed")
	}))
	defer server.Close()
	adapter := New(fixtureSecrets{name: "provider-key", value: []byte("x")})
	result, err := adapter.Execute(context.Background(), testDestination(server), testRequest(t), "assistant")
	if err != nil || result.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("result=%+v err=%v", result, err)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("not json"))
	}))
	defer bad.Close()
	_, err = adapter.Execute(context.Background(), testDestination(bad), testRequest(t), "assistant")
	var adapterErr *Error
	if !errors.As(err, &adapterErr) || adapterErr.Code != "invalid_response" {
		t.Fatalf("invalid response err=%v", err)
	}

	publicDestination := testDestination(bad)
	publicDestination.AllowPrivateNetwork = false
	publicDestination.BaseURL = "https://" + strings.TrimPrefix(publicDestination.BaseURL, "http://")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = adapter.Execute(ctx, publicDestination, testRequest(t), "assistant")
	if !errors.As(err, &adapterErr) || adapterErr.Code != "transport_error" || adapterErr.RequestWritten {
		t.Fatalf("public loopback err=%+v", err)
	}
}

func TestExecuteRejectsCredentialHeaderInjection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("upstream contacted") }))
	defer server.Close()
	_, err := New(fixtureSecrets{name: "provider-key", value: []byte("x\r\ny")}).Execute(context.Background(), testDestination(server), testRequest(t), "assistant")
	var adapterErr *Error
	if !errors.As(err, &adapterErr) || adapterErr.Code != "invalid_credential" {
		t.Fatalf("err=%v", err)
	}
}

func TestNormalizeResponseRejectsMalformedChoices(t *testing.T) {
	for _, body := range []string{
		`{"id":"x","object":"chat.completion","choices":null}`,
		`{"id":"x","object":"chat.completion","choices":[]}`,
		`{"id":"x","object":"chat.completion","choices":[null]}`,
		`{"id":"x","object":"chat.completion","choices":[{"index":0,"message":{"role":"user","content":"x"}}]}`,
		`{"id":"x","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant"}}]}`,
		`{"id":"x","id":"y","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"x"}}]}`,
	} {
		if _, err := normalizeResponse([]byte(body), "assistant"); err == nil {
			t.Errorf("accepted malformed response: %s", body)
		}
	}
	invalidUTF8 := []byte(`{"id":"x","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"`)
	invalidUTF8 = append(invalidUTF8, 0xff)
	invalidUTF8 = append(invalidUTF8, []byte(`"}}]}`)...)
	if _, err := normalizeResponse(invalidUTF8, "assistant"); err == nil {
		t.Error("accepted invalid UTF-8 response")
	}
}

func TestPublicHTTPDestinationIsRejectedBeforeCredentialUse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("upstream contacted") }))
	defer server.Close()
	destination := testDestination(server)
	destination.AllowPrivateNetwork = false
	_, err := New(fixtureSecrets{name: "provider-key", value: []byte("x")}).Execute(context.Background(), destination, testRequest(t), "assistant")
	var adapterErr *Error
	if !errors.As(err, &adapterErr) || adapterErr.Code != "invalid_destination" {
		t.Fatalf("err=%v", err)
	}
}

func TestOpenStreamSendsStreamingRequestAndReturnsSSEBody(t *testing.T) {
	var accept, model string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accept = r.Header.Get("Accept")
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		model, _ = request["model"].(string)
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()
	request := testRequest(t)
	request.Stream = true
	result, err := New(fixtureSecrets{name: "provider-key", value: []byte("fixture")}).OpenStream(context.Background(), testDestination(server), request)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Body.Close()
	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatal(err)
	}
	if result.StatusCode != http.StatusOK || accept != "text/event-stream" || model != "physical-model" || string(body) != "data: [DONE]\n\n" {
		t.Fatalf("result=%+v accept=%q model=%q body=%q", result, accept, model, body)
	}
}

func TestOpenStreamRejectsInvalidMediaTypeAndReturns429Metadata(t *testing.T) {
	invalid := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer invalid.Close()
	request := testRequest(t)
	request.Stream = true
	adapter := New(fixtureSecrets{name: "provider-key", value: []byte("fixture")})
	_, err := adapter.OpenStream(context.Background(), testDestination(invalid), request)
	var adapterErr *Error
	if !errors.As(err, &adapterErr) || adapterErr.Code != "invalid_response" {
		t.Fatalf("err=%v", err)
	}

	limited := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "11")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("private"))
	}))
	defer limited.Close()
	result, err := adapter.OpenStream(context.Background(), testDestination(limited), request)
	if err != nil || result.StatusCode != http.StatusTooManyRequests || result.RetryAfter != "11" || result.Body != nil {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestExecuteRejectsMalformedAssistantToolCalls(t *testing.T) {
	for _, calls := range []string{`[null]`, `[{}]`, `[{"id":"","type":"function","function":{"name":"lookup","arguments":"{}"}}]`, `[{"id":"call-1","type":"other","function":{"name":"lookup","arguments":"{}"}}]`} {
		t.Run(calls, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":"chat-1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":null,"tool_calls":` + calls + `},"finish_reason":"tool_calls"}]}`))
			}))
			defer server.Close()
			_, err := New(fixtureSecrets{name: "provider-key", value: []byte("fixture")}).Execute(context.Background(), testDestination(server), chatcompletions.Request{}, "alias")
			var adapterErr *Error
			if !errors.As(err, &adapterErr) || adapterErr.Code != "invalid_response" {
				t.Fatalf("err=%v", err)
			}
		})
	}
}
