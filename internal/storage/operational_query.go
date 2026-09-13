package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type OperationalRequestQuery struct {
	Outcome, Alias, BeforeID string
	From, To, BeforeStarted  *time.Time
	Limit                    int
}

type OperationalRequestPage struct {
	Items                 []OperationalRequest
	NextStartedAt, NextID string
}

type OperationalRequest struct {
	ID             string     `json:"id"`
	RequestedAlias string     `json:"requestedAlias"`
	StartedAt      time.Time  `json:"startedAt"`
	CompletedAt    *time.Time `json:"completedAt"`
	Outcome        *string    `json:"outcome"`
	HTTPStatus     *int       `json:"httpStatus"`
	InputTokens    *int64     `json:"inputTokens"`
	OutputTokens   *int64     `json:"outputTokens"`
	TTFTMillis     *int64     `json:"ttftMillis"`
	DurationMillis *int64     `json:"durationMillis"`
	Attempts       int64      `json:"attempts"`
}

type OperationalAttempt struct {
	Sequence          int        `json:"sequence"`
	DestinationID     *string    `json:"destinationID"`
	StartedAt         time.Time  `json:"startedAt"`
	CompletedAt       *time.Time `json:"completedAt"`
	Outcome           string     `json:"outcome"`
	ErrorClass        *string    `json:"errorClass"`
	ProviderStatus    *int       `json:"providerStatus"`
	ProviderRequestID *string    `json:"providerRequestID"`
	Retryable         bool       `json:"retryable"`
	FallbackReason    *string    `json:"fallbackReason"`
}

func ListOperationalRequests(ctx context.Context, db *sql.DB, query OperationalRequestQuery) (OperationalRequestPage, error) {
	page := OperationalRequestPage{Items: []OperationalRequest{}}
	if db == nil || query.Limit < 1 || query.Limit > 200 || (query.Outcome != "" && !validRequestOutcome(query.Outcome)) || len(query.Alias) > 128 {
		return page, &RepositoryError{Code: CodeInvalidResource}
	}
	clauses, args := []string{"1=1"}, []any{}
	if query.Outcome != "" {
		clauses = append(clauses, "r.outcome=?")
		args = append(args, query.Outcome)
	}
	if query.Alias != "" {
		clauses = append(clauses, "r.requested_alias=?")
		args = append(args, query.Alias)
	}
	if query.From != nil {
		clauses = append(clauses, "r.started_at>=?")
		args = append(args, query.From.UTC().Format(time.RFC3339Nano))
	}
	if query.To != nil {
		clauses = append(clauses, "r.started_at<?")
		args = append(args, query.To.UTC().Format(time.RFC3339Nano))
	}
	if query.BeforeStarted != nil {
		if query.BeforeID == "" {
			return page, &RepositoryError{Code: CodeInvalidResource}
		}
		stamp := query.BeforeStarted.UTC().Format(time.RFC3339Nano)
		clauses = append(clauses, "(r.started_at<? OR (r.started_at=? AND r.id<?))")
		args = append(args, stamp, stamp, query.BeforeID)
	} else if query.BeforeID != "" {
		return page, &RepositoryError{Code: CodeInvalidResource}
	}
	args = append(args, query.Limit+1)
	rows, err := db.QueryContext(ctx, `SELECT r.id,r.requested_alias,r.started_at,r.completed_at,r.outcome,r.http_status,r.input_tokens,r.output_tokens,r.ttft_ms,r.duration_ms,count(a.id)
		FROM requests r LEFT JOIN attempts a ON a.request_id=r.id WHERE `+strings.Join(clauses, " AND ")+`
		GROUP BY r.id ORDER BY r.started_at DESC,r.id DESC LIMIT ?`, args...)
	if err != nil {
		return page, fmt.Errorf("list operational requests: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		item, err := scanOperationalRequest(rows)
		if err != nil {
			return page, err
		}
		page.Items = append(page.Items, item)
	}
	if err := rows.Err(); err != nil {
		return page, fmt.Errorf("iterate operational requests: %w", err)
	}
	if len(page.Items) > query.Limit {
		page.Items = page.Items[:query.Limit]
		last := page.Items[len(page.Items)-1]
		page.NextStartedAt, page.NextID = last.StartedAt.Format(time.RFC3339Nano), last.ID
	}
	return page, nil
}

type operationalRowScanner interface{ Scan(...any) error }

func scanOperationalRequest(row operationalRowScanner) (OperationalRequest, error) {
	var item OperationalRequest
	var started string
	var completed, outcome sql.NullString
	var status, input, output, ttft, duration sql.NullInt64
	if err := row.Scan(&item.ID, &item.RequestedAlias, &started, &completed, &outcome, &status, &input, &output, &ttft, &duration, &item.Attempts); err != nil {
		return item, fmt.Errorf("scan operational request: %w", err)
	}
	parsed, err := time.Parse(time.RFC3339Nano, started)
	if err != nil {
		return item, fmt.Errorf("parse operational request start: %w", err)
	}
	item.StartedAt = parsed
	if completed.Valid {
		value, err := time.Parse(time.RFC3339Nano, completed.String)
		if err != nil {
			return item, fmt.Errorf("parse operational request completion: %w", err)
		}
		item.CompletedAt = &value
	}
	if outcome.Valid {
		item.Outcome = &outcome.String
	}
	if status.Valid {
		value := int(status.Int64)
		item.HTTPStatus = &value
	}
	if input.Valid {
		item.InputTokens = &input.Int64
	}
	if output.Valid {
		item.OutputTokens = &output.Int64
	}
	if ttft.Valid {
		item.TTFTMillis = &ttft.Int64
	}
	if duration.Valid {
		item.DurationMillis = &duration.Int64
	}
	return item, nil
}

func ListOperationalAttempts(ctx context.Context, db *sql.DB, requestID string) ([]OperationalAttempt, error) {
	if db == nil || requestID == "" || len(requestID) > 128 {
		return nil, &RepositoryError{Code: CodeInvalidResource}
	}
	rows, err := db.QueryContext(ctx, `SELECT sequence,destination_id,started_at,completed_at,outcome,error_class,provider_status,provider_request_id,retryable,fallback_reason FROM attempts WHERE request_id=? ORDER BY sequence LIMIT 32`, requestID)
	if err != nil {
		return nil, fmt.Errorf("list operational attempts: %w", err)
	}
	defer rows.Close()
	items := []OperationalAttempt{}
	for rows.Next() {
		var item OperationalAttempt
		var destination, completed, errorClass, providerID, fallback sql.NullString
		var started string
		var status sql.NullInt64
		if err := rows.Scan(&item.Sequence, &destination, &started, &completed, &item.Outcome, &errorClass, &status, &providerID, &item.Retryable, &fallback); err != nil {
			return nil, fmt.Errorf("scan operational attempt: %w", err)
		}
		item.StartedAt, err = time.Parse(time.RFC3339Nano, started)
		if err != nil {
			return nil, fmt.Errorf("parse attempt start: %w", err)
		}
		if destination.Valid {
			item.DestinationID = &destination.String
		}
		if completed.Valid {
			value, parseErr := time.Parse(time.RFC3339Nano, completed.String)
			if parseErr != nil {
				return nil, fmt.Errorf("parse attempt completion: %w", parseErr)
			}
			item.CompletedAt = &value
		}
		if errorClass.Valid {
			item.ErrorClass = &errorClass.String
		}
		if status.Valid {
			value := int(status.Int64)
			item.ProviderStatus = &value
		}
		if providerID.Valid {
			item.ProviderRequestID = &providerID.String
		}
		if fallback.Valid {
			item.FallbackReason = &fallback.String
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate operational attempts: %w", err)
	}
	return items, nil
}
