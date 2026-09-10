package storage

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestSettingsPlanPurposeIsolation(t *testing.T) {
	ctx := context.Background()
	i, err := OpenInstallation(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer i.Close()
	s := i.Secrets()
	// Identical bindings intentionally prove isolation is not just a digest check.
	binding := testPlanBinding()
	snapshot := func(*sql.Tx) (PlanBinding, error) { return binding, nil }
	configToken, err := s.CreatePlanToken(ctx, time.Now(), snapshot)
	if err != nil {
		t.Fatal(err)
	}
	settingsToken, err := s.CreateSettingsPlanToken(ctx, time.Now(), snapshot)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	apply := func(*sql.Tx) error { called = true; return nil }
	if err := s.ExecuteSettingsPlan(ctx, configToken, snapshot, apply); !errors.Is(err, ErrInvalidPlan) {
		t.Fatalf("config token accepted by settings: %v", err)
	}
	if err := s.ExecutePlan(ctx, settingsToken, snapshot, apply); !errors.Is(err, ErrInvalidPlan) {
		t.Fatalf("settings token accepted by config: %v", err)
	}
	if called {
		t.Fatal("cross-purpose attempt reached mutation")
	}
	var count int
	if err := i.DB().QueryRow("SELECT count(*) FROM consumed_plan_tokens").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("cross-purpose attempt consumed a token")
	}
	if err := s.ExecutePlan(ctx, configToken, snapshot, apply); err != nil {
		t.Fatal(err)
	}
	if err := s.ExecuteSettingsPlan(ctx, settingsToken, snapshot, apply); err != nil {
		t.Fatal(err)
	}
	if err := s.ExecuteSettingsPlan(ctx, settingsToken, snapshot, apply); !errors.Is(err, ErrPlanAlreadyUsed) {
		t.Fatalf("replay: %v", err)
	}
}

func TestSettingsPlanRollbackAndBindings(t *testing.T) {
	ctx := context.Background()
	i, err := OpenInstallation(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer i.Close()
	s := i.Secrets()
	binding := testPlanBinding()
	snapshot := func(*sql.Tx) (PlanBinding, error) { return binding, nil }
	token, err := s.CreateSettingsPlanToken(ctx, time.Now(), snapshot)
	if err != nil {
		t.Fatal(err)
	}
	changed := binding
	changed.Digest = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	if err := s.ExecuteSettingsPlan(ctx, token, func(*sql.Tx) (PlanBinding, error) { return changed, nil }, func(*sql.Tx) error { t.Fatal("different document applied"); return nil }); !errors.Is(err, ErrInvalidPlan) {
		t.Fatalf("digest mismatch: %v", err)
	}
	changed = binding
	changed.Revision++
	if err := s.ExecuteSettingsPlan(ctx, token, func(*sql.Tx) (PlanBinding, error) { return changed, nil }, func(*sql.Tx) error { t.Fatal("stale revision applied"); return nil }); !IsRepositoryCode(err, CodeVersionConflict) {
		t.Fatalf("revision mismatch: %v", err)
	}
	injected := errors.New("injected_audit_failure")
	err = s.ExecuteSettingsPlan(ctx, token, snapshot, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "INSERT INTO admin_settings VALUES(1,1,'{}','test')"); err != nil {
			return err
		}
		return injected
	})
	if !errors.Is(err, injected) {
		t.Fatalf("injected failure: %v", err)
	}
	var rows int
	if err := i.DB().QueryRow("SELECT (SELECT count(*) FROM admin_settings)+(SELECT count(*) FROM consumed_plan_tokens)").Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Fatal("failure left settings or consumed nonce behind")
	}
	if err := s.ExecuteSettingsPlan(ctx, token, snapshot, func(*sql.Tx) error { return nil }); err != nil {
		t.Fatalf("retry after rollback: %v", err)
	}
}

func TestSettingsPlanExpiryAndConcurrentUse(t *testing.T) {
	ctx := context.Background()
	i, err := OpenInstallation(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer i.Close()
	s := i.Secrets()
	binding := testPlanBinding()
	snapshot := func(*sql.Tx) (PlanBinding, error) { return binding, nil }
	expired, err := s.CreateSettingsPlanToken(ctx, time.Now().Add(-10*time.Minute), snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ExecuteSettingsPlan(ctx, expired, snapshot, func(*sql.Tx) error { t.Fatal("expired mutation"); return nil }); !errors.Is(err, ErrPlanExpired) {
		t.Fatalf("expiry: %v", err)
	}
	token, err := s.CreateSettingsPlanToken(ctx, time.Now(), snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Go(func() { results <- s.ExecuteSettingsPlan(ctx, token, snapshot, func(*sql.Tx) error { return nil }) })
	}
	wg.Wait()
	close(results)
	successes, replays := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, ErrPlanAlreadyUsed) {
			replays++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || replays != 1 {
		t.Fatalf("successes=%d replays=%d", successes, replays)
	}
}
