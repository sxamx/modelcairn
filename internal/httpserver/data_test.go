package httpserver

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sxamx/modelcairn/internal/adminsettings"
	"github.com/sxamx/modelcairn/internal/storage"
)

func dataHandlerFixture(t *testing.T) (http.Handler, string, *int, *storage.Installation) {
	t.Helper()
	return dataHandlerFixtureWithUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chat-1","object":"chat.completion","model":"physical","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10}}`))
	})
}

func dataHandlerFixtureWithUpstream(t *testing.T, upstreamHandler http.HandlerFunc) (http.Handler, string, *int, *storage.Installation) {
	t.Helper()
	ctx := context.Background()
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		upstreamHandler(w, r)
	}))
	t.Cleanup(upstream.Close)
	installation, err := storage.OpenInstallation(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = installation.Close() })
	settings := adminsettings.Defaults()
	settings.PublicOrigin = "http://127.0.0.1:8080"
	admin, _, err := storage.BootstrapAdmin(ctx, installation, "owner", []byte("a secure password"), settings)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := installation.Secrets().Put(ctx, storage.PutSecret{Name: "provider-secret", Value: []byte("test-value-123456")}, storage.Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	repository := storage.NewRepository(installation.DB())
	actor := storage.Actor{Type: "cli"}
	put := func(kind storage.ResourceKind, name, spec string) storage.Resource {
		t.Helper()
		item, putErr := repository.Put(ctx, storage.PutResource{Kind: kind, Name: name, Spec: json.RawMessage(spec)}, actor)
		if putErr != nil {
			t.Fatalf("put %s: %v", kind, putErr)
		}
		return item
	}
	put(storage.KindProvider, "provider", `{}`)
	put(storage.KindProviderAccount, "account", `{"providerRef":{"name":"provider"}}`)
	put(storage.KindProviderConnection, "connection", `{"providerRef":{"name":"provider"},"baseUrl":"`+upstream.URL+`","adapter":"openai-chat-v1","allowPrivateNetwork":true,"enabled":true}`)
	put(storage.KindEgress, "direct", `{"type":"direct","enabled":true}`)
	put(storage.KindCredential, "credential", `{"providerAccountRef":{"name":"account"},"egressRef":{"name":"direct"},"secretRef":{"name":"provider-secret"},"enabled":true}`)
	put(storage.KindModel, "model", `{"connectionRef":{"name":"connection"},"providerModelId":"physical","capabilities":["text","stream"],"enabled":true}`)
	put(storage.KindDestination, "destination", `{"modelRef":{"name":"model"},"credentialRef":{"name":"credential"},"weight":100,"enabled":true}`)
	put(storage.KindStrategy, "strategy", `{"destinations":[{"name":"destination"}],"maxAttempts":1,"attemptTimeoutMs":1000,"totalTimeoutMs":2000}`)
	put(storage.KindRoute, "route", `{"modelAlias":"assistant","strategyRef":{"name":"strategy"},"enabled":true}`)
	put(storage.KindAgentToken, "agent", `{"allowedRouteRefs":[{"name":"route"}],"expiresAt":null,"enabled":true}`)
	session, err := storage.CreateAdminSession(ctx, installation, storage.VerifiedAdmin{ID: admin.ID, Username: admin.Username, AuthVersion: admin.AuthVersion}, 1800, 3600, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := storage.NewAgentTokenService(installation)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := tokens.IssueSession(ctx, "agent", session.SessionToken, session.CSRFToken)
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewAdmin("127.0.0.1:0", NewReadinessProbe(), slog.New(slog.NewTextHandler(io.Discard, nil)), installation, settings)
	if err != nil {
		t.Fatal(err)
	}
	return server.Handler, issued.Token, &upstreamCalls, installation
}

func TestDataChatCompletionsStreamingVerticalPath(t *testing.T) {
	handler, token, calls, installation := dataHandlerFixtureWithUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		_, _ = w.Write([]byte("data: {\"id\":\"chat-stream-1\",\"object\":\"chat.completion.chunk\",\"model\":\"physical\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"ok\"},\"finish_reason\":null}]}\n\ndata: {\"id\":\"chat-stream-1\",\"object\":\"chat.completion.chunk\",\"model\":\"physical\",\"choices\":[],\"usage\":{\"prompt_tokens\":8,\"completion_tokens\":2,\"total_tokens\":10}}\n\ndata: [DONE]\n\n"))
	})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, dataRequest(`{"model":"assistant","messages":[{"role":"user","content":"hello"}],"stream":true}`, token))
	if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != "text/event-stream" || recorder.Header().Get("X-ModelCairn-Request-ID") == "" || *calls != 1 {
		t.Fatalf("status=%d headers=%v calls=%d body=%s", recorder.Code, recorder.Header(), *calls, recorder.Body.String())
	}
	if body := recorder.Body.String(); !strings.Contains(body, `"model":"assistant"`) || !strings.HasSuffix(body, "data: [DONE]\n\n") {
		t.Fatalf("body=%q", body)
	}
	assertRecordedRequest(t, installation, recorder.Header().Get("X-ModelCairn-Request-ID"), "success", http.StatusOK, "success")
	assertRecordedMetrics(t, installation, recorder.Header().Get("X-ModelCairn-Request-ID"), 8, 2, true)
}

func TestDataChatCompletionsPersistsCommittedStreamInterruption(t *testing.T) {
	handler, token, _, installation := dataHandlerFixtureWithUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"id\":\"chat-stream-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"partial\"},\"finish_reason\":null}]}\n\ndata: not-json\n\n"))
	})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, dataRequest(`{"model":"assistant","messages":[{"role":"user","content":"hello"}],"stream":true}`, token))
	if recorder.Code != http.StatusOK || strings.Contains(recorder.Body.String(), "modelcairn_error") || strings.Contains(recorder.Body.String(), "[DONE]") {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
	assertRecordedRequest(t, installation, recorder.Header().Get("X-ModelCairn-Request-ID"), "partial", http.StatusOK, "partial")
}

func assertRecordedRequest(t *testing.T, installation *storage.Installation, requestID, requestOutcome string, status int, attemptOutcome string) {
	t.Helper()
	var actualOutcome, actualAttempt string
	var actualStatus int
	if err := installation.DB().QueryRow("SELECT outcome,http_status FROM requests WHERE id=?", requestID).Scan(&actualOutcome, &actualStatus); err != nil {
		t.Fatal(err)
	}
	if err := installation.DB().QueryRow("SELECT outcome FROM attempts WHERE request_id=?", requestID).Scan(&actualAttempt); err != nil {
		t.Fatal(err)
	}
	if actualOutcome != requestOutcome || actualStatus != status || actualAttempt != attemptOutcome {
		t.Fatalf("request outcome=%q status=%d attempt outcome=%q", actualOutcome, actualStatus, actualAttempt)
	}
}

func dataRequest(body, token string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/v1/chat/completions", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return request
}

func TestDataChatCompletionsAuthenticatedVerticalPath(t *testing.T) {
	handler, token, calls, installation := dataHandlerFixture(t)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, dataRequest(`{"model":"assistant","messages":[{"role":"user","content":"hello"}]}`, token))
	if recorder.Code != http.StatusOK || recorder.Header().Get("X-ModelCairn-Request-ID") == "" || recorder.Header().Get("Cache-Control") != "no-store" || *calls != 1 {
		t.Fatalf("status=%d headers=%v calls=%d body=%s", recorder.Code, recorder.Header(), *calls, recorder.Body.String())
	}
	var response map[string]any
	if json.Unmarshal(recorder.Body.Bytes(), &response) != nil || response["model"] != "assistant" {
		t.Fatalf("body=%s", recorder.Body.String())
	}
	var outcome string
	var contentStored, attempts, inputTokens, outputTokens int
	requestID := recorder.Header().Get("X-ModelCairn-Request-ID")
	if err := installation.DB().QueryRow("SELECT outcome,content_stored,input_tokens,output_tokens FROM requests WHERE id=?", requestID).Scan(&outcome, &contentStored, &inputTokens, &outputTokens); err != nil {
		t.Fatal(err)
	}
	if err := installation.DB().QueryRow("SELECT count(*) FROM attempts WHERE request_id=?", requestID).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if outcome != "success" || contentStored != 0 || attempts != 1 || inputTokens != 7 || outputTokens != 3 {
		t.Fatalf("outcome=%q content=%d attempts=%d input=%d output=%d", outcome, contentStored, attempts, inputTokens, outputTokens)
	}
}

func assertRecordedMetrics(t *testing.T, installation *storage.Installation, requestID string, input, output int, requireTTFT bool) {
	t.Helper()
	var actualInput, actualOutput int
	var ttft any
	if err := installation.DB().QueryRow("SELECT input_tokens,output_tokens,ttft_ms FROM requests WHERE id=?", requestID).Scan(&actualInput, &actualOutput, &ttft); err != nil {
		t.Fatal(err)
	}
	if actualInput != input || actualOutput != output || (requireTTFT && ttft == nil) {
		t.Fatalf("input=%d output=%d ttft=%v", actualInput, actualOutput, ttft)
	}
}

func TestDataEndpointRejectsBeforeUpstream(t *testing.T) {
	handler, token, calls, _ := dataHandlerFixture(t)
	tests := []struct {
		name, body, token string
		status            int
	}{
		{"missing-token", `{"model":"assistant","messages":[{"role":"user","content":"x"}]}`, "", http.StatusUnauthorized},
		{"invalid-json", `{`, token, http.StatusBadRequest},
		{"unknown-alias-invalid-token", `{"model":"missing","messages":[{"role":"user","content":"x"}]}`, "invalid", http.StatusUnauthorized},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, dataRequest(test.body, test.token))
			if recorder.Code != test.status {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
	ambiguous := dataRequest(`{"model":"assistant","messages":[{"role":"user","content":"x"}]}`, token)
	ambiguous.Header.Add("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, ambiguous)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("ambiguous authorization status=%d", recorder.Code)
	}
	if *calls != 0 {
		t.Fatalf("upstream calls=%d", *calls)
	}
}
