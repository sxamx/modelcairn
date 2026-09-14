//go:build linux

package backupmcb1

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sxamx/modelcairn/internal/storage"
)

func TestRestoreActivatesNewGenerationAndPreservesLegacyState(t *testing.T) {
	ctx := context.Background()
	sourceDir := t.TempDir()
	source, err := storage.OpenInstallation(ctx, sourceDir)
	if err != nil {
		t.Fatal(err)
	}
	value := []byte("provider-secret-value")
	if _, err := source.Secrets().Put(ctx, storage.PutSecret{Name: "provider-key", Value: value}, storage.Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	sourceIdentity := source.Secrets().InstallationID()
	if _, err := source.DB().ExecContext(ctx, `INSERT INTO admin_users
		(id,username,password_phc,auth_version,created_at,updated_at)
		VALUES('admin-1','admin','test-phc',1,'2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := source.DB().ExecContext(ctx, `INSERT INTO admin_sessions
		(id_hash,admin_id,auth_version,csrf_hash,csrf_previous_hash,csrf_rotated_at,created_at,last_seen_at,expires_at,revoked_at,idle_seconds)
		VALUES(x'01','admin-1',1,x'02',NULL,'2026-01-01T00:00:00Z','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z','2026-01-02T00:00:00Z',NULL,3600)`); err != nil {
		t.Fatal(err)
	}
	var originalID string
	if err := source.DB().QueryRowContext(ctx, "SELECT id FROM secrets WHERE name='provider-key'").Scan(&originalID); err != nil {
		t.Fatal(err)
	}
	backupPath := filepath.Join(t.TempDir(), "source.mcb.age")
	passphrase := []byte("correct horse battery staple")
	if _, err := Create(ctx, CreateOptions{Installation: source, Destination: backupPath, Passphrase: passphrase}); err != nil {
		t.Fatal(err)
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}

	targetDir := t.TempDir()
	legacy, err := storage.OpenInstallation(ctx, targetDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Secrets().Put(ctx, storage.PutSecret{Name: "legacy-key", Value: []byte("legacy-secret")}, storage.Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	legacyDatabase := filepath.Join(targetDir, "modelcairn.db")
	if _, err := Restore(ctx, backupPath, passphrase, targetDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(targetDir, "current")); err != nil {
		t.Fatalf("current generation pointer missing: %v", err)
	}
	if _, err := os.Stat(legacyDatabase); err != nil {
		t.Fatalf("legacy database was not preserved: %v", err)
	}
	restored, err := storage.OpenInstallation(ctx, targetDir)
	if err != nil {
		t.Fatal(err)
	}
	if restored.StateDirectory() == targetDir {
		t.Fatal("restored installation did not resolve current generation")
	}
	if restored.Secrets().InstallationID() == sourceIdentity {
		t.Fatal("restore reused the source installation identity")
	}
	var sessions int
	if err := restored.DB().QueryRowContext(ctx, "SELECT count(*) FROM admin_sessions").Scan(&sessions); err != nil || sessions != 0 {
		t.Fatalf("restored admin sessions = %d, err=%v; want zero", sessions, err)
	}
	var restoredID string
	if err := restored.DB().QueryRowContext(ctx, "SELECT id FROM secrets WHERE name='provider-key'").Scan(&restoredID); err != nil {
		t.Fatal(err)
	}
	if restoredID != originalID {
		t.Fatalf("secret ID changed: got %s want %s", restoredID, originalID)
	}
	if err := restored.Secrets().Use(ctx, "provider-key", func(plain []byte) error {
		if !bytes.Equal(plain, value) {
			t.Fatalf("restored plaintext differs")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := restored.Secrets().GetMetadata(ctx, "legacy-key"); !storage.IsRepositoryCode(err, storage.CodeNotFound) {
		t.Fatalf("legacy secret unexpectedly active: %v", err)
	}
	if err := restored.Close(); err != nil {
		t.Fatal(err)
	}
	if from, to, err := storage.RollbackGeneration(targetDir); err != nil || from == "" || to != "legacy" {
		t.Fatalf("rollback from=%q to=%q err=%v", from, to, err)
	}
	rolledBack, err := storage.OpenInstallation(ctx, targetDir)
	if err != nil {
		t.Fatal(err)
	}
	defer rolledBack.Close()
	if _, err := rolledBack.Secrets().GetMetadata(ctx, "legacy-key"); err != nil {
		t.Fatalf("legacy state was not recovered: %v", err)
	}
}
