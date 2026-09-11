package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/sxamx/modelcairn/internal/adminsettings"
	"github.com/sxamx/modelcairn/internal/config"
	"github.com/sxamx/modelcairn/internal/storage"
)

func TestAdminHTTPAgentTokenOneTimeLifecycle(t *testing.T) {
	ctx := context.Background()
	installation, err := storage.OpenInstallation(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer installation.Close()
	settings := adminsettings.Defaults()
	settings.PublicOrigin = "http://127.0.0.1:8080"
	if _, _, err := storage.BootstrapAdmin(ctx, installation, "owner", []byte("a secure password"), settings); err != nil {
		t.Fatal(err)
	}
	if _, err := installation.Secrets().Put(ctx, storage.PutSecret{Name: "example-key-secret", Value: []byte("provider secret value")}, storage.Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../docs/contratos/config/example-v1alpha1.yaml")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := config.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	manager := config.NewManager(installation)
	plan, err := manager.Plan(ctx, doc, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Apply(ctx, plan.Token, doc, false, storage.Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	server, err := NewAdmin("127.0.0.1:0", NewReadinessProbe(), slog.New(slog.NewTextHandler(io.Discard, nil)), installation, settings)
	if err != nil {
		t.Fatal(err)
	}
	handler := server.Handler

	login := adminRequest(http.MethodPost, "/api/v1/admin/session", []byte(`{"username":"owner","password":"a secure password"}`))
	login.Header.Set("Content-Type", "application/json")
	lr := httptest.NewRecorder()
	handler.ServeHTTP(lr, login)
	var session sessionView
	if lr.Code != http.StatusOK || json.Unmarshal(lr.Body.Bytes(), &session) != nil || len(lr.Result().Cookies()) != 1 {
		t.Fatalf("login=%d %s", lr.Code, lr.Body.String())
	}
	request := func(method, path string) *http.Request {
		r := adminRequest(method, path, nil)
		r.AddCookie(lr.Result().Cookies()[0])
		r.Header.Set("X-CSRF-Token", session.CSRFToken)
		return r
	}
	status := httptest.NewRecorder()
	handler.ServeHTTP(status, request(http.MethodGet, "/api/v1/admin/agent-tokens/example-agent/status"))
	if status.Code != http.StatusOK || !bytes.Contains(status.Body.Bytes(), []byte(`"state":"unissued"`)) {
		t.Fatalf("initial status=%d %s", status.Code, status.Body.String())
	}
	issuedResponse := httptest.NewRecorder()
	handler.ServeHTTP(issuedResponse, request(http.MethodPost, "/api/v1/admin/agent-tokens/example-agent/issue"))
	var issued storage.IssuedAgentToken
	if issuedResponse.Code != http.StatusCreated || issuedResponse.Header().Get("Cache-Control") != "no-store" || json.Unmarshal(issuedResponse.Body.Bytes(), &issued) != nil || issued.Token == "" {
		t.Fatalf("issue=%d %s", issuedResponse.Code, issuedResponse.Body.String())
	}
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, request(http.MethodPost, "/api/v1/admin/agent-tokens/example-agent/issue"))
	if second.Code != http.StatusConflict || bytes.Contains(second.Body.Bytes(), []byte(issued.Token)) {
		t.Fatalf("second issue=%d %s", second.Code, second.Body.String())
	}
	revoke := httptest.NewRecorder()
	handler.ServeHTTP(revoke, request(http.MethodPost, "/api/v1/admin/agent-tokens/example-agent/revoke"))
	if revoke.Code != http.StatusNoContent || revoke.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("revoke=%d %s", revoke.Code, revoke.Body.String())
	}
	finalStatus := httptest.NewRecorder()
	handler.ServeHTTP(finalStatus, request(http.MethodGet, "/api/v1/admin/agent-tokens/example-agent/status"))
	if finalStatus.Code != http.StatusOK || !bytes.Contains(finalStatus.Body.Bytes(), []byte(`"state":"revoked"`)) || bytes.Contains(finalStatus.Body.Bytes(), []byte(issued.Token)) {
		t.Fatalf("final status=%d %s", finalStatus.Code, finalStatus.Body.String())
	}
}
