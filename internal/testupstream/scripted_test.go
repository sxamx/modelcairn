package testupstream

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestScriptedServesStepsAndKeepsOnlySafeMetadata(t *testing.T) {
	handler := New(
		Step{Status: http.StatusTooManyRequests, Header: http.Header{"Retry-After": {"7"}}, Body: `{"error":"limited"}`},
		Step{Header: http.Header{"Content-Type": {"application/json"}}, Body: `{"id":"ok"}`},
	)
	server := httptest.NewServer(handler)
	defer server.Close()
	for index, wantStatus := range []int{http.StatusTooManyRequests, http.StatusOK} {
		request, err := http.NewRequest(http.MethodPost, server.URL+"/chat/completions", strings.NewReader(`{"secret prompt":"do not retain"}`))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer x")
		request.Header.Set("Content-Type", "application/json")
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
		if response.StatusCode != wantStatus {
			t.Fatalf("response %d status=%d", index, response.StatusCode)
		}
	}
	observations := handler.Observations()
	if len(observations) != 2 || observations[0].Sequence != 1 || !observations[0].AuthorizationPresent {
		t.Fatalf("observations=%+v", observations)
	}
	if observations[0].Method != http.MethodPost || observations[0].Path != "/chat/completions" || observations[0].BodyBytes == 0 {
		t.Fatalf("observation=%+v", observations[0])
	}
	if handler.Remaining() != 0 {
		t.Fatalf("remaining=%d", handler.Remaining())
	}
}

func TestScriptedDelayHonorsCancellation(t *testing.T) {
	startedHandler := make(chan struct{}, 1)
	handler := New(Step{Delay: time.Minute, Body: "too late", Started: startedHandler})
	server := httptest.NewServer(handler)
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		_, requestErr := http.DefaultClient.Do(request)
		result <- requestErr
	}()
	select {
	case <-startedHandler:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}
	started := time.Now()
	cancel()
	select {
	case err = <-result:
		if err == nil || time.Since(started) > time.Second {
			t.Fatalf("err=%v duration=%s", err, time.Since(started))
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation did not stop the request")
	}
}

func TestScriptedDisconnectsAndExhaustionIsExplicit(t *testing.T) {
	handler := New(Step{Disconnect: true})
	server := httptest.NewServer(handler)
	defer server.Close()
	if _, err := http.Post(server.URL, "application/json", nil); err == nil {
		t.Fatal("expected disconnect error")
	}
	response, err := http.Post(server.URL, "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status=%d", response.StatusCode)
	}
}
