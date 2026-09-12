package router

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/sxamx/modelcairn/internal/chatcompletions"
)

type UpstreamResult struct {
	StatusCode        int
	ContentType       string
	RetryAfter        string
	ProviderRequestID string
	Body              []byte
}

type UpstreamExecutor interface {
	Execute(context.Context, Destination, chatcompletions.Request, string) (UpstreamResult, error)
}

type ClassifiedFailure interface {
	error
	FailureCode() string
	WasRequestWritten() bool
}

type Attempt struct {
	Sequence               int
	DestinationID          string
	StatusCode             int
	ErrorClass             string
	Retryable              bool
	FallbackReason         string
	Outcome                string
	ProviderRequestID      string
	RetryAfter             string
	StartedAt, CompletedAt time.Time
	Duration               time.Duration
}

type RunResult struct {
	Upstream UpstreamResult
	Attempts []Attempt
}

type RunError struct {
	Code     string
	Attempts []Attempt
	Cause    error
}

func (e *RunError) Error() string { return e.Code }
func (e *RunError) Unwrap() error { return e.Cause }

type Engine struct {
	executor UpstreamExecutor
	now      func() time.Time
}

func NewEngine(executor UpstreamExecutor) *Engine { return &Engine{executor: executor, now: time.Now} }

func (e *Engine) Run(ctx context.Context, snapshot Snapshot, request chatcompletions.Request) (RunResult, error) {
	if e == nil || e.executor == nil || snapshot.MaxAttempts < 1 || snapshot.AttemptTimeout <= 0 || snapshot.TotalTimeout <= 0 {
		return RunResult{}, &RunError{Code: "invalid_router_snapshot"}
	}
	if request.Stream {
		return RunResult{}, &RunError{Code: "streaming_not_supported"}
	}
	eligible := make([]Destination, 0, len(snapshot.Destinations))
	for _, destination := range snapshot.Destinations {
		if destination.Eligible() {
			eligible = append(eligible, destination)
		}
	}
	if len(eligible) == 0 {
		return RunResult{}, &RunError{Code: "no_eligible_destination"}
	}
	totalContext, cancelTotal := context.WithTimeout(ctx, snapshot.TotalTimeout)
	defer cancelTotal()
	result := RunResult{Attempts: make([]Attempt, 0, min(snapshot.MaxAttempts, len(eligible)))}
	limit := min(snapshot.MaxAttempts, len(eligible))
	for index := 0; index < limit; index++ {
		if err := totalContext.Err(); err != nil {
			return result, terminalContextError(ctx, err, result.Attempts)
		}
		destination := eligible[index]
		attemptContext, cancelAttempt := context.WithTimeout(totalContext, snapshot.AttemptTimeout)
		started := e.now()
		upstream, executeErr := e.executor.Execute(attemptContext, destination, request, snapshot.Alias)
		cancelAttempt()
		result.Upstream = upstream
		completed := e.now()
		attempt := Attempt{Sequence: index + 1, DestinationID: destination.ID, StatusCode: upstream.StatusCode, ProviderRequestID: upstream.ProviderRequestID, RetryAfter: upstream.RetryAfter, StartedAt: started, CompletedAt: completed, Duration: completed.Sub(started), Outcome: "error"}
		retry, terminalCode := classifyAttempt(ctx, totalContext, executeErr, upstream.StatusCode, &attempt)
		result.Attempts = append(result.Attempts, attempt)
		if executeErr == nil && upstream.StatusCode >= 200 && upstream.StatusCode < 300 {
			result.Attempts[len(result.Attempts)-1].Outcome = "success"
			return result, nil
		}
		if !retry {
			return result, &RunError{Code: terminalCode, Attempts: append([]Attempt(nil), result.Attempts...), Cause: executeErr}
		}
		if index+1 == limit {
			return result, &RunError{Code: "fallback_exhausted", Attempts: append([]Attempt(nil), result.Attempts...), Cause: executeErr}
		}
	}
	return result, &RunError{Code: "fallback_exhausted", Attempts: append([]Attempt(nil), result.Attempts...)}
}

func classifyAttempt(parent, total context.Context, executeErr error, status int, attempt *Attempt) (bool, string) {
	if parent.Err() != nil {
		attempt.ErrorClass = "client_cancelled"
		attempt.Outcome = "cancelled"
		return false, "request_cancelled"
	}
	if total.Err() != nil {
		attempt.ErrorClass = "total_timeout"
		attempt.Outcome = "indeterminate"
		return false, "total_timeout"
	}
	if executeErr != nil {
		var failure ClassifiedFailure
		if errors.As(executeErr, &failure) {
			attempt.ErrorClass = failure.FailureCode()
			switch failure.FailureCode() {
			case "transport_error":
				if !failure.WasRequestWritten() {
					attempt.Retryable = true
					attempt.FallbackReason = "transport_pre_send"
					return true, ""
				}
				attempt.Outcome = "indeterminate"
				return false, "indeterminate_upstream"
			case "invalid_response", "response_too_large":
				attempt.Retryable = true
				attempt.FallbackReason = failure.FailureCode()
				return true, ""
			default:
				return false, failure.FailureCode()
			}
		}
		if errors.Is(executeErr, context.DeadlineExceeded) {
			attempt.ErrorClass = "attempt_timeout"
			attempt.Outcome = "indeterminate"
			return false, "indeterminate_upstream"
		}
		attempt.ErrorClass = "unclassified_error"
		return false, "upstream_error"
	}
	attempt.ErrorClass = "provider_status"
	switch status {
	case http.StatusTooManyRequests:
		attempt.Retryable = true
		attempt.FallbackReason = "rate_limited"
		return true, ""
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		attempt.Retryable = true
		attempt.FallbackReason = "provider_transient"
		return true, ""
	default:
		return false, "provider_error"
	}
}

func terminalContextError(parent context.Context, err error, attempts []Attempt) error {
	code := "total_timeout"
	if parent.Err() != nil {
		code = "request_cancelled"
	}
	return &RunError{Code: code, Attempts: append([]Attempt(nil), attempts...), Cause: err}
}
