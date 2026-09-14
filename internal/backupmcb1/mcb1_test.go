package backupmcb1

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sxamx/modelcairn/internal/storage"
)

func TestCreateAndVerifyMCB1(t *testing.T) {
	ctx := context.Background()
	installation, err := storage.OpenInstallation(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer installation.Close()
	secret := []byte("provider-secret-value")
	if _, err := installation.Secrets().Put(ctx, storage.PutSecret{Name: "provider-key", Value: secret}, storage.Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "backup.mcb.age")
	passphrase := []byte("correct horse battery staple")
	created, err := Create(ctx, CreateOptions{Installation: installation, Destination: destination, Passphrase: passphrase,
		ApplicationVersion: "test", Now: time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	if created.Manifest.Profile != Profile || created.Secrets != 1 {
		t.Fatalf("unexpected creation result: %+v", created)
	}
	verified, err := Verify(ctx, destination, passphrase)
	if err != nil {
		t.Fatal(err)
	}
	if verified.Manifest.ApplicationVersion != "test" || verified.Secrets != 1 {
		t.Fatalf("unexpected verification result: %+v", verified)
	}
	contents, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(contents, secret) {
		t.Fatal("encrypted backup contains plaintext secret")
	}
}

func TestVerifyRejectsWrongPasswordAndTruncation(t *testing.T) {
	ctx := context.Background()
	installation, err := storage.OpenInstallation(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer installation.Close()
	destination := filepath.Join(t.TempDir(), "backup.mcb.age")
	passphrase := []byte("correct horse battery staple")
	if _, err := Create(ctx, CreateOptions{Installation: installation, Destination: destination, Passphrase: passphrase}); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(ctx, destination, []byte("different-password")); err == nil {
		t.Fatal("wrong password was accepted")
	}
	contents, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	truncated := filepath.Join(t.TempDir(), "truncated.mcb.age")
	if err := os.WriteFile(truncated, contents[:len(contents)-16], 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(ctx, truncated, passphrase); err == nil {
		t.Fatal("truncated backup was accepted")
	}
}
