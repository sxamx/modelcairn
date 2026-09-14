package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ExecuteAdminMutation serializes an administrative graph mutation with secret
// rotation, authenticates it, and commits all work atomically. The callback is
// trusted application code and must only use the supplied transaction.
func (s *SecretStore) ExecuteAdminMutation(ctx context.Context, sessionToken, csrfToken string, work func(*sql.Tx, Actor) error) error {
	if work == nil {
		return errors.New("admin_mutation_callback_missing")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unavailable {
		return errKeyMaterialUnavailable
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin admin mutation: %w", err)
	}
	actor, err := AuthorizeAdminMutationTx(ctx, tx, sessionToken, csrfToken)
	if err == nil {
		err = work(tx, actor)
	}
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("admin mutation failed (%v) and rollback failed: %w", err, rollbackErr)
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		// The outcome is uncertain. Refuse further writes until the installation
		// is reopened instead of risking state/redactor divergence.
		s.unavailable = true
		return fmt.Errorf("commit admin mutation: %w", err)
	}
	return nil
}

// GetResourceTx reads a resource through the caller's transaction.
func GetResourceTx(ctx context.Context, tx *sql.Tx, kind ResourceKind, name string) (Resource, error) {
	return scanResource(tx.QueryRowContext(ctx, `SELECT id,kind,name,display_name,description,resource_version,spec_json,created_at,updated_at
		FROM resources WHERE kind=? AND name=?`, kind, name))
}

// AuditResourceNoopTx records an accepted conditional update that changed no
// persisted resource fields. It intentionally does not bump config_revision.
func AuditResourceNoopTx(ctx context.Context, tx *sql.Tx, item Resource, actor Actor) error {
	if err := validateActor(actor); err != nil {
		return err
	}
	return insertAudit(ctx, tx, time.Now(), actor, auditRecord{
		Action: "resource.update_noop", Kind: item.Kind, ResourceID: item.ID,
		Result: "success", Version: item.ResourceVersion,
	})
}

// PublishStrategyTx turns the current draft into an immutable version and moves
// every route using the strategy atomically. The resource version protects the
// operator from publishing a draft that changed in another session.
func PublishStrategyTx(ctx context.Context, tx *sql.Tx, name string, expectedVersion int64, actor Actor) (Resource, error) {
	if err := validateActor(actor); err != nil {
		return Resource{}, err
	}
	item, err := GetResourceTx(ctx, tx, KindStrategy, name)
	if err != nil {
		return Resource{}, err
	}
	if expectedVersion < 1 || item.ResourceVersion != expectedVersion {
		return Resource{}, &RepositoryError{Code: CodeVersionConflict}
	}
	if _, err := publishStrategyTx(ctx, tx, item.ID, item.Spec, time.Now().UTC()); err != nil {
		return Resource{}, err
	}
	if err := insertAudit(ctx, tx, time.Now(), actor, auditRecord{Action: "strategy.publish", Kind: KindStrategy, ResourceID: item.ID, Result: "success", Version: item.ResourceVersion}); err != nil {
		return Resource{}, err
	}
	if err := bumpConfigRevisionTx(ctx, tx, time.Now()); err != nil {
		return Resource{}, err
	}
	return item, nil
}
