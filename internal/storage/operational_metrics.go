package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// OperationalMetrics summarizes only the operational rows still retained locally.
// Pricing is deliberately excluded: current model tariffs are not historical invoices.
type OperationalMetrics struct {
	Requests           int64              `json:"requests"`
	InputTokens        int64              `json:"inputTokens"`
	OutputTokens       int64              `json:"outputTokens"`
	UsageKnownRequests int64              `json:"usageKnownRequests"`
	Daily              []OperationalDay   `json:"daily"`
	Models             []OperationalModel `json:"models"`
	GeneratedAt        time.Time          `json:"generatedAt"`
}

type OperationalDay struct {
	Date         string `json:"date"`
	Requests     int64  `json:"requests"`
	Success      int64  `json:"success"`
	InputTokens  int64  `json:"inputTokens"`
	OutputTokens int64  `json:"outputTokens"`
}

type OperationalModel struct {
	Name                  string   `json:"name"`
	Requests              int64    `json:"requests"`
	InputTokens           int64    `json:"inputTokens"`
	OutputTokens          int64    `json:"outputTokens"`
	AvgLatencyMillis      *float64 `json:"avgLatencyMillis"`
	OutputTokensPerSecond *float64 `json:"outputTokensPerSecond"`
}

func ReadOperationalMetrics(ctx context.Context, db *sql.DB, now time.Time) (OperationalMetrics, error) {
	result := OperationalMetrics{Daily: []OperationalDay{}, Models: []OperationalModel{}, GeneratedAt: now.UTC()}
	if db == nil || now.IsZero() {
		return result, &RepositoryError{Code: CodeInvalidResource}
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*),coalesce(sum(input_tokens),0),coalesce(sum(output_tokens),0),
		coalesce(sum(input_tokens IS NOT NULL AND output_tokens IS NOT NULL),0) FROM requests`).Scan(
		&result.Requests, &result.InputTokens, &result.OutputTokens, &result.UsageKnownRequests); err != nil {
		return result, fmt.Errorf("summarize retained usage: %w", err)
	}
	since := now.UTC().AddDate(0, 0, -89).Format("2006-01-02")
	until := now.UTC().AddDate(0, 0, 1).Format("2006-01-02")
	days, err := db.QueryContext(ctx, `SELECT substr(started_at,1,10),count(*),coalesce(sum(outcome='success'),0),
		coalesce(sum(input_tokens),0),coalesce(sum(output_tokens),0)
		FROM requests WHERE started_at>=? AND started_at<? GROUP BY substr(started_at,1,10) ORDER BY 1`, since, until)
	if err != nil {
		return result, fmt.Errorf("summarize daily usage: %w", err)
	}
	for days.Next() {
		var day OperationalDay
		if err := days.Scan(&day.Date, &day.Requests, &day.Success, &day.InputTokens, &day.OutputTokens); err != nil {
			days.Close()
			return result, fmt.Errorf("scan daily usage: %w", err)
		}
		result.Daily = append(result.Daily, day)
	}
	if err := days.Err(); err != nil {
		days.Close()
		return result, fmt.Errorf("iterate daily usage: %w", err)
	}
	days.Close()
	models, err := db.QueryContext(ctx, `SELECT m.name,count(*),coalesce(sum(r.input_tokens),0),coalesce(sum(r.output_tokens),0),avg(r.duration_ms),
		1000.0 * sum(CASE WHEN r.duration_ms>0 THEN r.output_tokens ELSE 0 END) /
		nullif(sum(CASE WHEN r.duration_ms>0 THEN r.duration_ms ELSE 0 END),0)
		FROM requests r
		JOIN attempts a ON a.request_id=r.id AND a.sequence=(SELECT max(sequence) FROM attempts WHERE request_id=r.id)
		JOIN destinations d ON d.resource_id=a.destination_id
		JOIN resources m ON m.id=d.model_id AND m.kind='Model'
		WHERE r.input_tokens IS NOT NULL AND r.output_tokens IS NOT NULL
		GROUP BY m.id ORDER BY count(*) DESC,m.name`)
	if err != nil {
		return result, fmt.Errorf("summarize model usage: %w", err)
	}
	defer models.Close()
	for models.Next() {
		var model OperationalModel
		var latency, throughput sql.NullFloat64
		if err := models.Scan(&model.Name, &model.Requests, &model.InputTokens, &model.OutputTokens, &latency, &throughput); err != nil {
			return result, fmt.Errorf("scan model usage: %w", err)
		}
		if latency.Valid {
			model.AvgLatencyMillis = &latency.Float64
		}
		if throughput.Valid {
			model.OutputTokensPerSecond = &throughput.Float64
		}
		result.Models = append(result.Models, model)
	}
	if err := models.Err(); err != nil {
		return result, fmt.Errorf("iterate model usage: %w", err)
	}
	return result, nil
}
