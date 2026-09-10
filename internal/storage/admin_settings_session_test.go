package storage

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSettingsSessionAuthorizationAndAtomicRollback(t *testing.T) {
	ctx := context.Background()
	i, admin := sessionFixture(t)
	defer i.Close()
	s, err := NewAdminSettingsService(i)
	if err != nil {
		t.Fatal(err)
	}
	credentials, err := CreateAdminSession(ctx, i, VerifiedAdmin{admin.ID, admin.Username, admin.AuthVersion}, 1800, 3600, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	input := []byte(`{"apiVersion":"modelcairn.io/v1alpha1","kind":"AdminSettings","resourceVersion":1,"spec":{"idleSeconds":600}}`)
	plan, err := s.Plan(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	var before string
	if err := i.DB().QueryRow("SELECT last_seen_at FROM admin_sessions").Scan(&before); err != nil {
		t.Fatal(err)
	}
	assertUnchanged := func() {
		t.Helper()
		var after string
		var nonces, audits int
		if err := i.DB().QueryRow("SELECT last_seen_at FROM admin_sessions").Scan(&after); err != nil {
			t.Fatal(err)
		}
		if err := i.DB().QueryRow("SELECT count(*) FROM consumed_plan_tokens").Scan(&nonces); err != nil {
			t.Fatal(err)
		}
		if err := i.DB().QueryRow("SELECT count(*) FROM audit_events WHERE action='admin_settings.apply'").Scan(&audits); err != nil {
			t.Fatal(err)
		}
		state, err := s.State(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if after != before || nonces != 0 || audits != 0 || state.Desired.ResourceVersion != 1 {
			t.Fatal("rejected operation left side effects")
		}
	}
	if _, err := s.Apply(ctx, input, plan.Token, Actor{Type: "admin", ID: admin.ID}); !errors.Is(err, ErrAdminSessionInvalid) {
		t.Fatalf("identity bypass: %v", err)
	}
	if _, err := s.ApplySession(ctx, input, plan.Token, credentials.SessionToken, credentials.SessionToken); !errors.Is(err, ErrAdminSessionInvalid) {
		t.Fatalf("wrong CSRF: %v", err)
	}
	assertUnchanged()
	if _, err := i.DB().Exec(`CREATE TRIGGER reject_settings_audit BEFORE INSERT ON audit_events WHEN NEW.action='admin_settings.apply' BEGIN SELECT RAISE(ABORT,'test audit failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplySession(ctx, input, plan.Token, credentials.SessionToken, credentials.CSRFToken); err == nil {
		t.Fatal("audit failure accepted")
	}
	assertUnchanged()
	if _, err := i.DB().Exec("DROP TRIGGER reject_settings_audit"); err != nil {
		t.Fatal(err)
	}
	result, err := s.ApplySession(ctx, input, plan.Token, credentials.SessionToken, credentials.CSRFToken)
	if err != nil {
		t.Fatal(err)
	}
	if result.State.Desired.ResourceVersion != 2 {
		t.Fatal("settings not applied")
	}
	var actorID, actorType string
	if err := i.DB().QueryRow("SELECT actor_id,actor_type FROM audit_events WHERE action='admin_settings.apply'").Scan(&actorID, &actorType); err != nil {
		t.Fatal(err)
	}
	if actorID != admin.ID || actorType != "admin" {
		t.Fatal("audit identity not derived from session")
	}
	if _, err := s.ApplySession(ctx, input, plan.Token, credentials.SessionToken, credentials.CSRFToken); !errors.Is(err, ErrPlanAlreadyUsed) {
		t.Fatalf("replay: %v", err)
	}
}

// Reproduce the dangerous interleaving deterministically: middleware validates,
// password reset commits, then the handler attempts its settings mutation.
func TestSettingsApplyRejectsSessionValidatedBeforePasswordReset(t *testing.T) {
	ctx := context.Background()
	i, admin := sessionFixture(t)
	defer i.Close()
	s, err := NewAdminSettingsService(i)
	if err != nil {
		t.Fatal(err)
	}
	credentials, err := CreateAdminSession(ctx, i, VerifiedAdmin{admin.ID, admin.Username, admin.AuthVersion}, 1800, 3600, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	input := []byte(`{"apiVersion":"modelcairn.io/v1alpha1","kind":"AdminSettings","resourceVersion":1,"spec":{"idleSeconds":600}}`)
	plan, err := s.Plan(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := UseAdminSession(ctx, i, credentials.SessionToken, credentials.CSRFToken, true, time.Now()); err != nil {
		t.Fatal(err)
	}
	// Hold the plan lock so reset commits while the HTTP mutation is queued.
	done := make(chan error, 1)
	i.Secrets().mu.Lock()
	go func() {
		_, err := s.ApplySession(ctx, input, plan.Token, credentials.SessionToken, credentials.CSRFToken)
		done <- err
	}()
	_, resetErr := ResetAdminPassword(ctx, i, []byte("another secure password"))
	i.Secrets().mu.Unlock()
	applyErr := <-done
	if resetErr != nil {
		t.Fatal(resetErr)
	}
	if err := applyErr; !errors.Is(err, ErrAdminSessionInvalid) {
		t.Fatalf("stale authorization accepted: %v", err)
	}
	state, err := s.State(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var nonces int
	if err := i.DB().QueryRow("SELECT count(*) FROM consumed_plan_tokens").Scan(&nonces); err != nil {
		t.Fatal(err)
	}
	if state.Desired.ResourceVersion != 1 || nonces != 0 {
		t.Fatal("reset session mutated state")
	}
}
