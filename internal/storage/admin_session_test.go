package storage

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func sessionFixture(t *testing.T) (*Installation, AdminIdentity) {
	t.Helper()
	i, spec := bootstrapFixture(t)
	admin, _, err := BootstrapAdmin(context.Background(), i, "owner", []byte("a secure password"), spec)
	if err != nil {
		i.Close()
		t.Fatal(err)
	}
	return i, admin
}

func TestAdminSessionLifecycleAndCSRFWindow(t *testing.T) {
	ctx := context.Background()
	i, admin := sessionFixture(t)
	defer i.Close()
	t0 := admin.CreatedAt.Add(time.Hour)
	credentials, err := CreateAdminSession(ctx, i, VerifiedAdmin{ID: admin.ID, Username: admin.Username, AuthVersion: admin.AuthVersion}, 300, 3600, t0)
	if err != nil {
		t.Fatal(err)
	}
	if credentials.SessionToken == "" || credentials.CSRFToken == "" || credentials.SessionToken == credentials.CSRFToken || !credentials.ExpiresAt.Equal(t0.Add(300*time.Second)) {
		t.Fatal("invalid issued credentials")
	}
	var idHash, csrfHash []byte
	if err := i.DB().QueryRow("SELECT id_hash,csrf_hash FROM admin_sessions").Scan(&idHash, &csrfHash); err != nil {
		t.Fatal(err)
	}
	if string(idHash) == credentials.SessionToken || string(csrfHash) == credentials.CSRFToken {
		t.Fatal("raw credential persisted")
	}
	used, err := UseAdminSession(ctx, i, credentials.SessionToken, credentials.CSRFToken, true, t0.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if !used.ExpiresAt.Equal(t0.Add(6 * time.Minute)) {
		t.Fatalf("expiry=%v", used.ExpiresAt)
	}
	rotated, err := RotateAdminSessionCSRF(ctx, i, credentials.SessionToken, t0.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if rotated.CSRFToken == "" || rotated.CSRFToken == credentials.CSRFToken {
		t.Fatal("csrf not rotated")
	}
	if _, err := UseAdminSession(ctx, i, credentials.SessionToken, credentials.CSRFToken, true, t0.Add(3*time.Minute)); err != nil {
		t.Fatal("previous csrf rejected at boundary", err)
	}
	if _, err := UseAdminSession(ctx, i, credentials.SessionToken, credentials.CSRFToken, true, t0.Add(3*time.Minute+time.Nanosecond)); !errors.Is(err, ErrAdminSessionInvalid) {
		t.Fatalf("old csrf accepted: %v", err)
	}
	if err := RevokeAdminSession(ctx, i, credentials.SessionToken, rotated.CSRFToken, t0.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := UseAdminSession(ctx, i, credentials.SessionToken, rotated.CSRFToken, true, t0.Add(3*time.Minute)); !errors.Is(err, ErrAdminSessionInvalid) {
		t.Fatalf("revoked session accepted: %v", err)
	}
	var creates, logouts int
	_ = i.DB().QueryRow("SELECT count(*) FROM audit_events WHERE action='admin.session_create'").Scan(&creates)
	_ = i.DB().QueryRow("SELECT count(*) FROM audit_events WHERE action='admin.session_logout'").Scan(&logouts)
	if creates != 1 || logouts != 1 {
		t.Fatalf("creates=%d logouts=%d", creates, logouts)
	}
}

func TestExpiredAdminSessionDoesNotAdvanceOrRevive(t *testing.T) {
	ctx := context.Background()
	i, admin := sessionFixture(t)
	defer i.Close()
	t0 := admin.CreatedAt.Add(time.Hour)
	c, err := CreateAdminSession(ctx, i, VerifiedAdmin{ID: admin.ID, Username: admin.Username, AuthVersion: 1}, 300, 7200, t0)
	if err != nil {
		t.Fatal(err)
	}
	var before string
	_ = i.DB().QueryRow("SELECT last_seen_at FROM admin_sessions").Scan(&before)
	if _, err := UseAdminSession(ctx, i, c.SessionToken, c.CSRFToken, true, t0.Add(301*time.Second)); !errors.Is(err, ErrAdminSessionInvalid) {
		t.Fatalf("expired accepted: %v", err)
	}
	var after string
	_ = i.DB().QueryRow("SELECT last_seen_at FROM admin_sessions").Scan(&after)
	if before != after {
		t.Fatal("rejected session advanced activity")
	}
	// A larger current setting cannot change the idle policy captured in the row.
	if _, err := i.DB().Exec("UPDATE admin_settings SET spec_json=json_set(spec_json,'$.idleSeconds',86400),resource_version=resource_version+1"); err != nil {
		t.Fatal(err)
	}
	if _, err := UseAdminSession(ctx, i, c.SessionToken, c.CSRFToken, true, t0.Add(302*time.Second)); !errors.Is(err, ErrAdminSessionInvalid) {
		t.Fatal("policy change revived session")
	}
}

func TestPasswordResetAndSessionCreationRaceLeavesNoValidSession(t *testing.T) {
	ctx := context.Background()
	i, admin := sessionFixture(t)
	defer i.Close()
	t0 := time.Now().UTC()
	start := make(chan struct{})
	var wg sync.WaitGroup
	var credentials AdminSessionCredentials
	var createErr, resetErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		credentials, createErr = CreateAdminSession(ctx, i, VerifiedAdmin{ID: admin.ID, Username: admin.Username, AuthVersion: 1}, 1800, 43200, t0)
	}()
	go func() {
		defer wg.Done()
		<-start
		_, resetErr = ResetAdminPassword(ctx, i, []byte("replacement password"))
	}()
	close(start)
	wg.Wait()
	if resetErr != nil {
		t.Fatal(resetErr)
	}
	if createErr != nil && !IsRepositoryCode(createErr, CodeVersionConflict) {
		t.Fatal(createErr)
	}
	if createErr == nil {
		if _, err := UseAdminSession(ctx, i, credentials.SessionToken, credentials.CSRFToken, true, t0.Add(time.Second)); !errors.Is(err, ErrAdminSessionInvalid) {
			t.Fatalf("race left valid session: %v", err)
		}
	}
	var active int
	_ = i.DB().QueryRow("SELECT count(*) FROM admin_sessions WHERE revoked_at IS NULL").Scan(&active)
	if active != 0 {
		t.Fatal("active session survived reset")
	}
}

func TestAdminSessionAuditFailureRollsBack(t *testing.T) {
	ctx := context.Background()
	i, admin := sessionFixture(t)
	defer i.Close()
	t0 := time.Now().UTC()
	if _, err := i.DB().Exec(`CREATE TRIGGER reject_session_audit BEFORE INSERT ON audit_events WHEN NEW.action='admin.session_create' BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateAdminSession(ctx, i, VerifiedAdmin{ID: admin.ID, Username: admin.Username, AuthVersion: 1}, 1800, 43200, t0); err == nil {
		t.Fatal("audit failure accepted")
	}
	var count int
	_ = i.DB().QueryRow("SELECT count(*) FROM admin_sessions").Scan(&count)
	if count != 0 {
		t.Fatal("session survived failed audit")
	}
}

func TestAdminSessionRejectsMalformedCredentials(t *testing.T) {
	if _, err := decodeAdminToken(strings.Repeat("A", 1<<20)); !errors.Is(err, ErrAdminSessionInvalid) {
		t.Fatal("oversized token accepted")
	}
	i, _ := sessionFixture(t)
	defer i.Close()
	ctx := context.Background()
	if _, err := UseAdminSession(ctx, i, "not-a-token", "", false, time.Now()); !errors.Is(err, ErrAdminSessionInvalid) {
		t.Fatal(err)
	}
	if err := RevokeAdminSession(ctx, i, "not-a-token", "not-a-token", time.Now()); !errors.Is(err, ErrAdminSessionInvalid) {
		t.Fatal(err)
	}
}

func TestCSRFClockRollbackDoesNotExtendPreviousWindowOrMoveActivity(t *testing.T) {
	ctx := context.Background()
	i, admin := sessionFixture(t)
	defer i.Close()
	t0 := admin.CreatedAt.Add(time.Hour)
	credentials, err := CreateAdminSession(ctx, i, VerifiedAdmin{ID: admin.ID, Username: admin.Username, AuthVersion: 1}, 300, 3600, t0)
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := RotateAdminSessionCSRF(ctx, i, credentials.SessionToken, t0.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := UseAdminSession(ctx, i, credentials.SessionToken, credentials.CSRFToken, true, t0.Add(time.Minute)); !errors.Is(err, ErrAdminSessionInvalid) {
		t.Fatalf("previous CSRF accepted before its rotation time: %v", err)
	}
	if _, err := RotateAdminSessionCSRF(ctx, i, credentials.SessionToken, t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	var lastSeen string
	if err := i.DB().QueryRow("SELECT last_seen_at FROM admin_sessions").Scan(&lastSeen); err != nil {
		t.Fatal(err)
	}
	parsed, err := time.Parse(time.RFC3339Nano, lastSeen)
	if err != nil || !parsed.Equal(t0.Add(2*time.Minute)) {
		t.Fatalf("last_seen=%v err=%v", parsed, err)
	}
	if _, err := UseAdminSession(ctx, i, credentials.SessionToken, rotated.CSRFToken, true, t0.Add(time.Minute)); !errors.Is(err, ErrAdminSessionInvalid) {
		t.Fatal("superseded token survived a backwards-clock rotation")
	}
}
