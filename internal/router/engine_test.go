package router

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sxamx/modelcairn/internal/chatcompletions"
)

type fixtureFailure struct {
	code    string
	written bool
}

func (e fixtureFailure) Error() string           { return e.code }
func (e fixtureFailure) FailureCode() string     { return e.code }
func (e fixtureFailure) WasRequestWritten() bool { return e.written }

type fixtureOutcome struct {
	result UpstreamResult
	err    error
}
type fixtureExecutor struct {
	outcomes     []fixtureOutcome
	destinations []string
}

func (e *fixtureExecutor) Execute(_ context.Context, destination Destination, _ chatcompletions.Request, _ string) (UpstreamResult, error) {
	e.destinations = append(e.destinations, destination.ID)
	if len(e.outcomes) == 0 {
		return UpstreamResult{}, errors.New("fixture exhausted")
	}
	outcome := e.outcomes[0]
	e.outcomes = e.outcomes[1:]
	return outcome.result, outcome.err
}

func engineSnapshot() Snapshot {
	return Snapshot{Alias: "assistant", MaxAttempts: 3, AttemptTimeout: time.Second, TotalTimeout: 3 * time.Second,
		Destinations: []Destination{{ID: "excluded", ExclusionReasons: []string{"cooldown_active"}}, {ID: "one"}, {ID: "two"}, {ID: "three"}}}
}

func TestEngineFallsBackSequentiallyAcrossEligibleDestinations(t *testing.T) {
	executor := &fixtureExecutor{outcomes: []fixtureOutcome{
		{result: UpstreamResult{StatusCode: 429, RetryAfter: "5"}},
		{result: UpstreamResult{StatusCode: 503}},
		{result: UpstreamResult{StatusCode: 200, Body: []byte(`{"id":"ok"}`)}},
	}}
	result, err := NewEngine(executor).Run(context.Background(), engineSnapshot(), chatcompletions.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if string(result.Upstream.Body) != `{"id":"ok"}` || len(result.Attempts) != 3 {
		t.Fatalf("result=%+v", result)
	}
	if got := executor.destinations; len(got) != 3 || got[0] != "one" || got[1] != "two" || got[2] != "three" {
		t.Fatalf("destinations=%v", got)
	}
	if !result.Attempts[0].Retryable || result.Attempts[0].FallbackReason != "rate_limited" || result.Attempts[1].FallbackReason != "provider_transient" {
		t.Fatalf("attempts=%+v", result.Attempts)
	}
}

func TestEngineDoesNotRetryTerminalOrIndeterminateFailure(t *testing.T) {
	tests := []struct {
		name    string
		outcome fixtureOutcome
		code    string
	}{
		{"client-error", fixtureOutcome{result: UpstreamResult{StatusCode: 400}}, "provider_error"},
		{"written-transport", fixtureOutcome{err: fixtureFailure{code: "transport_error", written: true}}, "indeterminate_upstream"},
		{"credential", fixtureOutcome{err: fixtureFailure{code: "credential_unavailable"}}, "credential_unavailable"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			executor := &fixtureExecutor{outcomes: []fixtureOutcome{test.outcome, {result: UpstreamResult{StatusCode: 200}}}}
			result, err := NewEngine(executor).Run(context.Background(), engineSnapshot(), chatcompletions.Request{})
			var runErr *RunError
			if !errors.As(err, &runErr) || runErr.Code != test.code || len(result.Attempts) != 1 || len(executor.destinations) != 1 {
				t.Fatalf("result=%+v err=%v calls=%v", result, err, executor.destinations)
			}
		})
	}
}

func TestEngineRetriesOnlySafeExecutionFailures(t *testing.T) {
	for _, failure := range []fixtureFailure{{code: "transport_error", written: false}, {code: "invalid_response", written: true}, {code: "response_too_large", written: true}} {
		executor := &fixtureExecutor{outcomes: []fixtureOutcome{{err: failure}, {result: UpstreamResult{StatusCode: 200}}}}
		result, err := NewEngine(executor).Run(context.Background(), engineSnapshot(), chatcompletions.Request{})
		if err != nil || len(result.Attempts) != 2 || !result.Attempts[0].Retryable {
			t.Fatalf("failure=%+v result=%+v err=%v", failure, result, err)
		}
	}
}

func TestEngineHonorsAttemptCapAndRejectsStreaming(t *testing.T) {
	snapshot := engineSnapshot()
	snapshot.MaxAttempts = 1
	executor := &fixtureExecutor{outcomes: []fixtureOutcome{{result: UpstreamResult{StatusCode: 429}}, {result: UpstreamResult{StatusCode: 200}}}}
	result, err := NewEngine(executor).Run(context.Background(), snapshot, chatcompletions.Request{})
	var runErr *RunError
	if !errors.As(err, &runErr) || runErr.Code != "fallback_exhausted" || len(result.Attempts) != 1 || len(executor.destinations) != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	_, err = NewEngine(executor).Run(context.Background(), snapshot, chatcompletions.Request{Stream: true})
	if !errors.As(err, &runErr) || runErr.Code != "streaming_not_supported" {
		t.Fatalf("stream err=%v", err)
	}
}

func TestEngineStopsImmediatelyWhenCallerIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	executor := &fixtureExecutor{outcomes: []fixtureOutcome{{result: UpstreamResult{StatusCode: 200}}}}
	_, err := NewEngine(executor).Run(ctx, engineSnapshot(), chatcompletions.Request{})
	var runErr *RunError
	if !errors.As(err, &runErr) || runErr.Code != "request_cancelled" || len(executor.destinations) != 0 {
		t.Fatalf("err=%v calls=%v", err, executor.destinations)
	}
}
