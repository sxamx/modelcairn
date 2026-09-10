package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/sxamx/modelcairn/internal/adminsettings"
)

func TestAdminSettingsUpdateAtomicAuditAndRetry(t *testing.T) {
	ctx := context.Background()
	i, err := OpenInstallation(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer i.Close()
	initial := adminsettings.Defaults()
	initial.PublicOrigin = "http://127.0.0.1:8080"
	tx, err := i.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := InsertAdminSettingsTx(ctx, tx, initial, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	want := initial
	want.IdleSeconds = 600
	binding := testPlanBinding()
	snapshot := func(*sql.Tx) (PlanBinding, error) { return binding, nil }
	token, err := i.Secrets().CreateSettingsPlanToken(ctx, time.Now(), snapshot)
	if err != nil {
		t.Fatal(err)
	}
	// A failure after writing settings must also undo token consumption.
	if _, err := i.DB().Exec(`CREATE TRIGGER reject_settings_audit BEFORE INSERT ON audit_events WHEN NEW.action='admin_settings.apply' BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	apply := func(tx *sql.Tx) error {
		_, err := UpdateAdminSettingsTx(ctx, tx, 1, want, Actor{Type: "cli"}, time.Now())
		return err
	}
	if err := i.Secrets().ExecuteSettingsPlan(ctx, token, snapshot, apply); err == nil {
		t.Fatal("audit failure accepted")
	}
	current, err := ReadAdminSettings(ctx, i.DB())
	if err != nil {
		t.Fatal(err)
	}
	if current.ResourceVersion != 1 || current.Spec.IdleSeconds != 1800 {
		t.Fatal("audit failure changed settings")
	}
	var count int
	if err := i.DB().QueryRow("SELECT count(*) FROM consumed_plan_tokens").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("audit failure consumed nonce")
	}
	if _, err := i.DB().Exec("DROP TRIGGER reject_settings_audit"); err != nil {
		t.Fatal(err)
	}
	if err := i.Secrets().ExecuteSettingsPlan(ctx, token, snapshot, apply); err != nil {
		t.Fatal(err)
	}
	current, err = ReadAdminSettings(ctx, i.DB())
	if err != nil {
		t.Fatal(err)
	}
	if current.ResourceVersion != 2 || current.Spec.IdleSeconds != 600 {
		t.Fatal("update missing")
	}
	var details string
	if err := i.DB().QueryRow("SELECT details_json FROM audit_events WHERE action='admin_settings.apply'").Scan(&details); err != nil {
		t.Fatal(err)
	}
	var audit struct {
		PreviousVersion int64
		Version         int64
		ChangedFields   []string
	}
	if err := json.Unmarshal([]byte(details), &audit); err != nil {
		t.Fatal(err)
	}
	if audit.PreviousVersion != 1 || audit.Version != 2 || len(audit.ChangedFields) != 1 || audit.ChangedFields[0] != "idleSeconds" {
		t.Fatalf("audit=%s", details)
	}
	if strings.Contains(details, initial.PublicOrigin) || strings.Contains(details, "600") {
		t.Fatal("settings values leaked into audit")
	}
}

func TestAdminSettingsUpdateNoopAndStaleVersion(t *testing.T) {
	ctx := context.Background()
	i, err := OpenInstallation(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer i.Close()
	spec := adminsettings.Defaults()
	spec.PublicOrigin = "http://127.0.0.1:8080"
	stamp := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	tx, err := i.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := InsertAdminSettingsTx(ctx, tx, spec, stamp); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	tx, err = i.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := UpdateAdminSettingsTx(ctx, tx, 1, spec, Actor{Type: "cli"}, stamp.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if result.ResourceVersion != 1 || !result.UpdatedAt.Equal(stamp) {
		t.Fatal("no-op changed revision or timestamp")
	}
	tx, err = i.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = UpdateAdminSettingsTx(ctx, tx, 2, spec, Actor{Type: "cli"}, time.Now())
	_ = tx.Rollback()
	if !IsRepositoryCode(err, CodeVersionConflict) {
		t.Fatalf("stale version: %v", err)
	}
	var count int
	if err := i.DB().QueryRow("SELECT count(*) FROM audit_events").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("stale update wrote success audit")
	}
}
