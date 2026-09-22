package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestActivityRejectsMalformedQueryBeforeStorage(t *testing.T) {
	handler := adminHandler(t)
	login := adminRequest(http.MethodPost, "/api/v1/admin/session", []byte(`{"username":"owner","password":"a secure password"}`))
	login.Header.Set("Content-Type", "application/json")
	lr := httptest.NewRecorder()
	handler.ServeHTTP(lr, login)
	var session sessionView
	decodeJSONBody(t, lr, &session)
	for _, path := range []string{"/api/v1/admin/requests?limit=201", "/api/v1/admin/requests?cursor=bad!", "/api/v1/admin/requests?from=2026-09-14T00:00:00Z&to=2026-09-13T00:00:00Z", "/api/v1/admin/requests?unknown=x", "/api/v1/admin/requests?outcome=unknown", "/api/v1/admin/requests?alias=" + strings.Repeat("x", 129), "/api/v1/admin/requests/" + strings.Repeat("x", 129) + "/attempts"} {
		r := adminRequest(http.MethodGet, path, nil)
		r.AddCookie(lr.Result().Cookies()[0])
		r.Header.Set("X-CSRF-Token", session.CSRFToken)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("GET %s = %d", path, w.Code)
		}
	}
}

func TestMetricsRequiresSessionAndReturnsEmptyAggregate(t *testing.T) {
	handler := adminHandler(t)
	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, adminRequest(http.MethodGet, "/api/v1/admin/metrics", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized metrics status=%d", unauthorized.Code)
	}
	login := adminRequest(http.MethodPost, "/api/v1/admin/session", []byte(`{"username":"owner","password":"a secure password"}`))
	login.Header.Set("Content-Type", "application/json")
	lr := httptest.NewRecorder()
	handler.ServeHTTP(lr, login)
	var session sessionView
	decodeJSONBody(t, lr, &session)
	request := adminRequest(http.MethodGet, "/api/v1/admin/metrics", nil)
	request.AddCookie(lr.Result().Cookies()[0])
	request.Header.Set("X-CSRF-Token", session.CSRFToken)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("metrics status=%d body=%s", response.Code, response.Body.String())
	}
	var metrics struct {
		Requests, InputTokens, OutputTokens int64
		Daily, Models                       []any
	}
	decodeJSONBody(t, response, &metrics)
	if metrics.Requests != 0 || metrics.InputTokens != 0 || metrics.OutputTokens != 0 || metrics.Daily == nil || metrics.Models == nil {
		t.Fatalf("unexpected empty metrics: %+v", metrics)
	}
}

func decodeJSONBody(t *testing.T, w *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), target); err != nil {
		t.Fatal(err)
	}
}
