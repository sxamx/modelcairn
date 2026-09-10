package storage

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/sxamx/modelcairn/internal/adminauth"
	"github.com/sxamx/modelcairn/internal/adminsettings"
)

func bootstrapFixture(t *testing.T) (*Installation, adminsettings.Resolved) {
	t.Helper()
	i, err := OpenInstallation(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	spec := adminsettings.Defaults()
	spec.PublicOrigin = "http://127.0.0.1:8080"
	return i, spec
}

func TestBootstrapAdminAtomicAndExclusive(t *testing.T) {
	ctx := context.Background()
	i, spec := bootstrapFixture(t)
	defer i.Close()
	admin, settings, err := BootstrapAdmin(ctx, i, "owner", []byte("a secure password"), spec)
	if err != nil {
		t.Fatal(err)
	}
	if admin.AuthVersion != 1 || settings.ResourceVersion != 1 {
		t.Fatal("wrong initial versions")
	}
	var phc string
	if err := i.DB().QueryRow("SELECT password_phc FROM admin_users WHERE id=?", admin.ID).Scan(&phc); err != nil {
		t.Fatal(err)
	}
	if ok, err := adminauth.Verify([]byte("a secure password"), phc); err != nil || !ok {
		t.Fatal("password not persisted correctly")
	}
	if _, _, err := BootstrapAdmin(ctx, i, "other", []byte("another password"), spec); !IsRepositoryCode(err, CodeAlreadyExists) {
		t.Fatalf("replacement allowed: %v", err)
	}
	var audits int
	if err := i.DB().QueryRow("SELECT count(*) FROM audit_events WHERE action='admin.bootstrap' AND actor_id=? AND resource_id=?", admin.ID, admin.ID).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("audit=%d err=%v", audits, err)
	}
}

func TestConcurrentBootstrapHasOneWinner(t *testing.T) {
	ctx := context.Background()
	i, spec := bootstrapFixture(t)
	defer i.Close()
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, name := range []string{"first", "second"} {
		wg.Add(1)
		go func(username string) {
			defer wg.Done()
			<-start
			_, _, err := BootstrapAdmin(ctx, i, username, []byte("concurrent password"), spec)
			errs <- err
		}(name)
	}
	close(start)
	wg.Wait()
	close(errs)
	var success, exists int
	for err := range errs {
		if err == nil {
			success++
		} else if IsRepositoryCode(err, CodeAlreadyExists) {
			exists++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || exists != 1 {
		t.Fatalf("success=%d exists=%d", success, exists)
	}
	var admins, settings, audits int
	_ = i.DB().QueryRow("SELECT count(*) FROM admin_users").Scan(&admins)
	_ = i.DB().QueryRow("SELECT count(*) FROM admin_settings").Scan(&settings)
	_ = i.DB().QueryRow("SELECT count(*) FROM audit_events WHERE action='admin.bootstrap'").Scan(&audits)
	if admins != 1 || settings != 1 || audits != 1 {
		t.Fatalf("admins=%d settings=%d audits=%d", admins, settings, audits)
	}
}

func TestBootstrapDetectsIncompleteInstallation(t *testing.T) {
	ctx := context.Background()
	i, spec := bootstrapFixture(t)
	defer i.Close()
	if _, err := i.DB().Exec("INSERT INTO admin_users(id,username,password_phc,auth_version,created_at,updated_at) VALUES('x','x','x',1,'x','x')"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := BootstrapAdmin(ctx, i, "owner", []byte("a secure password"), spec); !IsRepositoryCode(err, CodeInstallationIncomplete) {
		t.Fatalf("incomplete accepted: %v", err)
	}
	var settings int
	_ = i.DB().QueryRow("SELECT count(*) FROM admin_settings").Scan(&settings)
	if settings != 0 {
		t.Fatal("incomplete bootstrap wrote settings")
	}
}

func TestResetAdminPasswordRevokesSessionsAtomically(t *testing.T) {
	ctx := context.Background()
	i, spec := bootstrapFixture(t)
	defer i.Close()
	admin, _, err := BootstrapAdmin(ctx, i, "owner", []byte("original password"), spec)
	if err != nil {
		t.Fatal(err)
	}
	stamp := "2026-09-10T00:00:00Z"
	if _, err := i.DB().Exec(`INSERT INTO admin_sessions(id_hash,admin_id,auth_version,csrf_hash,csrf_rotated_at,created_at,last_seen_at,expires_at,revoked_at,idle_seconds) VALUES(?,?,?,?,?,?,?,?,NULL,?)`, make([]byte, 32), admin.ID, 1, make([]byte, 32), stamp, stamp, stamp, "2026-09-11T00:00:00Z", 1800); err != nil {
		t.Fatal(err)
	}
	reset, err := ResetAdminPassword(ctx, i, []byte("replacement password"))
	if err != nil {
		t.Fatal(err)
	}
	if reset.AuthVersion != 2 {
		t.Fatal("auth version not incremented")
	}
	var phc string
	var active, audits int
	_ = i.DB().QueryRow("SELECT password_phc FROM admin_users WHERE id=?", admin.ID).Scan(&phc)
	_ = i.DB().QueryRow("SELECT count(*) FROM admin_sessions WHERE revoked_at IS NULL").Scan(&active)
	_ = i.DB().QueryRow("SELECT count(*) FROM audit_events WHERE action='admin.password_reset'").Scan(&audits)
	if ok, err := adminauth.Verify([]byte("replacement password"), phc); err != nil || !ok || active != 0 || audits != 1 {
		t.Fatalf("ok=%v err=%v active=%d audits=%d", ok, err, active, audits)
	}
	if ok, _ := adminauth.Verify([]byte("original password"), phc); ok {
		t.Fatal("old password remains valid")
	}
}

func TestBootstrapInputValidation(t *testing.T) {
	i, spec := bootstrapFixture(t)
	defer i.Close()
	if _, _, err := BootstrapAdmin(context.Background(), i, "", []byte("a secure password"), spec); !IsRepositoryCode(err, CodeInvalidUsername) {
		t.Fatal(err)
	}
	if _, _, err := BootstrapAdmin(context.Background(), i, "owner", []byte("short"), spec); !errors.Is(err, adminauth.ErrInvalidPassword) {
		t.Fatal(err)
	}
}

func TestBootstrapAndResetRollBackOnAuditFailure(t *testing.T) {
	ctx := context.Background()
	i, spec := bootstrapFixture(t)
	defer i.Close()
	if _, err := i.DB().Exec(`CREATE TRIGGER reject_identity_audit BEFORE INSERT ON audit_events WHEN NEW.action IN ('admin.bootstrap','admin.password_reset') BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := BootstrapAdmin(ctx, i, "owner", []byte("a secure password"), spec); err == nil {
		t.Fatal("bootstrap audit failure accepted")
	}
	var admins, settings int
	_ = i.DB().QueryRow("SELECT count(*) FROM admin_users").Scan(&admins)
	_ = i.DB().QueryRow("SELECT count(*) FROM admin_settings").Scan(&settings)
	if admins != 0 || settings != 0 {
		t.Fatal("failed bootstrap partially committed")
	}
	if _, err := i.DB().Exec("DROP TRIGGER reject_identity_audit"); err != nil {
		t.Fatal(err)
	}
	admin, _, err := BootstrapAdmin(ctx, i, "owner", []byte("a secure password"), spec)
	if err != nil {
		t.Fatal(err)
	}
	stamp := "2026-09-10T00:00:00Z"
	if _, err := i.DB().Exec(`INSERT INTO admin_sessions(id_hash,admin_id,auth_version,csrf_hash,csrf_rotated_at,created_at,last_seen_at,expires_at,revoked_at,idle_seconds) VALUES(?,?,?,?,?,?,?,?,NULL,?)`, make([]byte, 32), admin.ID, 1, make([]byte, 32), stamp, stamp, stamp, "2026-09-11T00:00:00Z", 1800); err != nil {
		t.Fatal(err)
	}
	if _, err := i.DB().Exec(`CREATE TRIGGER reject_reset_audit BEFORE INSERT ON audit_events WHEN NEW.action='admin.password_reset' BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := ResetAdminPassword(ctx, i, []byte("replacement password")); err == nil {
		t.Fatal("reset audit failure accepted")
	}
	var phc string
	var version, active int
	_ = i.DB().QueryRow("SELECT password_phc,auth_version FROM admin_users WHERE id=?", admin.ID).Scan(&phc, &version)
	_ = i.DB().QueryRow("SELECT count(*) FROM admin_sessions WHERE revoked_at IS NULL").Scan(&active)
	ok, err := adminauth.Verify([]byte("a secure password"), phc)
	if err != nil || !ok || version != 1 || active != 1 {
		t.Fatalf("rollback ok=%v err=%v version=%d active=%d", ok, err, version, active)
	}
}
