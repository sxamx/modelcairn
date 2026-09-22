package storage

import (
	"context"
	"testing"
	"time"
)

func TestOperationalMetricsUseRetainedRowsAndKnownTokenUsage(t *testing.T) {
	repository, installation := repositoryForTest(t)
	items := seedRepositoryGraph(t, repository, installation)
	recorder := NewOperationalRecorder(installation.DB())
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	for index, tokens := range []bool{true, false} {
		start := now.Add(-time.Duration(index) * time.Hour)
		id := []string{"priced", "missing-usage"}[index]
		if err := recorder.Begin(context.Background(), RequestStart{ID: id, AgentTokenID: items[KindAgentToken].ID, RouteID: items[KindRoute].ID, RequestedAlias: "assistant", StartedAt: start}); err != nil {
			t.Fatal(err)
		}
		completion := RequestCompletion{Outcome: "success", HTTPStatus: 200, CompletedAt: start.Add(time.Second), Attempts: []AttemptCompletion{{Sequence: 1, DestinationID: items[KindDestination].ID, Outcome: "success", ProviderStatus: 200, StartedAt: start, CompletedAt: start.Add(time.Second)}}}
		if tokens {
			input, output := 120, 30
			completion.InputTokens, completion.OutputTokens = &input, &output
		}
		if err := recorder.Complete(context.Background(), id, completion); err != nil {
			t.Fatal(err)
		}
	}
	metrics, err := ReadOperationalMetrics(context.Background(), installation.DB(), now)
	if err != nil {
		t.Fatal(err)
	}
	if metrics.Requests != 2 || metrics.UsageKnownRequests != 1 || metrics.InputTokens != 120 || metrics.OutputTokens != 30 || len(metrics.Daily) != 1 || metrics.Daily[0].Requests != 2 {
		t.Fatalf("unexpected totals: %+v", metrics)
	}
	if len(metrics.Models) != 1 || metrics.Models[0].Name != "model" || metrics.Models[0].Requests != 1 || metrics.Models[0].InputTokens != 120 {
		t.Fatalf("unexpected model attribution: %+v", metrics.Models)
	}
	if metrics.Models[0].AvgLatencyMillis == nil || *metrics.Models[0].AvgLatencyMillis != 1000 || metrics.Models[0].OutputTokensPerSecond == nil || *metrics.Models[0].OutputTokensPerSecond != 30 {
		t.Fatalf("unexpected observed performance: %+v", metrics.Models[0])
	}
}
