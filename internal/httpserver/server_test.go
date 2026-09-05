package httpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testHandler(probe *ReadinessProbe) http.Handler {
	return New("127.0.0.1:0", probe, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler
}

func TestHealthIsIndependentFromReadiness(t *testing.T) {
	recorder := httptest.NewRecorder()
	testHandler(NewReadinessProbe()).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("health status = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestReadinessReportsDependencies(t *testing.T) {
	probe := NewReadinessProbe()
	handler := testHandler(probe)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("initial readiness status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}

	for _, name := range []Component{ComponentConfiguration, ComponentPersistence, ComponentSecretStore} {
		if !probe.Set(name, true, "") {
			t.Fatalf("known component %q rejected", name)
		}
	}
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("ready status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode readiness response: %v", err)
	}
	if response.Status != "ready" {
		t.Fatalf("readiness response status = %q, want ready", response.Status)
	}
}

func TestReadinessRejectsUnknownComponent(t *testing.T) {
	probe := NewReadinessProbe()
	if probe.Set(Component("persistnce"), true, "") {
		t.Fatal("unknown component was accepted")
	}
}

func TestRequestLogDoesNotRecordRawPath(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	handler := New("127.0.0.1:0", NewReadinessProbe(), logger).Handler
	secret := "sk-should-never-be-logged"
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/missing/"+secret, nil))
	if strings.Contains(output.String(), secret) {
		t.Fatalf("request log leaked raw path: %s", output.String())
	}
	if !strings.Contains(output.String(), `"route":"unmatched"`) {
		t.Fatalf("request log did not use safe route label: %s", output.String())
	}
}

func BenchmarkHealthz(b *testing.B) {
	handler := testHandler(NewReadinessProbe())
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	b.ReportAllocs()
	for b.Loop() {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
	}
}
