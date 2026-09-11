package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sxamx/modelcairn/internal/adminsettings"
	"github.com/sxamx/modelcairn/internal/storage"
)

func adminHandler(t *testing.T) http.Handler {
	t.Helper()
	i, err := storage.OpenInstallation(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = i.Close() })
	spec := adminsettings.Defaults()
	spec.PublicOrigin = "http://127.0.0.1:8080"
	if _, _, err := storage.BootstrapAdmin(context.Background(), i, "owner", []byte("a secure password"), spec); err != nil {
		t.Fatal(err)
	}
	server, err := NewAdmin("127.0.0.1:0", NewReadinessProbe(), slog.New(slog.NewTextHandler(io.Discard, nil)), i, spec)
	if err != nil {
		t.Fatal(err)
	}
	return server.Handler
}

func adminRequest(method, path string, body []byte) *http.Request {
	r := httptest.NewRequest(method, "http://127.0.0.1:8080"+path, bytes.NewReader(body))
	r.RemoteAddr = "127.0.0.1:4567"
	r.Header.Set("Origin", "http://127.0.0.1:8080")
	return r
}

func TestAdminHTTPLoginSettingsAndLogout(t *testing.T) {
	handler := adminHandler(t)
	login := adminRequest(http.MethodPost, "/api/v1/admin/session", []byte(`{"username":"owner","password":"a secure password"}`))
	login.Header.Set("Content-Type", "application/json")
	lr := httptest.NewRecorder()
	handler.ServeHTTP(lr, login)
	if lr.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", lr.Code, lr.Body.String())
	}
	var session sessionView
	if err := json.Unmarshal(lr.Body.Bytes(), &session); err != nil || session.CSRFToken == "" || session.Admin.Username != "owner" {
		t.Fatalf("login response=%+v err=%v", session, err)
	}
	response := lr.Result()
	cookies := response.Cookies()
	if len(cookies) == 1 && (!cookies[0].Expires.IsZero() || cookies[0].MaxAge != 0) {
		t.Fatal("cookie imposes a fixed idle deadline")
	}
	if len(cookies) != 1 || cookies[0].Name != "mc_session" || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode || cookies[0].Secure {
		t.Fatalf("invalid session cookie: %+v", cookies)
	}
	request := func(method, path string, body []byte) *http.Request {
		r := adminRequest(method, path, body)
		r.AddCookie(cookies[0])
		r.Header.Set("X-CSRF-Token", session.CSRFToken)
		return r
	}

	me := request(http.MethodGet, "/api/v1/admin/session/me", nil)
	mr := httptest.NewRecorder()
	handler.ServeHTTP(mr, me)
	if mr.Code != http.StatusOK || json.Unmarshal(mr.Body.Bytes(), &session) != nil || session.CSRFToken == "" {
		t.Fatalf("me status=%d body=%s", mr.Code, mr.Body.String())
	}

	get := request(http.MethodGet, "/api/v1/admin/settings", nil)
	get.Header.Set("X-CSRF-Token", session.CSRFToken)
	for _, badCSRF := range []string{"malformed", cookies[0].Value} {
		bad := get.Clone(get.Context())
		bad.Header.Set("X-CSRF-Token", badCSRF)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, bad)
		if w.Code != http.StatusForbidden {
			t.Fatalf("bad CSRF returned %d, expected recovery-compatible 403", w.Code)
		}
	}
	gr := httptest.NewRecorder()
	handler.ServeHTTP(gr, get)
	var state settingsState
	if gr.Code != http.StatusOK || json.Unmarshal(gr.Body.Bytes(), &state) != nil || state.Desired.ResourceVersion != 1 {
		t.Fatalf("settings status=%d body=%s", gr.Code, gr.Body.String())
	}

	update := []byte(`{"apiVersion":"modelcairn.io/v1alpha1","kind":"AdminSettings","resourceVersion":1,"spec":{"idleSeconds":600}}`)
	planRequest := request(http.MethodPost, "/api/v1/admin/settings/plan", update)
	planRequest.Header.Set("Content-Type", "application/json")
	planRequest.Header.Set("X-CSRF-Token", session.CSRFToken)
	pr := httptest.NewRecorder()
	handler.ServeHTTP(pr, planRequest)
	var plan struct {
		PlanToken     string   `json:"planToken"`
		ChangedFields []string `json:"changedFields"`
	}
	if pr.Code != http.StatusOK || json.Unmarshal(pr.Body.Bytes(), &plan) != nil || plan.PlanToken == "" || len(plan.ChangedFields) != 1 {
		t.Fatalf("plan status=%d body=%s", pr.Code, pr.Body.String())
	}

	apply := request(http.MethodPost, "/api/v1/admin/settings/apply", update)
	apply.Header.Set("Content-Type", "application/json")
	apply.Header.Set("X-CSRF-Token", session.CSRFToken)
	apply.Header.Set("X-ModelCairn-Plan-Token", plan.PlanToken)
	ar := httptest.NewRecorder()
	handler.ServeHTTP(ar, apply)
	if ar.Code != http.StatusOK {
		t.Fatalf("apply status=%d body=%s", ar.Code, ar.Body.String())
	}
	if bytes.Contains(ar.Body.Bytes(), []byte(plan.PlanToken)) {
		t.Fatal("apply echoed plan token")
	}

	configuration := []byte(`{"apiVersion":"modelcairn.io/v1alpha1","kind":"Configuration","resources":[{"kind":"Provider","metadata":{"name":"provider"},"spec":{}}]}`)
	validate := request(http.MethodPost, "/api/v1/admin/config/validate", configuration)
	validate.Header.Set("Content-Type", "application/json")
	validate.Header.Set("X-CSRF-Token", session.CSRFToken)
	vr := httptest.NewRecorder()
	handler.ServeHTTP(vr, validate)
	if vr.Code != http.StatusOK {
		t.Fatalf("validate status=%d body=%s", vr.Code, vr.Body.String())
	}
	configPlan := request(http.MethodPost, "/api/v1/admin/config/plan", configuration)
	configPlan.Header.Set("Content-Type", "application/json")
	configPlan.Header.Set("X-CSRF-Token", session.CSRFToken)
	cpr := httptest.NewRecorder()
	handler.ServeHTTP(cpr, configPlan)
	var cp struct {
		PlanToken string    `json:"planToken"`
		ExpiresAt time.Time `json:"expiresAt"`
	}
	if cpr.Code != http.StatusOK || json.Unmarshal(cpr.Body.Bytes(), &cp) != nil || cp.PlanToken == "" || cp.ExpiresAt.IsZero() {
		t.Fatalf("config plan=%d body=%s", cpr.Code, cpr.Body.String())
	}
	configApply := request(http.MethodPost, "/api/v1/admin/config/apply", configuration)
	configApply.Header.Set("Content-Type", "application/json")
	configApply.Header.Set("X-CSRF-Token", session.CSRFToken)
	configApply.Header.Set("X-ModelCairn-Plan-Token", cp.PlanToken)
	car := httptest.NewRecorder()
	handler.ServeHTTP(car, configApply)
	if car.Code != http.StatusOK {
		t.Fatalf("config apply=%d body=%s", car.Code, car.Body.String())
	}
	export := request(http.MethodGet, "/api/v1/admin/config/export", nil)
	export.Header.Set("X-CSRF-Token", session.CSRFToken)
	er := httptest.NewRecorder()
	handler.ServeHTTP(er, export)
	if er.Code != http.StatusOK || !bytes.Contains(er.Body.Bytes(), []byte(`"name": "provider"`)) || bytes.Contains(er.Body.Bytes(), []byte(cp.PlanToken)) {
		t.Fatalf("export=%d body=%s", er.Code, er.Body.String())
	}
	resource := request(http.MethodGet, "/api/v1/admin/resources/providers/provider", nil)
	resource.Header.Set("X-CSRF-Token", session.CSRFToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, resource)
	if rr.Code != http.StatusOK || rr.Header().Get("ETag") != `"1"` {
		t.Fatalf("resource=%d etag=%q body=%s", rr.Code, rr.Header().Get("ETag"), rr.Body.String())
	}
	list := request(http.MethodGet, "/api/v1/admin/resources/providers?limit=1", nil)
	list.Header.Set("X-CSRF-Token", session.CSRFToken)
	listr := httptest.NewRecorder()
	handler.ServeHTTP(listr, list)
	if listr.Code != http.StatusOK || !bytes.Contains(listr.Body.Bytes(), []byte(`"provider"`)) {
		t.Fatalf("list=%d body=%s", listr.Code, listr.Body.String())
	}
	badCursor := request(http.MethodGet, "/api/v1/admin/resources/providers?cursor="+encodeCursor("secrets", "some-id"), nil)
	badCursor.Header.Set("X-CSRF-Token", session.CSRFToken)
	bcr := httptest.NewRecorder()
	handler.ServeHTTP(bcr, badCursor)
	if bcr.Code != http.StatusBadRequest {
		t.Fatalf("cross-scope cursor=%d", bcr.Code)
	}
	secrets := request(http.MethodGet, "/api/v1/admin/secrets", nil)
	secrets.Header.Set("X-CSRF-Token", session.CSRFToken)
	sr := httptest.NewRecorder()
	handler.ServeHTTP(sr, secrets)
	if sr.Code != http.StatusOK || !bytes.Contains(sr.Body.Bytes(), []byte(`"items":[]`)) {
		t.Fatalf("secrets=%d body=%s", sr.Code, sr.Body.String())
	}

	logout := request(http.MethodDelete, "/api/v1/admin/session", nil)
	logout.Header.Set("X-CSRF-Token", session.CSRFToken)
	or := httptest.NewRecorder()
	handler.ServeHTTP(or, logout)
	if or.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("logout lacks no-store")
	}
	if or.Code != http.StatusNoContent || len(or.Result().Cookies()) != 1 || or.Result().Cookies()[0].MaxAge >= 0 {
		t.Fatalf("logout status=%d cookies=%+v", or.Code, or.Result().Cookies())
	}
	gr = httptest.NewRecorder()
	handler.ServeHTTP(gr, get)
	if gr.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session status=%d", gr.Code)
	}
}

func TestAdminHTTPRejectsAmbiguousLoginJSON(t *testing.T) {
	handler := adminHandler(t)
	for _, body := range [][]byte{
		[]byte(`{"username":"owner","username":"other","password":"a secure password"}`),
		[]byte(`{"username":"owner","password":"a secure password","extra":true}`),
		append([]byte(`{"username":"owner","password":"`), append([]byte{0xff}, []byte(`"}`)...)...),
	} {
		r := adminRequest(http.MethodPost, "/api/v1/admin/session", body)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	}
}

func TestAdminHTTPRejectsBoundaryBeforeLoginWork(t *testing.T) {
	handler := adminHandler(t)
	r := adminRequest(http.MethodPost, "/api/v1/admin/session", []byte(`{"username":"owner","password":"a secure password"}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "http://evil.test")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}
