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

func dataHandlerFixture(t *testing.T) (http.Handler, string, *int) {
	t.Helper()
	ctx := context.Background()
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chat-1","object":"chat.completion","model":"physical","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
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
	put(storage.KindModel, "model", `{"connectionRef":{"name":"connection"},"providerModelId":"physical","capabilities":["text"],"enabled":true}`)
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
	return server.Handler, issued.Token, &upstreamCalls
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
	handler, token, calls := dataHandlerFixture(t)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, dataRequest(`{"model":"assistant","messages":[{"role":"user","content":"hello"}]}`, token))
	if recorder.Code != http.StatusOK || recorder.Header().Get("X-ModelCairn-Request-ID") == "" || recorder.Header().Get("Cache-Control") != "no-store" || *calls != 1 {
		t.Fatalf("status=%d headers=%v calls=%d body=%s", recorder.Code, recorder.Header(), *calls, recorder.Body.String())
	}
	var response map[string]any
	if json.Unmarshal(recorder.Body.Bytes(), &response) != nil || response["model"] != "assistant" {
		t.Fatalf("body=%s", recorder.Body.String())
	}
}

func TestDataEndpointRejectsBeforeUpstream(t *testing.T) {
	handler, token, calls := dataHandlerFixture(t)
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
