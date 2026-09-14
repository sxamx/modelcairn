package backupmcb1

import (
	"archive/tar"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"filippo.io/age"

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

func TestCreateRefusesToReplaceExistingDestination(t *testing.T) {
	ctx := context.Background()
	installation, err := storage.OpenInstallation(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer installation.Close()
	destination := filepath.Join(t.TempDir(), "existing.mcb.age")
	sentinel := []byte("keep-this-file")
	if err := os.WriteFile(destination, sentinel, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(ctx, CreateOptions{Installation: installation, Destination: destination, Passphrase: []byte("strong-backup-passphrase")}); err == nil {
		t.Fatal("existing backup destination was accepted")
	}
	contents, err := os.ReadFile(destination)
	if err != nil || !bytes.Equal(contents, sentinel) {
		t.Fatalf("existing destination changed: %q err=%v", contents, err)
	}
}

func TestVerifyRejectsNonScryptRecipientAndUnexpectedPath(t *testing.T) {
	x25519, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	nonScrypt := filepath.Join(t.TempDir(), "non-scrypt.age")
	writeTestAgeTar(t, nonScrypt, x25519.Recipient(), "manifest.json")
	if _, err := Verify(context.Background(), nonScrypt, []byte("strong-backup-passphrase")); err == nil {
		t.Fatal("non-scrypt recipient was accepted")
	}
	recipient, err := age.NewScryptRecipient("strong-backup-passphrase")
	if err != nil {
		t.Fatal(err)
	}
	recipient.SetWorkFactor(defaultScryptLogN)
	unexpected := filepath.Join(t.TempDir(), "unexpected-path.age")
	writeTestAgeTar(t, unexpected, recipient, "../manifest.json")
	if _, err := Verify(context.Background(), unexpected, []byte("strong-backup-passphrase")); err == nil {
		t.Fatal("unexpected tar path was accepted")
	}
}

func writeTestAgeTar(t *testing.T, path string, recipient age.Recipient, name string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := age.Encrypt(file, recipient)
	if err != nil {
		t.Fatal(err)
	}
	writer := tar.NewWriter(encrypted)
	data := []byte("{}")
	if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := encrypted.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
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
