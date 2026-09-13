package storage

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type RequestStart struct {
	ID, AgentTokenID, RouteID, RequestedAlias string
	StartedAt                                 time.Time
}

type AttemptCompletion struct {
	Sequence                           int
	DestinationID, Outcome, ErrorClass string
	ProviderRequestID, FallbackReason  string
	RetryAfter                         string
	ProviderStatus                     int
	Retryable                          bool
	StartedAt, CompletedAt             time.Time
}

type RequestCompletion struct {
	Outcome                   string
	HTTPStatus                int
	InputTokens, OutputTokens *int
	TTFT                      *time.Duration
	CompletedAt               time.Time
	Attempts                  []AttemptCompletion
}

type OperationalRecorder struct{ db *sql.DB }

func NewOperationalRecorder(db *sql.DB) *OperationalRecorder { return &OperationalRecorder{db: db} }

func (r *OperationalRecorder) Begin(ctx context.Context, input RequestStart) error {
	if r == nil || r.db == nil || input.ID == "" || input.AgentTokenID == "" || input.RouteID == "" || input.RequestedAlias == "" || input.StartedAt.IsZero() {
		return &RepositoryError{Code: CodeInvalidResource}
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO requests(id,agent_token_id,route_id,requested_alias,started_at,content_stored)
		VALUES(?,?,?,?,?,0)`, input.ID, input.AgentTokenID, input.RouteID, input.RequestedAlias, input.StartedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("begin operational request: %w", err)
	}
	return nil
}

func (r *OperationalRecorder) Complete(ctx context.Context, requestID string, completion RequestCompletion) error {
	if r == nil || r.db == nil || requestID == "" || completion.CompletedAt.IsZero() || !validRequestOutcome(completion.Outcome) || completion.HTTPStatus < 100 || completion.HTTPStatus > 599 {
		return &RepositoryError{Code: CodeInvalidResource}
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin operational completion: %w", err)
	}
	defer tx.Rollback()
	for index, attempt := range completion.Attempts {
		if attempt.Sequence != index+1 || attempt.DestinationID == "" || !validRequestOutcome(attempt.Outcome) || attempt.StartedAt.IsZero() || attempt.CompletedAt.Before(attempt.StartedAt) {
			return &RepositoryError{Code: CodeInvalidResource}
		}
		attemptID, idErr := newUUID()
		if idErr != nil {
			return idErr
		}
		var providerStatus any
		if attempt.ProviderStatus != 0 {
			providerStatus = attempt.ProviderStatus
		}
		var errorClass, providerRequestID, fallbackReason any
		if attempt.ErrorClass != "" {
			errorClass = attempt.ErrorClass
		}
		if attempt.ProviderRequestID != "" {
			providerRequestID = attempt.ProviderRequestID
		}
		if attempt.FallbackReason != "" {
			fallbackReason = attempt.FallbackReason
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO attempts(id,request_id,sequence,destination_id,started_at,completed_at,outcome,error_class,provider_status,provider_request_id,retryable,fallback_reason)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, attemptID, requestID, attempt.Sequence, attempt.DestinationID,
			attempt.StartedAt.UTC().Format(time.RFC3339Nano), attempt.CompletedAt.UTC().Format(time.RFC3339Nano), attempt.Outcome,
			errorClass, providerStatus, providerRequestID, attempt.Retryable, fallbackReason)
		if err != nil {
			return fmt.Errorf("record operational attempt: %w", err)
		}
		if attempt.ProviderStatus == http.StatusTooManyRequests {
			if err := recordRateLimitTx(ctx, tx, attemptID, attempt); err != nil {
				return err
			}
		}
	}
	var inputTokens, outputTokens, ttft any
	if completion.InputTokens != nil {
		inputTokens = *completion.InputTokens
	}
	if completion.OutputTokens != nil {
		outputTokens = *completion.OutputTokens
	}
	if completion.TTFT != nil {
		ttft = completion.TTFT.Milliseconds()
	}
	var started string
	if err := tx.QueryRowContext(ctx, "SELECT started_at FROM requests WHERE id=? AND completed_at IS NULL", requestID).Scan(&started); err != nil {
		if err == sql.ErrNoRows {
			return &RepositoryError{Code: CodeNotFound}
		}
		return err
	}
	startedAt, err := time.Parse(time.RFC3339Nano, started)
	if err != nil || completion.CompletedAt.Before(startedAt) {
		return &RepositoryError{Code: CodeInvalidResource}
	}
	duration := completion.CompletedAt.Sub(startedAt).Milliseconds()
	result, err := tx.ExecContext(ctx, `UPDATE requests SET completed_at=?,outcome=?,http_status=?,input_tokens=?,output_tokens=?,ttft_ms=?,duration_ms=? WHERE id=? AND completed_at IS NULL`,
		completion.CompletedAt.UTC().Format(time.RFC3339Nano), completion.Outcome, completion.HTTPStatus, inputTokens, outputTokens, ttft, duration, requestID)
	if err != nil {
		return fmt.Errorf("complete operational request: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return &RepositoryError{Code: CodeNotFound}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit operational completion: %w", err)
	}
	return nil
}

func recordRateLimitTx(ctx context.Context, tx *sql.Tx, attemptID string, attempt AttemptCompletion) error {
	resetAt, source := retryReset(attempt.RetryAfter, attempt.CompletedAt)
	observationID, err := newUUID()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO rate_limit_observations(id,attempt_id,scope_kind,scope_resource_id,source,resets_at,observed_at,raw_content_stored)
		VALUES(?,?,'destination',?,?,?, ?,0)`, observationID, attemptID, attempt.DestinationID, source,
		resetAt.UTC().Format(time.RFC3339Nano), attempt.CompletedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("record rate limit observation: %w", err)
	}
	cooldownID, err := newUUID()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO cooldowns(id,scope_kind,scope_resource_id,reason,starts_at,ends_at,source_observation_id)
		VALUES(?,'destination',?,'rate_limited',?,?,?)
		ON CONFLICT(scope_kind,scope_resource_id) DO UPDATE SET reason=excluded.reason,starts_at=excluded.starts_at,ends_at=excluded.ends_at,source_observation_id=excluded.source_observation_id
		WHERE excluded.ends_at>cooldowns.ends_at`, cooldownID, attempt.DestinationID,
		attempt.CompletedAt.UTC().Format(time.RFC3339Nano), resetAt.UTC().Format(time.RFC3339Nano), observationID)
	if err != nil {
		return fmt.Errorf("record destination cooldown: %w", err)
	}
	return nil
}

func retryReset(value string, observed time.Time) (time.Time, string) {
	const maximum = 24 * time.Hour
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 {
		delay := maximum
		if seconds <= int64(maximum/time.Second) {
			delay = time.Duration(seconds) * time.Second
		}
		if delay < time.Second {
			delay = time.Second
		}
		if delay > maximum {
			delay = maximum
		}
		return observed.Add(delay), "header"
	}
	if parsed, err := http.ParseTime(value); err == nil && parsed.After(observed) {
		if parsed.After(observed.Add(maximum)) {
			parsed = observed.Add(maximum)
		}
		return parsed, "header"
	}
	return observed.Add(30 * time.Second), "inference"
}

func validRequestOutcome(value string) bool {
	return value == "success" || value == "error" || value == "partial" || value == "cancelled" || value == "indeterminate"
}
