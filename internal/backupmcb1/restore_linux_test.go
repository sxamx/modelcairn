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
	defer restored.Close()
	if restored.StateDirectory() == targetDir {
		t.Fatal("restored installation did not resolve current generation")
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
}
