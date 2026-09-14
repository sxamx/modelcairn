package storage

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSnapshotSQLiteIncludesCommittedWALState(t *testing.T) {
	ctx := context.Background()
	installation, err := OpenInstallation(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer installation.Close()
	if _, err := installation.DB().ExecContext(ctx, `INSERT INTO resources
		(id,kind,name,resource_version,spec_json,created_at,updated_at)
		VALUES('r1','Provider','one',1,'{}','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "snapshot.sqlite")
	if err := installation.SnapshotSQLite(ctx, destination); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("snapshot mode = %o, want 600", info.Mode().Perm())
	}
	db, err := OpenSQLite(ctx, destination)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM resources WHERE id='r1'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("snapshot row count = %d, err=%v", count, err)
	}
}

func TestStreamSecretsPreservesIdentityAndClearsCallbackBuffer(t *testing.T) {
	ctx := context.Background()
	installation, err := OpenInstallation(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer installation.Close()
	value := []byte("secret-value")
	metadata, err := installation.Secrets().Put(ctx, PutSecret{Name: "provider-key", Value: value}, Actor{Type: "cli"})
	if err != nil {
		t.Fatal(err)
	}
	var retained []byte
	var exported ExportedSecret
	if err := installation.Secrets().StreamSecrets(ctx, func(item ExportedSecret, plain []byte) error {
		exported = item
		retained = plain
		if !bytes.Equal(plain, value) {
			t.Fatalf("exported value differs")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if exported.ID == "" || exported.Name != metadata.Name || exported.ResourceVersion != metadata.ResourceVersion {
		t.Fatalf("unexpected exported metadata: %+v", exported)
	}
	if !bytes.Equal(retained, make([]byte, len(retained))) {
		t.Fatal("callback plaintext buffer was not cleared")
	}
}
