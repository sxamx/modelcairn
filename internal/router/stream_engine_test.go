package router

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sxamx/modelcairn/internal/chatcompletions"
)

type fixtureStreamOutcome struct {
	result StreamUpstream
	err    error
}
type fixtureStreamExecutor struct {
	outcomes []fixtureStreamOutcome
	calls    []string
}

func (e *fixtureStreamExecutor) Execute(context.Context, Destination, chatcompletions.Request, string) (UpstreamResult, error) {
	return UpstreamResult{}, errors.New("non-streaming not expected")
}
func (e *fixtureStreamExecutor) OpenStream(_ context.Context, destination Destination, _ chatcompletions.Request) (StreamUpstream, error) {
	e.calls = append(e.calls, destination.ID)
	if len(e.outcomes) == 0 {
		return StreamUpstream{}, errors.New("fixture exhausted")
	}
	outcome := e.outcomes[0]
	e.outcomes = e.outcomes[1:]
	return outcome.result, outcome.err
}

func streamBody(value string) io.ReadCloser { return io.NopCloser(strings.NewReader(value)) }
func streamSnapshot() Snapshot {
	return Snapshot{Alias: "assistant", MaxAttempts: 2, AttemptTimeout: time.Second, TotalTimeout: 3 * time.Second, Destinations: []Destination{{ID: "one"}, {ID: "two"}}}
}

func TestStreamEngineFallsBackBeforeCommitment(t *testing.T) {
	valid := `data: {"id":"c","object":"chat.completion.chunk","model":"physical","choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":null}]}` + "\n\ndata: [DONE]\n\n"
	executor := &fixtureStreamExecutor{outcomes: []fixtureStreamOutcome{
		{result: StreamUpstream{StatusCode: http.StatusOK, Body: streamBody("data: invalid\n\n")}},
		{result: StreamUpstream{StatusCode: http.StatusOK, Body: streamBody(valid)}},
	}}
	w := httptest.NewRecorder()
	result, err := NewEngine(executor).RunStream(context.Background(), w, streamSnapshot(), chatcompletions.Request{Stream: true})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Committed || !result.Completed || len(result.Attempts) != 2 || !result.Attempts[0].Retryable || result.Attempts[0].FallbackReason != "invalid_response" {
		t.Fatalf("result=%+v", result)
	}
	if strings.Contains(w.Body.String(), "invalid") || strings.Contains(w.Body.String(), "physical") || !strings.Contains(w.Body.String(), "assistant") {
		t.Fatalf("body=%q", w.Body.String())
	}
}

func TestStreamEngineFallsBackFrom429BeforeOpeningBody(t *testing.T) {
	valid := `data: {"id":"c","object":"chat.completion.chunk","model":"physical","choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":null}]}` + "\n\ndata: [DONE]\n\n"
	executor := &fixtureStreamExecutor{outcomes: []fixtureStreamOutcome{
		{result: StreamUpstream{StatusCode: http.StatusTooManyRequests, RetryAfter: "4"}},
		{result: StreamUpstream{StatusCode: http.StatusOK, Body: streamBody(valid)}},
	}}
	result, err := NewEngine(executor).RunStream(context.Background(), httptest.NewRecorder(), streamSnapshot(), chatcompletions.Request{Stream: true})
	if err != nil || len(result.Attempts) != 2 || result.Attempts[0].RetryAfter != "4" || result.Attempts[0].FallbackReason != "rate_limited" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestStreamEngineNeverFallsBackAfterCommitment(t *testing.T) {
	partial := `data: {"id":"c","object":"chat.completion.chunk","model":"physical","choices":[{"index":0,"delta":{"content":"first"},"finish_reason":null}]}` + "\n\ndata: invalid\n\n"
	executor := &fixtureStreamExecutor{outcomes: []fixtureStreamOutcome{
		{result: StreamUpstream{StatusCode: http.StatusOK, Body: streamBody(partial)}},
		{result: StreamUpstream{StatusCode: http.StatusOK, Body: streamBody("data: [DONE]\n\n")}},
	}}
	result, err := NewEngine(executor).RunStream(context.Background(), httptest.NewRecorder(), streamSnapshot(), chatcompletions.Request{Stream: true})
	var runErr *RunError
	if !errors.As(err, &runErr) || runErr.Code != "stream_interrupted" || !result.Committed || result.Completed || len(result.Attempts) != 1 || result.Attempts[0].Outcome != "partial" || len(executor.calls) != 1 {
		t.Fatalf("result=%+v err=%v calls=%v", result, err, executor.calls)
	}
}

func TestStreamEngineRejectsNonStreamingRequestAndUnsupportedExecutor(t *testing.T) {
	_, err := NewEngine(&fixtureStreamExecutor{}).RunStream(context.Background(), httptest.NewRecorder(), streamSnapshot(), chatcompletions.Request{})
	var runErr *RunError
	if !errors.As(err, &runErr) || runErr.Code != "streaming_required" {
		t.Fatalf("err=%v", err)
	}
	_, err = NewEngine(&fixtureExecutor{}).RunStream(context.Background(), httptest.NewRecorder(), streamSnapshot(), chatcompletions.Request{Stream: true})
	if !errors.As(err, &runErr) || runErr.Code != "streaming_not_supported" {
		t.Fatalf("err=%v", err)
	}
}
