package consoleui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRootServesProtectedConsoleShell(t *testing.T) {
	recorder := httptest.NewRecorder()
	Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "ModelCairn") {
		t.Fatalf("root response = %d %q", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Content-Security-Policy") == "" || recorder.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("missing console protections: %v", recorder.Header())
	}
}

func TestNavigationFallsBackButReservedAndMissingAssetsDoNot(t *testing.T) {
	for _, test := range []struct {
		path string
		want int
		html bool
	}{
		{"/settings", http.StatusOK, true},
		{"/api/v1/admin/unknown", http.StatusNotFound, false},
		{"/v1/unknown", http.StatusNotFound, false},
		{"/missing.js", http.StatusNotFound, false},
	} {
		recorder := httptest.NewRecorder()
		Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
		if recorder.Code != test.want || strings.Contains(recorder.Body.String(), "<html") != test.html {
			t.Errorf("GET %s = %d html=%v", test.path, recorder.Code, strings.Contains(recorder.Body.String(), "<html"))
		}
	}
}

func TestManifestIsAvailableWithoutDurableCaching(t *testing.T) {
	recorder := httptest.NewRecorder()
	Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/manifest.webmanifest", nil))
	if recorder.Code != http.StatusOK || recorder.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("manifest response = %d, cache=%q", recorder.Code, recorder.Header().Get("Cache-Control"))
	}
}
