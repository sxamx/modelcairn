package storage

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallationBootstrapsDurableVerifiedKeyring(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	first, err := OpenInstallation(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	id := first.Secrets().InstallationID()
	if id == "" || first.Secrets().ActiveKeyVersion() != 1 {
		t.Fatal("installation secret-store metadata was not initialized")
	}
	var check []byte
	if err := first.DB().QueryRowContext(ctx, "SELECT key_check FROM installation_state WHERE singleton=1").Scan(&check); err != nil || len(check) != 32 {
		t.Fatal("master-key check was not committed")
	}
	keyPath := filepath.Join(dir, keyringName, "v1.key")
	info, err := os.Stat(keyPath)
	if err != nil || info.Size() != masterKeySize {
		t.Fatal("master key was not published")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("master key mode = %o, want 600", info.Mode().Perm())
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := OpenInstallation(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if second.Secrets().InstallationID() != id || second.Secrets().ActiveKeyVersion() != 1 {
		t.Fatal("repeated startup changed secret-store identity")
	}
}

func TestSecretStoreFailsClosedForMissingWrongAndMalformedKeys(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(string) error
	}{
		{"missing", func(path string) error { return os.Remove(path) }},
		{"wrong", func(path string) error { return os.WriteFile(path, bytes.Repeat([]byte{7}, 32), 0o600) }},
		{"short", func(path string) error { return os.WriteFile(path, bytes.Repeat([]byte{7}, 31), 0o600) }},
		{"long", func(path string) error { return os.WriteFile(path, bytes.Repeat([]byte{7}, 33), 0o600) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			installation, err := OpenInstallation(context.Background(), dir)
			if err != nil {
				t.Fatal(err)
			}
			if err := installation.Close(); err != nil {
				t.Fatal(err)
			}
			if err := test.mutate(filepath.Join(dir, keyringName, "v1.key")); err != nil {
				t.Fatal(err)
			}
			if reopened, err := OpenInstallation(context.Background(), dir); reopened != nil || !errors.Is(err, errKeyMaterialUnavailable) {
				t.Fatalf("startup error = %v, want key_material_unavailable", err)
			}
			lock, err := AcquireLock(dir)
			if err != nil {
				t.Fatalf("failed startup retained installation lock: %v", err)
			}
			_ = lock.Close()
		})
	}
}

func TestBootstrapSkipsOrphanKeyVersion(t *testing.T) {
	dir := t.TempDir()
	keys, err := openKeyring(dir)
	if err != nil {
		t.Fatal(err)
	}
	orphan, err := keys.create(1)
	if err != nil {
		t.Fatal(err)
	}
	clear(orphan)
	installation, err := OpenInstallation(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer installation.Close()
	if installation.Secrets().ActiveKeyVersion() != 2 {
		t.Fatal("bootstrap reused an uncommitted key version")
	}
}

func TestKeyPublicationNeverReplacesVersion(t *testing.T) {
	dir := t.TempDir()
	keys, err := openKeyring(dir)
	if err != nil {
		t.Fatal(err)
	}
	first, err := keys.create(1)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(first)
	if second, err := keys.create(1); err == nil || second != nil {
		clear(second)
		t.Fatal("existing master-key version was replaced")
	}
	loaded, err := keys.load(1)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(loaded)
	if !bytes.Equal(first, loaded) {
		t.Fatal("failed publication changed existing master key")
	}
}

func TestKeyringRejectsUnexpectedVersionFilename(t *testing.T) {
	keys, err := openKeyring(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(keys.dir, "v01.key"), bytes.Repeat([]byte{1}, 32), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := keys.versions(); err == nil || !strings.Contains(err.Error(), "invalid keyring entry") {
		t.Fatalf("versions error = %v", err)
	}
}
