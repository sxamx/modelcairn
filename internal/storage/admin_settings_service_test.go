package storage

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/sxamx/modelcairn/internal/adminsettings"
)

func TestAdminSettingsServicePlanApplyLifecycle(t *testing.T) {
	ctx := context.Background()
	i, err := OpenInstallation(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer i.Close()
	spec := adminsettings.Defaults()
	spec.PublicOrigin = "http://127.0.0.1:8080"
	tx, err := i.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := InsertAdminSettingsTx(ctx, tx, spec, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	s, err := NewAdminSettingsService(i)
	if err != nil {
		t.Fatal(err)
	}
	input := func(version, idle int) []byte {
		return []byte(fmt.Sprintf(`{"apiVersion":"modelcairn.io/v1alpha1","kind":"AdminSettings","resourceVersion":%d,"spec":{"idleSeconds":%d}}`, version, idle))
	}
	request := input(1, 600)
	before := time.Now()
	plan, err := s.Plan(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Token == "" || !plan.ExpiresAt.After(before) || plan.ExpiresAt.After(time.Now().Add(10*time.Minute)) || len(plan.ChangedFields) != 1 || plan.ChangedFields[0] != "idleSeconds" {
		t.Fatal("invalid plan metadata")
	}
	if _, err := s.Apply(ctx, input(1, 900), plan.Token, Actor{Type: "cli"}); err == nil {
		t.Fatal("altered plan accepted")
	}
	state, err := s.State(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.Desired.ResourceVersion != 1 || state.RestartRequired {
		t.Fatal("planning or rejection changed state")
	}
	result, err := s.Apply(ctx, request, plan.Token, Actor{Type: "cli"})
	if err != nil {
		t.Fatal(err)
	}
	if result.State.Desired.ResourceVersion != 2 || result.State.Desired.Spec.IdleSeconds != 600 || result.State.Effective.Spec.IdleSeconds != 1800 || !result.State.RestartRequired || result.AppliedAt.IsZero() {
		t.Fatal("incorrect desired/effective state")
	}
	if _, err := s.Apply(ctx, request, plan.Token, Actor{Type: "cli"}); !errors.Is(err, ErrPlanAlreadyUsed) {
		t.Fatalf("replay error: %v", err)
	}
	if _, err := s.Apply(ctx, request, "invalid", Actor{Type: "cli"}); !errors.Is(err, ErrInvalidPlan) {
		t.Fatalf("stale input skipped authentication: %v", err)
	}
	if _, err := s.Plan(ctx, request); !IsRepositoryCode(err, CodeVersionConflict) {
		t.Fatalf("stale plan: %v", err)
	}
	noop := input(2, 600)
	p, err := s.Plan(ctx, noop)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ChangedFields) != 0 {
		t.Fatal("no-op reports changes")
	}
	r, err := s.Apply(ctx, noop, p.Token, Actor{Type: "cli"})
	if err != nil {
		t.Fatal(err)
	}
	if r.State.Desired.ResourceVersion != 2 || !r.State.Desired.UpdatedAt.Equal(result.State.Desired.UpdatedAt) {
		t.Fatal("no-op changed version or timestamp")
	}
	if _, err := s.Apply(ctx, noop, p.Token, Actor{Type: "cli"}); err == nil {
		t.Fatal("no-op replay accepted")
	}
	// A newly started service adopts persisted settings, not the old snapshot.
	restarted, err := NewAdminSettingsService(i)
	if err != nil {
		t.Fatal(err)
	}
	state, err = restarted.State(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.RestartRequired || state.Effective.Spec.IdleSeconds != 600 {
		t.Fatal("restart did not adopt settings")
	}
	// Reverting to the running values needs no restart despite a new version.
	revert := input(2, 1800)
	p, err = s.Plan(ctx, revert)
	if err != nil {
		t.Fatal(err)
	}
	r, err = s.Apply(ctx, revert, p.Token, Actor{Type: "cli"})
	if err != nil {
		t.Fatal(err)
	}
	if r.State.RestartRequired || r.State.Desired.ResourceVersion != 3 {
		t.Fatal("revert confused version with effective values")
	}
}
