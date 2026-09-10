package storage

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestInstallationBootstrapsAndRepeatedStartupIsNoop(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	first, err := OpenInstallation(ctx, dir)
	if err != nil {
		t.Fatalf("first startup: %v", err)
	}
	var version int
	var checksum string
	if err := first.DB().QueryRowContext(ctx, "SELECT version, checksum FROM schema_migrations").Scan(&version, &checksum); err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if version != 1 || len(checksum) != sha256.Size*2 {
		t.Fatalf("migration = (%d,%q), want version 1 and SHA-256", version, checksum)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first startup: %v", err)
	}

	second, err := OpenInstallation(ctx, dir)
	if err != nil {
		t.Fatalf("second startup: %v", err)
	}
	defer second.Close()
	var count int
	if err := second.DB().QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("migration count = %d, want 3", count)
	}
}

func TestAdminSettingsMigrationRevokesLegacySessions(t *testing.T) {
	ctx := context.Background()
	db, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "upgrade.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	migrations, err := embeddedMigrations()
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) != 3 {
		t.Fatalf("migration count=%d, want 3", len(migrations))
	}
	if err := migrateWithSet(ctx, db, migrations[:2]); err != nil {
		t.Fatal(err)
	}
	now := "2026-01-01T00:00:00Z"
	if _, err := db.ExecContext(ctx, "INSERT INTO admin_users VALUES(?,?,?,1,?,?)", "admin", "admin", "test-phc", now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO admin_sessions(id_hash,admin_id,auth_version,csrf_hash,csrf_previous_hash,csrf_rotated_at,created_at,last_seen_at,expires_at,revoked_at) VALUES(?, 'admin', 1, ?, NULL, ?, ?, ?, ?, NULL)`, make([]byte, 32), make([]byte, 32), now, now, now, "2026-01-02T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if err := migrateWithSet(ctx, db, migrations); err != nil {
		t.Fatal(err)
	}
	var revoked sql.NullString
	var idle sql.NullInt64
	if err := db.QueryRowContext(ctx, "SELECT revoked_at,idle_seconds FROM admin_sessions").Scan(&revoked, &idle); err != nil {
		t.Fatal(err)
	}
	if !revoked.Valid || idle.Valid {
		t.Fatalf("legacy session revoked=%v idle=%v", revoked.Valid, idle.Valid)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO admin_sessions(id_hash,admin_id,auth_version,csrf_hash,csrf_rotated_at,created_at,last_seen_at,expires_at,revoked_at,idle_seconds) VALUES(?, 'admin', 1, ?, ?, ?, ?, ?, NULL, NULL)`, bytes.Repeat([]byte{1}, 32), make([]byte, 32), now, now, now, now); err == nil {
		t.Fatal("unrevoked session without idle policy accepted")
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO admin_settings VALUES(1,1,'{}',?)", now); err != nil {
		t.Fatal(err)
	}
	if err := CheckSchemaCompatibility(ctx, db); err != nil {
		t.Fatal(err)
	}
}

func TestMigrationMatchesDocumentedPhysicalObjects(t *testing.T) {
	ctx := context.Background()
	migrated, err := OpenInstallation(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer migrated.Close()
	contract := openContractDatabase(t)
	got := schemaObjects(t, migrated.DB())
	want := schemaObjects(t, contract)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("migration objects differ from schema-v1.sql\ngot:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func schemaObjects(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query("SELECT type || ':' || name FROM sqlite_master WHERE name NOT LIKE 'sqlite_%' ORDER BY type, name")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var objects []string
	for rows.Next() {
		var object string
		if err := rows.Scan(&object); err != nil {
			t.Fatal(err)
		}
		objects = append(objects, object)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	sort.Strings(objects)
	return objects
}

func TestInstallationLockHasExactlyOneOwnerInBothOrders(t *testing.T) {
	for _, firstOwner := range []string{"service", "cli"} {
		t.Run(firstOwner+"_first", func(t *testing.T) {
			dir := t.TempDir()
			first, err := AcquireLock(dir)
			if err != nil {
				t.Fatalf("%s acquire: %v", firstOwner, err)
			}
			if _, err := AcquireLock(dir); !errors.Is(err, ErrInstallationInUse) {
				t.Fatalf("contender error = %v, want installation_in_use", err)
			}
			if _, err := os.Stat(filepath.Join(dir, "modelcairn.db")); !os.IsNotExist(err) {
				t.Fatalf("contender touched SQLite: %v", err)
			}
			if err := first.Close(); err != nil {
				t.Fatal(err)
			}
			second, err := AcquireLock(dir)
			if err != nil {
				t.Fatalf("lock not released after %s: %v", firstOwner, err)
			}
			_ = second.Close()
		})
	}
}

func TestMigrationChecksumMismatchFailsWithoutSchemaChanges(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	installation, err := OpenInstallation(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := installation.DB().ExecContext(ctx, "UPDATE schema_migrations SET checksum = ? WHERE version = 1", strings.Repeat("0", sha256.Size*2)); err != nil {
		t.Fatal(err)
	}
	if err := installation.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := OpenSQLite(ctx, filepath.Join(dir, "modelcairn.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(ctx, db); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("Migrate error = %v", err)
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM resources").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("resources changed after rejection: %d", count)
	}
}

func TestFutureMigrationFailsWithoutSchemaChanges(t *testing.T) {
	ctx := context.Background()
	db, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "future.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, `CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY, checksum TEXT NOT NULL, applied_at TEXT NOT NULL) STRICT`); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("future"))
	if _, err := db.ExecContext(ctx, "INSERT INTO schema_migrations VALUES(4, ?, '2026-01-01T00:00:00Z')", hex.EncodeToString(sum[:])); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, db); err == nil || !strings.Contains(err.Error(), "unsupported schema version") {
		t.Fatalf("Migrate error = %v", err)
	}
	var exists int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='resources'").Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists != 0 {
		t.Fatal("future-version rejection applied migration")
	}
}

func TestMigrationHistoryMustBeContiguous(t *testing.T) {
	migrations := []migration{{version: 1}, {version: 2}, {version: 3}}
	if err := validateAppliedMigrations(map[int]string{1: "one", 3: "three"}, migrations); err == nil || !strings.Contains(err.Error(), "missing version 2") {
		t.Fatalf("validation error = %v, want missing version 2", err)
	}
}

func TestDriftIsRejectedBeforeAFutureMigrationCanRun(t *testing.T) {
	ctx := context.Background()
	db, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "drifted.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "DROP INDEX idx_audit_time"); err != nil {
		t.Fatal(err)
	}
	migrations, err := embeddedMigrations()
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a future binary containing v4. The pre-migration check must reject
	// the drift while the synthetic v4 marker remains unapplied.
	migrations = append(migrations, migration{version: 4, checksum: strings.Repeat("a", 64), sql: "CREATE TABLE future_marker(id INTEGER) STRICT"})
	if err := migrateWithSet(ctx, db, migrations); err == nil {
		t.Fatal("drifted v1 schema was accepted before v2")
	}
	var exists int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE name='future_marker'").Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists != 0 {
		t.Fatal("future migration changed a drifted schema")
	}
}

func TestLogicallyIncompatibleSchemaFailsClosed(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	installation, err := OpenInstallation(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := installation.DB().ExecContext(ctx, "DROP TRIGGER destinations_provider_match_insert"); err != nil {
		t.Fatal(err)
	}
	if err := installation.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenInstallation(ctx, dir); err == nil || !strings.Contains(err.Error(), "incompatible sqlite schema") {
		t.Fatalf("OpenInstallation error = %v, want incompatible schema", err)
	}
	// A failed compatibility check must still release ownership for diagnosis or repair.
	lock, err := AcquireLock(dir)
	if err != nil {
		t.Fatalf("failed startup retained lock: %v", err)
	}
	_ = lock.Close()
}

func TestInterruptedInitialMigrationLeavesNoCommittedSubset(t *testing.T) {
	ctx := context.Background()
	db, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "interrupted.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// This pre-existing table forces migration 1 to fail after earlier CREATE
	// statements have executed inside its transaction.
	if _, err := db.ExecContext(ctx, "CREATE TABLE resources(id TEXT PRIMARY KEY) STRICT"); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, db); err == nil {
		t.Fatal("conflicting bootstrap unexpectedly succeeded")
	}
	for _, table := range []string{"installation_state", "consumed_plan_tokens"} {
		var exists int
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if exists != 0 {
			t.Fatalf("%s survived rolled-back migration", table)
		}
	}
	var applied int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != 0 {
		t.Fatalf("ledger contains %d applied migrations after rollback", applied)
	}
}

func TestLockFileSymlinkIsRejected(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("symlink privileges are environment-dependent on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "modelcairn.lock")); err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireLock(dir); err == nil {
		t.Fatal("symlink lock file was followed")
	}
}

func TestKernelReleasesLockAfterOwnerProcessDies(t *testing.T) {
	dir := t.TempDir()
	command := exec.Command(os.Args[0], "-test.run=^TestLockHelperProcess$", "--", dir)
	command.Env = append(os.Environ(), "MODELCAIRN_LOCK_HELPER=1")
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	if line, err := bufio.NewReader(stdout).ReadString('\n'); err != nil || strings.TrimSpace(line) != "READY" {
		_ = command.Process.Kill()
		t.Fatalf("helper readiness line=%q err=%v", line, err)
	}
	if _, err := AcquireLock(dir); !errors.Is(err, ErrInstallationInUse) {
		t.Fatalf("cross-process contender error=%v", err)
	}
	if err := command.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = command.Wait()
	lock, err := AcquireLock(dir)
	if err != nil {
		t.Fatalf("kernel did not release dead owner's lock: %v", err)
	}
	_ = lock.Close()
}

func TestLockHelperProcess(t *testing.T) {
	if os.Getenv("MODELCAIRN_LOCK_HELPER") != "1" {
		return
	}
	dir := os.Args[len(os.Args)-1]
	lock, err := AcquireLock(dir)
	if err != nil {
		os.Exit(3)
	}
	defer lock.Close()
	_, _ = os.Stdout.WriteString("READY\n")
	select {}
}
