package storage

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestOperationalRequestHistoryFiltersAndPaginatesWithoutDuplicates(t *testing.T) {
	repository, installation := repositoryForTest(t)
	items := seedRepositoryGraph(t, repository, installation)
	recorder := NewOperationalRecorder(installation.DB())
	base := time.Date(2026, 9, 13, 15, 0, 0, 0, time.UTC)
	for index := 0; index < 6; index++ {
		started := base.Add(-time.Duration(index/2) * time.Minute)
		id, outcome, status := fmt.Sprintf("history-%d", index), "success", 200
		if index%2 == 1 {
			outcome, status = "error", 503
		}
		if err := recorder.Begin(context.Background(), RequestStart{ID: id, AgentTokenID: items[KindAgentToken].ID, RouteID: items[KindRoute].ID, RequestedAlias: "assistant", StartedAt: started}); err != nil {
			t.Fatal(err)
		}
		if err := recorder.Complete(context.Background(), id, RequestCompletion{Outcome: outcome, HTTPStatus: status, CompletedAt: started.Add(time.Second), Attempts: []AttemptCompletion{{Sequence: 1, DestinationID: items[KindDestination].ID, Outcome: outcome, ProviderStatus: status, Retryable: outcome == "error", FallbackReason: "test", StartedAt: started, CompletedAt: started.Add(time.Second)}}}); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	query := OperationalRequestQuery{Limit: 2}
	for {
		page, err := ListOperationalRequests(context.Background(), installation.DB(), query)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range page.Items {
			if seen[item.ID] {
				t.Fatalf("duplicate %s", item.ID)
			}
			seen[item.ID] = true
		}
		if page.NextID == "" {
			break
		}
		stamp, err := time.Parse(time.RFC3339Nano, page.NextStartedAt)
		if err != nil {
			t.Fatal(err)
		}
		query.BeforeStarted = &stamp
		query.BeforeID = page.NextID
	}
	if len(seen) != 6 {
		t.Fatalf("seen=%d", len(seen))
	}
	filtered, err := ListOperationalRequests(context.Background(), installation.DB(), OperationalRequestQuery{Limit: 10, Outcome: "error", Alias: "assistant"})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Items) != 3 {
		t.Fatalf("filtered=%d", len(filtered.Items))
	}
	attempts, err := ListOperationalAttempts(context.Background(), installation.DB(), "history-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 1 || attempts[0].ProviderStatus == nil || *attempts[0].ProviderStatus != 503 || attempts[0].FallbackReason == nil {
		t.Fatalf("attempts=%+v", attempts)
	}
}
