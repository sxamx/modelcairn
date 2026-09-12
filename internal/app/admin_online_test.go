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
