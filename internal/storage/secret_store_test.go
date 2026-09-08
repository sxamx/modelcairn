package storage

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestSecretStoreExposesMetadataAndScopedPlaintextOnly(t *testing.T) {
	ctx := context.Background()
	installation, err := OpenInstallation(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer installation.Close()
	store := installation.Secrets()
	value := []byte("canary-super-secret")
	created, err := store.Put(ctx, PutSecret{Name: "openrouter-primary", Value: value}, Actor{Type: "cli"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "openrouter-primary" || created.ResourceVersion != 1 || created.KeyVersion != 1 || created.Fingerprint == "" {
		t.Fatalf("unexpected metadata: %+v", created)
	}
	got, err := store.GetMetadata(ctx, created.Name)
	if err != nil || got != created {
		t.Fatalf("metadata round trip = (%+v,%v)", got, err)
	}
	listed, err := store.ListMetadata(ctx)
	if err != nil || len(listed) != 1 || listed[0] != created {
		t.Fatalf("metadata list = (%+v,%v)", listed, err)
	}
	var callbackCopy []byte
	var retainedView []byte
	if err := store.Use(ctx, created.Name, func(plain []byte) error {
		callbackCopy = bytes.Clone(plain)
		retainedView = plain
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(callbackCopy, value) {
		t.Fatal("scoped plaintext differed")
	}
	if !bytes.Equal(retainedView, make([]byte, len(retainedView))) {
		t.Fatal("scoped plaintext was not cleared after callback")
	}
	var databaseText string
	if err := installation.DB().QueryRowContext(ctx, `SELECT quote(ciphertext)||fingerprint FROM secrets WHERE name=?`, created.Name).Scan(&databaseText); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(databaseText, string(value)) {
		t.Fatal("database exposed plaintext")
	}
	var auditDetails string
	if err := installation.DB().QueryRowContext(ctx, `SELECT details_json FROM audit_events WHERE action='secret.create'`).Scan(&auditDetails); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(auditDetails, string(value)) || strings.Contains(auditDetails, created.Name) {
		t.Fatal("secret audit included value or name")
	}
	if strings.Contains(store.Redactor().String("prefix "+string(value)+" suffix"), string(value)) {
		t.Fatal("created secret was not registered with output redactor")
	}
}

func TestSecretStoreUpdateIsVersionedAndAuthenticated(t *testing.T) {
	ctx := context.Background()
	installation, err := OpenInstallation(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer installation.Close()
	store := installation.Secrets()
	first, err := store.Put(ctx, PutSecret{Name: "provider-key", Value: []byte("first-secret")}, Actor{Type: "cli"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Put(ctx, PutSecret{Name: first.Name, Value: []byte("second-secret"), ExpectedVersion: first.ResourceVersion}, Actor{Type: "admin", ID: "operator"})
	if err != nil {
		t.Fatal(err)
	}
	if second.ResourceVersion != 2 || second.Fingerprint == first.Fingerprint {
		t.Fatal("secret update did not change version and fingerprint")
	}
	if got := store.Redactor().String("first-secret second-secret"); got != "first-secret [REDACTED]" {
		t.Fatalf("updated redactions = %q", got)
	}
	if _, err := store.Put(ctx, PutSecret{Name: first.Name, Value: []byte("stale-secret"), ExpectedVersion: 1}, Actor{Type: "cli"}); !IsRepositoryCode(err, CodeVersionConflict) {
		t.Fatalf("stale update error = %v", err)
	}
	if err := store.Use(ctx, first.Name, func(plain []byte) error {
		if string(plain) != "second-secret" {
			t.Fatal("stale update changed plaintext")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestSecretStoreErrorsNeverEchoRejectedValue(t *testing.T) {
	installation, err := OpenInstallation(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer installation.Close()
	canary := "leak-me"
	_, err = installation.Secrets().Put(context.Background(), PutSecret{Name: "valid-name", Value: []byte(canary)}, Actor{Type: "invalid"})
	if err == nil || strings.Contains(err.Error(), canary) {
		t.Fatalf("unsafe error: %v", err)
	}
}

func TestStartupAuthenticatesEveryStoredSecret(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	installation, err := OpenInstallation(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := installation.Secrets().Put(ctx, PutSecret{Name: "tamper-target", Value: []byte("canary-secret")}, Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	if _, err := installation.DB().ExecContext(ctx, `UPDATE secrets SET ciphertext=x'010203' WHERE name='tamper-target'`); err != nil {
		t.Fatal(err)
	}
	if err := installation.Close(); err != nil {
		t.Fatal(err)
	}
	if reopened, err := OpenInstallation(ctx, dir); reopened != nil || err == nil || strings.Contains(err.Error(), "tamper-target") || strings.Contains(err.Error(), "canary-secret") {
		t.Fatalf("unsafe tamper startup result = (%v,%v)", reopened, err)
	}
}

func TestSecretDeletionRejectsCredentialReferences(t *testing.T) {
	ctx := context.Background()
	installation, err := OpenInstallation(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer installation.Close()
	store := installation.Secrets()
	metadata, err := store.Put(ctx, PutSecret{Name: "referenced-key", Value: []byte("canary-secret")}, Actor{Type: "cli"})
	if err != nil {
		t.Fatal(err)
	}
	var secretID string
	if err := installation.DB().QueryRowContext(ctx, "SELECT id FROM secrets WHERE name=?", metadata.Name).Scan(&secretID); err != nil {
		t.Fatal(err)
	}
	now := "2026-01-01T00:00:00Z"
	for _, item := range []struct{ id, kind string }{{"provider", "Provider"}, {"account", "ProviderAccount"}, {"egress", "Egress"}, {"credential", "Credential"}} {
		if _, err := installation.DB().ExecContext(ctx, `INSERT INTO resources(id,kind,name,spec_json,created_at,updated_at) VALUES(?,?,?,'{}',?,?)`, item.id, item.kind, item.id, now, now); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := installation.DB().ExecContext(ctx, "INSERT INTO provider_accounts VALUES('account','provider')"); err != nil {
		t.Fatal(err)
	}
	if _, err := installation.DB().ExecContext(ctx, "INSERT INTO egresses VALUES('egress','direct',1)"); err != nil {
		t.Fatal(err)
	}
	if _, err := installation.DB().ExecContext(ctx, "INSERT INTO credentials VALUES('credential','account','egress',?,'active',NULL)", secretID); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, metadata.Name, metadata.ResourceVersion, Actor{Type: "cli"}); !IsRepositoryCode(err, CodeResourceInUse) {
		t.Fatalf("referenced deletion error = %v", err)
	}
	if _, err := installation.DB().ExecContext(ctx, "DELETE FROM resources WHERE id='credential'"); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, metadata.Name, metadata.ResourceVersion, Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	if got := store.Redactor().String("canary-secret"); got != "canary-secret" {
		t.Fatalf("deleted secret remains registered: %q", got)
	}
	if _, err := store.GetMetadata(ctx, metadata.Name); !IsRepositoryCode(err, CodeNotFound) {
		t.Fatalf("metadata after deletion error = %v", err)
	}
	var audits int
	if err := installation.DB().QueryRowContext(ctx, "SELECT count(*) FROM audit_events WHERE action='secret.delete' AND result='success'").Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("successful deletion audits = %d, error = %v", audits, err)
	}
}

func TestStartupRegistersPersistedSecretsForRedaction(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	first, err := OpenInstallation(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Secrets().Put(ctx, PutSecret{Name: "persistent-key", Value: []byte("persistent-canary")}, Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := OpenInstallation(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if got := second.Secrets().Redactor().String("persistent-canary"); got != "[REDACTED]" {
		t.Fatalf("startup redaction = %q", got)
	}
}
