package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func testPlanBinding() PlanBinding {
	digest := sha256.Sum256([]byte("desired operation"))
	return PlanBinding{Revision: 1, Digest: base64.RawURLEncoding.EncodeToString(digest[:]),
		Observed: []ObservedResource{{Kind: KindProvider, Name: "provider"}}}
}

func TestPlanTokenSignedInvalidClaims(t *testing.T) {
	i, err := OpenInstallation(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer i.Close()
	s := i.Secrets()
	now := time.Unix(1800000000, 0)
	binding := testPlanBinding()
	token, err := s.IssuePlanToken(binding, now)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _, _ := strings.Cut(token, ".")
	payload, _ := base64.RawURLEncoding.DecodeString(encoded)
	var original planClaims
	if err := json.Unmarshal(payload, &original); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*planClaims){
		"version":      func(c *planClaims) { c.Version++ },
		"purpose":      func(c *planClaims) { c.Purpose = "another-purpose" },
		"installation": func(c *planClaims) { c.InstallationID = "another-installation" },
		"key-version":  func(c *planClaims) { c.KeyVersion++ },
		"nonce":        func(c *planClaims) { c.Nonce = "short" },
		"lifetime":     func(c *planClaims) { c.ExpiresAt++ },
	} {
		t.Run(name, func(t *testing.T) {
			claims := original
			mutate(&claims)
			raw, _ := json.Marshal(claims)
			s.mu.RLock()
			defer s.mu.RUnlock()
			body := base64.RawURLEncoding.EncodeToString(raw)
			mac, err := s.planMAC(body)
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.verifyPlanTokenLocked(body+"."+base64.RawURLEncoding.EncodeToString(mac), binding, now)
			if !errors.Is(err, ErrInvalidPlan) {
				t.Fatal("signed invalid claims accepted", err)
			}
		})
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	body := base64.RawURLEncoding.EncodeToString(append(payload, ' '))
	mac, err := s.planMAC(body)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.verifyPlanTokenLocked(body+"."+base64.RawURLEncoding.EncodeToString(mac), binding, now); !errors.Is(err, ErrInvalidPlan) {
		t.Fatal("noncanonical payload accepted", err)
	}
}

func TestExecutePlanExpiryDuringSnapshot(t *testing.T) {
	ctx := context.Background()
	i, err := OpenInstallation(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer i.Close()
	s := i.Secrets()
	now := time.Now()
	binding := testPlanBinding()
	token, err := s.IssuePlanToken(binding, now)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	err = s.executePlan(ctx, token, func() time.Time { return now }, func(tx *sql.Tx) (PlanBinding, error) {
		now = now.Add(10 * time.Minute)
		return binding, nil
	}, func(tx *sql.Tx) error { called = true; return nil })
	if !errors.Is(err, ErrPlanExpired) || called {
		t.Fatal("expired waiting plan executed", err)
	}
	var count int
	if err := i.DB().QueryRowContext(ctx, "SELECT count(*) FROM consumed_plan_tokens").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("expired token consumed")
	}
}

func TestExecutePlanConcurrentConsumption(t *testing.T) {
	ctx := context.Background()
	i, err := OpenInstallation(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer i.Close()
	s := i.Secrets()
	binding := testPlanBinding()
	token, err := s.IssuePlanToken(binding, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Go(func() {
			results <- s.ExecutePlan(ctx, token, func(tx *sql.Tx) (PlanBinding, error) { return binding, nil }, func(tx *sql.Tx) error { return nil })
		})
	}
	wg.Wait()
	close(results)
	success, reused := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrPlanAlreadyUsed) {
			reused++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || reused != 1 {
		t.Fatalf("success=%d reused=%d", success, reused)
	}
}

func TestPlanTokenBindingsAndClock(t *testing.T) {
	ctx := context.Background()
	i, err := OpenInstallation(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer i.Close()
	s := i.Secrets()
	now := time.Unix(1800000000, 0)
	binding := testPlanBinding()
	token, err := s.IssuePlanToken(binding, now)
	if err != nil {
		t.Fatal(err)
	}
	verify := func(token string, b PlanBinding, at time.Time) error {
		s.mu.RLock()
		defer s.mu.RUnlock()
		_, err := s.verifyPlanTokenLocked(token, b, at)
		return err
	}
	if err := verify(token, binding, now); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", token + "x", "x" + token, strings.Repeat("x", maxPlanTokenBytes+1)} {
		if !errors.Is(verify(bad, binding, now), ErrInvalidPlan) {
			t.Fatal("tampered token accepted")
		}
	}
	if !errors.Is(verify(token, binding, now.Add(10*time.Minute)), ErrPlanExpired) {
		t.Fatal("expiry boundary accepted")
	}
	if err := verify(token, binding, now.Add(-30*time.Second)); err != nil {
		t.Fatal("allowed clock skew rejected", err)
	}
	if !errors.Is(verify(token, binding, now.Add(-31*time.Second)), ErrInvalidPlan) {
		t.Fatal("future token accepted")
	}
	stale := binding
	stale.Revision++
	if !IsRepositoryCode(verify(token, stale, now), CodeVersionConflict) {
		t.Fatal("revision mismatch accepted")
	}
	stale = binding
	stale.Observed = []ObservedResource{{Kind: KindProvider, Name: "provider", ID: "new-id", Version: 1}}
	if !IsRepositoryCode(verify(token, stale, now), CodeVersionConflict) {
		t.Fatal("absence mismatch accepted")
	}
	other := sha256.Sum256([]byte("other operation"))
	stale = binding
	stale.Digest = base64.RawURLEncoding.EncodeToString(other[:])
	if !errors.Is(verify(token, stale, now), ErrInvalidPlan) {
		t.Fatal("different desired operation accepted")
	}
	otherInstallation, err := OpenInstallation(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer otherInstallation.Close()
	otherStore := otherInstallation.Secrets()
	otherStore.mu.RLock()
	_, err = otherStore.verifyPlanTokenLocked(token, binding, now)
	otherStore.mu.RUnlock()
	if !errors.Is(err, ErrInvalidPlan) {
		t.Fatal("cross-installation token accepted")
	}
	if _, err := s.RotateMasterKey(ctx, Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(verify(token, binding, now), ErrInvalidPlan) {
		t.Fatal("old key token accepted")
	}
}

func TestExecutePlanAtomicConsumption(t *testing.T) {
	ctx := context.Background()
	i, err := OpenInstallation(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer i.Close()
	s := i.Secrets()
	now := time.Now()
	binding := testPlanBinding()
	token, err := s.IssuePlanToken(binding, now)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := func(tx *sql.Tx) (PlanBinding, error) { return binding, nil }
	failed := errors.New("injected failure")
	err = s.executePlan(ctx, token, func() time.Time { return now }, snapshot, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "UPDATE installation_state SET config_revision=2"); err != nil {
			return err
		}
		return failed
	})
	if !errors.Is(err, failed) {
		t.Fatal(err)
	}
	var revision, count int
	if err := i.DB().QueryRowContext(ctx, "SELECT config_revision FROM installation_state").Scan(&revision); err != nil {
		t.Fatal(err)
	}
	if err := i.DB().QueryRowContext(ctx, "SELECT count(*) FROM consumed_plan_tokens").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if revision != 1 || count != 0 {
		t.Fatal("failed application was not rolled back")
	}
	noop := func(tx *sql.Tx) error { return nil }
	if err := s.executePlan(ctx, token, func() time.Time { return now }, snapshot, noop); err != nil {
		t.Fatal(err)
	}
	called := false
	err = s.executePlan(ctx, token, func() time.Time { return now }, snapshot, func(tx *sql.Tx) error { called = true; return nil })
	if !errors.Is(err, ErrPlanAlreadyUsed) || called {
		t.Fatal("consumed no-op token reused", err)
	}
}
