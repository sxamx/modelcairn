package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestOnlineAdminLoginWhoamiLogout(t *testing.T) {
	const sessionToken = "session-token-must-not-be-printed"
	var origin string
	var mu sync.Mutex
	csrf := "initial-csrf"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != origin {
			t.Errorf("origin=%q", r.Header.Get("Origin"))
			w.WriteHeader(403)
			return
		}
		switch r.URL.Path {
		case "/api/v1/admin/session":
			if r.Method == http.MethodPost {
				var login map[string]string
				if json.NewDecoder(r.Body).Decode(&login) != nil || login["username"] != "owner" || login["password"] != "a secure password" {
					w.WriteHeader(400)
					return
				}
				http.SetCookie(w, &http.Cookie{Name: "mc_session", Value: sessionToken, Path: "/", HttpOnly: true})
				_ = json.NewEncoder(w).Encode(map[string]any{"admin": map[string]string{"id": "admin-id", "username": "owner"}, "csrfToken": csrf, "expiresAt": time.Now().Add(time.Hour)})
				return
			}
			if r.Method == http.MethodDelete {
				if cookie, err := r.Cookie("mc_session"); err != nil || cookie.Value != sessionToken || r.Header.Get("X-CSRF-Token") != "rotated-csrf" {
					w.WriteHeader(401)
					return
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
		case "/api/v1/admin/session/me":
			if cookie, err := r.Cookie("mc_session"); err != nil || cookie.Value != sessionToken {
				w.WriteHeader(401)
				return
			}
			mu.Lock()
			csrf = "rotated-csrf"
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"admin": map[string]string{"id": "admin-id", "username": "owner"}, "csrfToken": "rotated-csrf", "expiresAt": time.Now().Add(time.Hour)})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	origin = server.URL
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	base := []string{"--server", origin, "--session-file", sessionFile}
	loginArgs := append([]string{"admin", "login", "--username", "owner"}, base...)
	code, out, errOut := runCLI(t, loginArgs, "a secure password\n", false)
	if code != 0 || !strings.Contains(out, "authenticated as owner") || strings.Contains(out+errOut, sessionToken) {
		t.Fatalf("login code=%d out=%q err=%q", code, out, errOut)
	}
	if info, err := os.Stat(sessionFile); err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0600) {
		t.Fatalf("session file info=%v err=%v", info, err)
	}
	code, out, errOut = runCLI(t, append([]string{"admin", "whoami"}, base...), "", false)
	if code != 0 || !strings.Contains(out, `"username": "owner"`) || strings.Contains(out+errOut, sessionToken) {
		t.Fatalf("whoami code=%d out=%q err=%q", code, out, errOut)
	}
	code, out, errOut = runCLI(t, append([]string{"admin", "logout"}, base...), "", false)
	if code != 0 || !strings.Contains(out, "revoked") || strings.Contains(out+errOut, sessionToken) {
		t.Fatalf("logout code=%d out=%q err=%q", code, out, errOut)
	}
	if _, err := os.Stat(sessionFile); !os.IsNotExist(err) {
		t.Fatalf("session file remains: %v", err)
	}
}

func TestAdminClientRejectsRedirectsAndRemotePlainHTTP(t *testing.T) {
	if _, err := newAdminClient("http://example.test"); err == nil {
		t.Fatal("accepted remote plaintext HTTP")
	}
	targetCalled := false
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { targetCalled = true }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	client, err := newAdminClient(redirect.URL)
	if err != nil {
		t.Fatal(err)
	}
	response, _, err := client.authenticated(t.Context(), onlineSession{SessionToken: "secret", CSRFToken: "csrf"}, http.MethodPost, "/redirect", []byte("sensitive"), "application/json")
	if err != nil || response.StatusCode != http.StatusTemporaryRedirect || targetCalled {
		t.Fatalf("response=%v targetCalled=%v err=%v", response, targetCalled, err)
	}
}

func TestAgentTokenCLIRecoversCSRFOnce(t *testing.T) {
	const bearer = "mc_at_v1_abcdefghijklmnopqrstuvwxyzABCDEFGH123456789"
	var origin string
	issues := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != origin {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		switch r.URL.Path {
		case "/api/v1/admin/session/me":
			_ = json.NewEncoder(w).Encode(map[string]any{"admin": map[string]string{"id": "admin-id", "username": "owner"}, "csrfToken": "fresh-csrf", "expiresAt": time.Now().Add(time.Hour)})
		case "/api/v1/admin/agent-tokens/agent/issue":
			issues++
			if r.Header.Get("X-CSRF-Token") != "fresh-csrf" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"token": bearer, "tokenStatus": map[string]string{"state": "active"}})
		case "/api/v1/admin/agent-tokens/agent/status":
			_ = json.NewEncoder(w).Encode(map[string]any{"tokenStatus": map[string]string{"state": "active"}})
		case "/api/v1/admin/agent-tokens/agent/revoke":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	origin = server.URL
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveOnlineSession(sessionFile, onlineSession{Server: origin, SessionToken: "session", CSRFToken: "stale-csrf", ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	base := []string{"--server", origin, "--session-file", sessionFile, "agent"}
	code, out, errOut := runCLI(t, append([]string{"agent-token", "issue"}, base...), "", false)
	if code != 0 || !strings.Contains(out, bearer) || errOut != "" || issues != 2 {
		t.Fatalf("issue code=%d issues=%d out=%q err=%q", code, issues, out, errOut)
	}
	code, out, errOut = runCLI(t, append([]string{"agent-token", "status"}, base...), "", false)
	if code != 0 || !strings.Contains(out, `"state": "active"`) || strings.Contains(errOut, bearer) {
		t.Fatalf("status code=%d out=%q err=%q", code, out, errOut)
	}
	code, out, errOut = runCLI(t, append([]string{"agent-token", "revoke"}, base...), "", false)
	if code != 0 || out != "agent token revoked\n" || strings.Contains(errOut, bearer) {
		t.Fatalf("revoke code=%d out=%q err=%q", code, out, errOut)
	}
}

func TestOnlineConfigurationPlanApplyExport(t *testing.T) {
	const token = "authenticated-plan-token"
	var origin string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != origin {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		switch r.URL.Path {
		case "/api/v1/admin/config/plan":
			if r.URL.Query().Get("allowDelete") != "false" {
				w.WriteHeader(400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"valid": true, "changes": []any{map[string]string{"operation": "create", "kind": "Provider", "name": "acme"}}, "planToken": token, "expiresAt": time.Now().Add(time.Minute)})
		case "/api/v1/admin/config/apply":
			if r.Header.Get("X-ModelCairn-Plan-Token") != token {
				w.WriteHeader(400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"applied": true, "changes": []any{}, "appliedAt": time.Now()})
		case "/api/v1/admin/config/export":
			_, _ = w.Write([]byte(`{"apiVersion":"modelcairn.io/v1alpha1","kind":"Configuration","resources":[{"kind":"Provider","state":"present","metadata":{"name":"acme"},"spec":{}}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	origin = server.URL
	directory := t.TempDir()
	sessionFile, configFile, planFile := filepath.Join(directory, "session.json"), filepath.Join(directory, "config.yaml"), filepath.Join(directory, "plan.json")
	if err := saveOnlineSession(sessionFile, onlineSession{Server: origin, SessionToken: "session", CSRFToken: "csrf", ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	configuration := "apiVersion: modelcairn.io/v1alpha1\nkind: Configuration\nresources:\n  - kind: Provider\n    metadata: {name: acme}\n    spec: {}\n"
	if err := os.WriteFile(configFile, []byte(configuration), 0600); err != nil {
		t.Fatal(err)
	}
	base := []string{"--server", origin, "--session-file", sessionFile}
	planArgs := append([]string{"config", "plan"}, base...)
	planArgs = append(planArgs, "--out", planFile, configFile)
	code, out, errOut := runCLI(t, planArgs, "", false)
	if code != 0 || !strings.Contains(out, "plan written") || errOut != "" {
		t.Fatalf("plan code=%d out=%q err=%q", code, out, errOut)
	}
	applyArgs := append([]string{"config", "apply"}, base...)
	applyArgs = append(applyArgs, "--plan", planFile, configFile)
	code, out, errOut = runCLI(t, applyArgs, "", false)
	if code != 0 || out != "configuration applied\n" || errOut != "" {
		t.Fatalf("apply code=%d out=%q err=%q", code, out, errOut)
	}
	exportArgs := append([]string{"config", "export"}, base...)
	code, out, errOut = runCLI(t, exportArgs, "", false)
	if code != 0 || !strings.Contains(out, `"name":"acme"`) || strings.Contains(out+errOut, token) {
		t.Fatalf("export code=%d out=%q err=%q", code, out, errOut)
	}
}

func TestOnlineSecretLifecycleDoesNotDiscloseValue(t *testing.T) {
	const secretValue = "provider-secret-value"
	var origin string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != origin {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if r.URL.Path == "/api/v1/admin/secrets" {
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{map[string]any{"name": "provider-key", "fingerprint": "sha256:test", "resourceVersion": 1, "updatedAt": time.Now()}}, "nextCursor": nil})
			return
		}
		if r.URL.Path != "/api/v1/admin/secrets/provider-key" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		switch r.Method {
		case http.MethodPut:
			var body map[string]string
			if json.NewDecoder(r.Body).Decode(&body) != nil || body["value"] != secretValue {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"name": "provider-key", "fingerprint": "sha256:test", "resourceVersion": 1, "updatedAt": time.Now()})
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"name": "provider-key", "fingerprint": "sha256:test", "resourceVersion": 1, "updatedAt": time.Now()})
		case http.MethodDelete:
			if r.Header.Get("If-Match") != `"1"` {
				w.WriteHeader(http.StatusPreconditionRequired)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()
	origin = server.URL
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveOnlineSession(sessionFile, onlineSession{Server: origin, SessionToken: "session", CSRFToken: "csrf", ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	base := []string{"--server", origin, "--session-file", sessionFile}
	setArgs := append([]string{"secret", "set"}, base...)
	setArgs = append(setArgs, "provider-key")
	code, out, errOut := runCLI(t, setArgs, secretValue+"\n", false)
	if code != 0 || strings.Contains(out+errOut, secretValue) || !strings.Contains(out, `"name": "provider-key"`) {
		t.Fatalf("set code=%d out=%q err=%q", code, out, errOut)
	}
	metadataArgs := append([]string{"secret", "metadata"}, base...)
	metadataArgs = append(metadataArgs, "provider-key")
	code, out, errOut = runCLI(t, metadataArgs, "", false)
	if code != 0 || strings.Contains(out+errOut, secretValue) || !strings.Contains(out, `"resourceVersion": 1`) {
		t.Fatalf("metadata code=%d out=%q err=%q", code, out, errOut)
	}
	listArgs := append([]string{"secret", "metadata"}, base...)
	code, out, errOut = runCLI(t, listArgs, "", false)
	if code != 0 || strings.Contains(out+errOut, secretValue) || !strings.Contains(out, "provider-key") {
		t.Fatalf("list code=%d out=%q err=%q", code, out, errOut)
	}
	deleteArgs := append([]string{"secret", "delete"}, base...)
	deleteArgs = append(deleteArgs, "--version", "1", "provider-key")
	code, out, errOut = runCLI(t, deleteArgs, "", false)
	if code != 0 || out != "secret deleted\n" || strings.Contains(out+errOut, secretValue) {
		t.Fatalf("delete code=%d out=%q err=%q", code, out, errOut)
	}
}
