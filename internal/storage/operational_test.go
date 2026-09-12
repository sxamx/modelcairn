package storage

import (
	"context"
	"testing"
	"time"
)

func TestOperationalRecorderCompletesRequestAndAttemptsWithoutContent(t *testing.T) {
	repository, installation := repositoryForTest(t)
	items := seedRepositoryGraph(t, repository, installation)
	recorder := NewOperationalRecorder(installation.DB())
	started := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	if err := recorder.Begin(context.Background(), RequestStart{ID: "req_fixture", AgentTokenID: items[KindAgentToken].ID, RouteID: items[KindRoute].ID, RequestedAlias: "assistant", StartedAt: started}); err != nil {
		t.Fatal(err)
	}
	if err := recorder.Complete(context.Background(), "req_fixture", RequestCompletion{
		Outcome: "success", HTTPStatus: 200, CompletedAt: started.Add(250 * time.Millisecond),
		Attempts: []AttemptCompletion{{Sequence: 1, DestinationID: items[KindDestination].ID, Outcome: "success", ProviderStatus: 200, ProviderRequestID: "upstream-1", StartedAt: started.Add(10 * time.Millisecond), CompletedAt: started.Add(240 * time.Millisecond)}},
	}); err != nil {
		t.Fatal(err)
	}
	var outcome string
	var duration, contentStored, attempts int
	if err := installation.DB().QueryRow("SELECT outcome,duration_ms,content_stored FROM requests WHERE id='req_fixture'").Scan(&outcome, &duration, &contentStored); err != nil {
		t.Fatal(err)
	}
	if err := installation.DB().QueryRow("SELECT count(*) FROM attempts WHERE request_id='req_fixture'").Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if outcome != "success" || duration != 250 || contentStored != 0 || attempts != 1 {
		t.Fatalf("outcome=%s duration=%d content=%d attempts=%d", outcome, duration, contentStored, attempts)
	}
}

func TestOperationalCompletionRollsBackAllAttemptsOnFailure(t *testing.T) {
	repository, installation := repositoryForTest(t)
	items := seedRepositoryGraph(t, repository, installation)
	recorder := NewOperationalRecorder(installation.DB())
	started := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	if err := recorder.Begin(context.Background(), RequestStart{ID: "req_atomic", AgentTokenID: items[KindAgentToken].ID, RouteID: items[KindRoute].ID, RequestedAlias: "assistant", StartedAt: started}); err != nil {
		t.Fatal(err)
	}
	err := recorder.Complete(context.Background(), "req_atomic", RequestCompletion{Outcome: "error", HTTPStatus: 503, CompletedAt: started.Add(time.Second), Attempts: []AttemptCompletion{
		{Sequence: 1, DestinationID: items[KindDestination].ID, Outcome: "error", ProviderStatus: 503, Retryable: true, StartedAt: started, CompletedAt: started.Add(100 * time.Millisecond)},
		{Sequence: 2, DestinationID: "missing-destination", Outcome: "error", ProviderStatus: 503, StartedAt: started.Add(100 * time.Millisecond), CompletedAt: started.Add(200 * time.Millisecond)},
	}})
	if err == nil {
		t.Fatal("completion unexpectedly succeeded")
	}
	var attempts int
	var completed *string
	if err := installation.DB().QueryRow("SELECT completed_at FROM requests WHERE id='req_atomic'").Scan(&completed); err != nil {
		t.Fatal(err)
	}
	if err := installation.DB().QueryRow("SELECT count(*) FROM attempts WHERE request_id='req_atomic'").Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if completed != nil || attempts != 0 {
		t.Fatalf("completed=%v attempts=%d", completed, attempts)
	}
}

func TestOperationalRecorderNormalizesRateLimitAndCreatesCooldown(t *testing.T) {
	repository, installation := repositoryForTest(t)
	items := seedRepositoryGraph(t, repository, installation)
	recorder := NewOperationalRecorder(installation.DB())
	started := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	if err := recorder.Begin(context.Background(), RequestStart{ID: "req_limited", AgentTokenID: items[KindAgentToken].ID, RouteID: items[KindRoute].ID, RequestedAlias: "assistant", StartedAt: started}); err != nil {
		t.Fatal(err)
	}
	completed := started.Add(time.Second)
	if err := recorder.Complete(context.Background(), "req_limited", RequestCompletion{Outcome: "error", HTTPStatus: 429, CompletedAt: completed, Attempts: []AttemptCompletion{
		{Sequence: 1, DestinationID: items[KindDestination].ID, Outcome: "error", ProviderStatus: 429, Retryable: true, RetryAfter: "120", StartedAt: started, CompletedAt: completed},
	}}); err != nil {
		t.Fatal(err)
	}
	var source, resetsAt, endsAt string
	var rawStored int
	if err := installation.DB().QueryRow("SELECT source,resets_at,raw_content_stored FROM rate_limit_observations").Scan(&source, &resetsAt, &rawStored); err != nil {
		t.Fatal(err)
	}
	if err := installation.DB().QueryRow("SELECT ends_at FROM cooldowns WHERE scope_kind='destination' AND scope_resource_id=?", items[KindDestination].ID).Scan(&endsAt); err != nil {
		t.Fatal(err)
	}
	want := completed.Add(120 * time.Second).Format(time.RFC3339Nano)
	if source != "header" || resetsAt != want || endsAt != want || rawStored != 0 {
		t.Fatalf("source=%q reset=%q end=%q raw=%d", source, resetsAt, endsAt, rawStored)
	}
}

func TestRetryResetBoundsAndFallsBackWithoutRawRetention(t *testing.T) {
	observed := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	if reset, source := retryReset("invalid private header", observed); source != "inference" || reset != observed.Add(30*time.Second) {
		t.Fatalf("reset=%s source=%s", reset, source)
	}
	if reset, source := retryReset("999999999", observed); source != "header" || reset != observed.Add(24*time.Hour) {
		t.Fatalf("bounded reset=%s source=%s", reset, source)
	}
}
