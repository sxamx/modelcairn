package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOverviewRequiresAdministrativeSession(t *testing.T) {
	recorder := httptest.NewRecorder()
	adminHandler(t).ServeHTTP(recorder, adminRequest(http.MethodGet, "/api/v1/admin/overview", nil))
	if recorder.Code != http.StatusUnauthorized || !strings.Contains(recorder.Body.String(), "authentication_required") {
		t.Fatalf("overview without session = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestOverviewReturnsBoundedContentFreeShape(t *testing.T) {
	handler := adminHandler(t)
	login := adminRequest(http.MethodPost, "/api/v1/admin/session", []byte(`{"username":"owner","password":"a secure password"}`))
	login.Header.Set("Content-Type", "application/json")
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, login)
	var session sessionView
	if loginResponse.Code != http.StatusOK || json.Unmarshal(loginResponse.Body.Bytes(), &session) != nil {
		t.Fatalf("login=%d %s", loginResponse.Code, loginResponse.Body.String())
	}
	request := adminRequest(http.MethodGet, "/api/v1/admin/overview", nil)
	request.AddCookie(loginResponse.Result().Cookies()[0])
	request.Header.Set("X-CSRF-Token", session.CSRFToken)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("overview=%d %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if json.Unmarshal(recorder.Body.Bytes(), &body) != nil || body["recentRequests"] == nil || body["resourceCounts"] == nil {
		t.Fatalf("overview shape=%s", recorder.Body.String())
	}
	for _, forbidden := range []string{"prompt", "response", "secret", "content"} {
		if strings.Contains(strings.ToLower(recorder.Body.String()), forbidden) {
			t.Fatalf("overview contains forbidden field %q: %s", forbidden, recorder.Body.String())
		}
	}
}
