package router

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/sxamx/modelcairn/internal/chatcompletions"
	"github.com/sxamx/modelcairn/internal/streaming"
)

type StreamUpstream struct {
	StatusCode        int
	ContentType       string
	RetryAfter        string
	ProviderRequestID string
	Body              io.ReadCloser
}

type StreamExecutor interface {
	OpenStream(context.Context, Destination, chatcompletions.Request) (StreamUpstream, error)
}

type StreamRunResult struct {
	Attempts     []Attempt
	Committed    bool
	Completed    bool
	Bytes        int64
	Events       int
	InputTokens  *int
	OutputTokens *int
	FirstEventAt time.Time
}

func (e *Engine) RunStream(ctx context.Context, destination http.ResponseWriter, snapshot Snapshot, request chatcompletions.Request) (StreamRunResult, error) {
	if e == nil {
		return StreamRunResult{}, &RunError{Code: "streaming_not_supported"}
	}
	executor, ok := e.executor.(StreamExecutor)
	if !ok || snapshot.MaxAttempts < 1 || snapshot.AttemptTimeout <= 0 || snapshot.TotalTimeout <= 0 {
		return StreamRunResult{}, &RunError{Code: "streaming_not_supported"}
	}
	if !request.Stream {
		return StreamRunResult{}, &RunError{Code: "streaming_required"}
	}
	eligible := make([]Destination, 0, len(snapshot.Destinations))
	for _, candidate := range snapshot.Destinations {
		if candidate.Eligible() {
			eligible = append(eligible, candidate)
		}
	}
	if len(eligible) == 0 {
		return StreamRunResult{}, &RunError{Code: "no_eligible_destination"}
	}
	totalContext, cancelTotal := context.WithTimeout(ctx, snapshot.TotalTimeout)
	defer cancelTotal()
	result := StreamRunResult{Attempts: make([]Attempt, 0, min(snapshot.MaxAttempts, len(eligible)))}
	limit := min(snapshot.MaxAttempts, len(eligible))
	for index := 0; index < limit; index++ {
		if err := totalContext.Err(); err != nil {
			return result, terminalContextError(ctx, err, result.Attempts)
		}
		candidate := eligible[index]
		attemptContext, cancelAttempt := context.WithTimeout(totalContext, snapshot.AttemptTimeout)
		started := e.now()
		upstream, openErr := executor.OpenStream(attemptContext, candidate, request)
		if openErr != nil || upstream.StatusCode < 200 || upstream.StatusCode >= 300 {
			cancelAttempt()
			completed := e.now()
			attempt := Attempt{Sequence: index + 1, DestinationID: candidate.ID, StatusCode: upstream.StatusCode,
				ProviderRequestID: upstream.ProviderRequestID, RetryAfter: upstream.RetryAfter,
				StartedAt: started, CompletedAt: completed, Duration: completed.Sub(started), Outcome: "error"}
			retry, terminalCode := classifyAttempt(ctx, totalContext, openErr, upstream.StatusCode, &attempt)
			result.Attempts = append(result.Attempts, attempt)
			if !retry {
				return result, &RunError{Code: terminalCode, Attempts: append([]Attempt(nil), result.Attempts...), Cause: openErr}
			}
			if index+1 == limit {
				return result, &RunError{Code: "fallback_exhausted", Attempts: append([]Attempt(nil), result.Attempts...), Cause: openErr}
			}
			continue
		}
		relay, relayErr := streaming.RelayChatCompletions(attemptContext, destination, upstream.Body, snapshot.Alias)
		cancelAttempt()
		completed := e.now()
		attempt := Attempt{Sequence: index + 1, DestinationID: candidate.ID, StatusCode: upstream.StatusCode,
			ProviderRequestID: upstream.ProviderRequestID, StartedAt: started, CompletedAt: completed,
			Duration: completed.Sub(started), Outcome: "error"}
		result.Committed, result.Completed, result.Bytes, result.Events = relay.Committed, relay.Completed, relay.Bytes, relay.Events
		result.InputTokens, result.OutputTokens, result.FirstEventAt = relay.InputTokens, relay.OutputTokens, relay.FirstEventAt
		if relayErr == nil {
			attempt.Outcome = "success"
			result.Attempts = append(result.Attempts, attempt)
			return result, nil
		}
		if relay.Committed {
			code := "stream_interrupted"
			attempt.ErrorClass, attempt.Outcome = code, "partial"
			if ctx.Err() != nil {
				code, attempt.ErrorClass, attempt.Outcome = "request_cancelled", "client_cancelled", "cancelled"
			}
			result.Attempts = append(result.Attempts, attempt)
			return result, &RunError{Code: code, Attempts: append([]Attempt(nil), result.Attempts...), Cause: relayErr}
		}
		if errors.Is(relayErr, context.Canceled) || errors.Is(relayErr, context.DeadlineExceeded) {
			attempt.ErrorClass, attempt.Outcome = "indeterminate_stream", "indeterminate"
			code := "indeterminate_upstream"
			if ctx.Err() != nil {
				code, attempt.ErrorClass, attempt.Outcome = "request_cancelled", "client_cancelled", "cancelled"
			}
			result.Attempts = append(result.Attempts, attempt)
			return result, &RunError{Code: code, Attempts: append([]Attempt(nil), result.Attempts...), Cause: relayErr}
		}
		failure := streamValidationFailure{err: relayErr}
		retry, terminalCode := classifyAttempt(ctx, totalContext, failure, upstream.StatusCode, &attempt)
		result.Attempts = append(result.Attempts, attempt)
		if !retry {
			return result, &RunError{Code: terminalCode, Attempts: append([]Attempt(nil), result.Attempts...), Cause: relayErr}
		}
		if index+1 == limit {
			return result, &RunError{Code: "fallback_exhausted", Attempts: append([]Attempt(nil), result.Attempts...), Cause: relayErr}
		}
	}
	return result, &RunError{Code: "fallback_exhausted", Attempts: append([]Attempt(nil), result.Attempts...)}
}

type streamValidationFailure struct{ err error }

func (e streamValidationFailure) Error() string         { return e.err.Error() }
func (e streamValidationFailure) Unwrap() error         { return e.err }
func (streamValidationFailure) FailureCode() string     { return "invalid_response" }
func (streamValidationFailure) WasRequestWritten() bool { return true }
