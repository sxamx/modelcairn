package storage

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSecretMutationDerivesActorFromCurrentSession(t *testing.T) {
	ctx := context.Background()
	i, admin := sessionFixture(t)
	defer i.Close()
	old, err := CreateAdminSession(ctx, i, VerifiedAdmin{ID: admin.ID, Username: admin.Username, AuthVersion: admin.AuthVersion}, 1800, 3600, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	reset, err := ResetAdminPassword(ctx, i, []byte("another secure password"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := i.Secrets().PutSession(ctx, PutSecret{Name: "provider-key", Value: []byte("secret value")}, old.SessionToken, old.CSRFToken); !errors.Is(err, ErrAdminSessionInvalid) {
		t.Fatalf("revoked session=%v", err)
	}
	if _, err := i.Secrets().GetMetadata(ctx, "provider-key"); !IsRepositoryCode(err, CodeNotFound) {
		t.Fatal("rejected mutation persisted")
	}
	fresh, err := CreateAdminSession(ctx, i, VerifiedAdmin{ID: reset.ID, Username: reset.Username, AuthVersion: reset.AuthVersion}, 1800, 3600, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := i.Secrets().PutSession(ctx, PutSecret{Name: "provider-key", Value: []byte("secret value")}, fresh.SessionToken, fresh.CSRFToken)
	if err != nil {
		t.Fatal(err)
	}
	var actorType, actorID string
	if err := i.DB().QueryRow("SELECT actor_type,actor_id FROM audit_events WHERE action='secret.create'").Scan(&actorType, &actorID); err != nil {
		t.Fatal(err)
	}
	if actorType != "admin" || actorID != admin.ID {
		t.Fatal("audit actor was caller-controlled")
	}
	if err := i.Secrets().DeleteSession(ctx, metadata.Name, metadata.ResourceVersion, fresh.SessionToken, fresh.CSRFToken); err != nil {
		t.Fatal(err)
	}
}
