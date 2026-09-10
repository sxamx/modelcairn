package httpserver

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sxamx/modelcairn/internal/adminsettings"
)

func boundarySettings() adminsettings.Resolved {
	s := adminsettings.Defaults()
	s.PublicOrigin = "http://127.0.0.1:8080"
	return s
}

func TestLoopbackAdminBoundary(t *testing.T) {
	b, err := newAdminBoundary(boundarySettings())
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "http://127.0.0.1:8080/api/v1/admin/settings/plan", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("Origin", "http://127.0.0.1:8080")
	if client, err := b.client(r); err != nil || !client.IsLoopback() {
		t.Fatalf("client=%v err=%v", client, err)
	}
	if err := b.requireOrigin(r); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*http.Request){
		"remote":         func(r *http.Request) { r.RemoteAddr = "192.0.2.1:1234" },
		"host":           func(r *http.Request) { r.Host = "example.test" },
		"forwarded":      func(r *http.Request) { r.Header.Set("X-Forwarded-For", "127.0.0.1") },
		"forwarded list": func(r *http.Request) { r.Header.Set("X-Forwarded-For", "127.0.0.1, 192.0.2.1") },
	} {
		t.Run(name, func(t *testing.T) {
			copy := r.Clone(r.Context())
			copy.Header = r.Header.Clone()
			mutate(copy)
			if _, err := b.client(copy); err == nil {
				t.Fatal("boundary accepted")
			}
		})
	}
}

func TestProxyTLSUsesOnlyTrustedSingleHop(t *testing.T) {
	s := adminsettings.Defaults()
	s.PublicOrigin = "https://admin.example.test"
	s.Listen = "100.64.0.1:8080"
	s.Transport = adminsettings.ProxyTLS
	s.TrustedProxyCIDRs = []string{"100.64.0.0/10"}
	b, err := newAdminBoundary(s)
	if err != nil {
		t.Fatal(err)
	}
	valid := httptest.NewRequest("GET", "http://admin.example.test/api/v1/admin/session/me", nil)
	valid.RemoteAddr = "100.96.1.2:5000"
	valid.Header.Set("X-Forwarded-Proto", "https")
	valid.Header.Set("X-Forwarded-For", "203.0.113.8")
	valid.Header.Set("Sec-Fetch-Site", "same-origin")
	client, err := b.client(valid)
	if err != nil || client.String() != "203.0.113.8" {
		t.Fatalf("client=%v err=%v", client, err)
	}
	if err := b.requireRecoveryOrigin(valid); err != nil {
		t.Fatal(err)
	}
	cases := []func(*http.Request){
		func(r *http.Request) { r.RemoteAddr = "192.0.2.2:5000" },
		func(r *http.Request) { r.Header.Set("X-Forwarded-For", "203.0.113.8, 203.0.113.9") },
		func(r *http.Request) { r.Header["X-Forwarded-Proto"] = []string{"https", "https"} },
		func(r *http.Request) { r.TLS = &tls.ConnectionState{} },
	}
	for index, mutate := range cases {
		copy := valid.Clone(valid.Context())
		copy.Header = valid.Header.Clone()
		mutate(copy)
		if _, err := b.client(copy); err == nil {
			t.Fatalf("case %d accepted", index)
		}
	}
}

func TestOriginRequiresExactlyOneCanonicalValue(t *testing.T) {
	b, _ := newAdminBoundary(boundarySettings())
	for _, values := range [][]string{nil, {"null"}, {"http://127.0.0.1:8080/"}, {"http://127.0.0.1:8080", "http://127.0.0.1:8080"}} {
		r := httptest.NewRequest("POST", "http://127.0.0.1:8080/", nil)
		r.Header["Origin"] = values
		if err := b.requireOrigin(r); err == nil {
			t.Fatalf("accepted %v", values)
		}
	}
	r := httptest.NewRequest("GET", "http://127.0.0.1:8080/", nil)
	r.Header.Set("Origin", "http://evil.test")
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	if err := b.requireRecoveryOrigin(r); err == nil {
		t.Fatal("foreign Origin accepted")
	}
	r.Header.Set("Origin", "http://evil.test, http://127.0.0.1:8080")
	if err := b.requireRecoveryOrigin(r); err == nil {
		t.Fatal("malformed Origin fell back to fetch metadata")
	}
}

func TestSessionCookieRejectsDuplicates(t *testing.T) {
	r := httptest.NewRequest("GET", "http://example.test/", nil)
	r.Header.Add("Cookie", "mc_session=one; mc_session=two")
	if _, err := sessionCookie(r); err == nil {
		t.Fatal("duplicate cookie accepted")
	}
}
