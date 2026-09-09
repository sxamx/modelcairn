package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

var ErrPlanAlreadyUsed = errors.New("plan_already_used")

// ExecutePlan holds the key lock and one database transaction across snapshot
// verification, nonce consumption and mutation. snapshot must only read through
// tx and return the binding of the actual desired operation and current state.
// apply must only write through tx. Neither callback may call SecretStore or DB
// methods: the installation has one connection and the key lock is held.
// These callbacks are internal trusted code, never user-supplied extensions.
func (s *SecretStore) ExecutePlan(ctx context.Context, token string,
	snapshot func(*sql.Tx) (PlanBinding, error), apply func(*sql.Tx) error) error {
	return s.executePlan(ctx, token, time.Now, snapshot, apply)
}

func (s *SecretStore) executePlan(ctx context.Context, token string, clock func() time.Time,
	snapshot func(*sql.Tx) (PlanBinding, error), apply func(*sql.Tx) error) error {
	if snapshot == nil || apply == nil {
		return ErrInvalidPlan
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unavailable {
		return errKeyMaterialUnavailable
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin plan transaction: %w", err)
	}
	defer tx.Rollback()
	binding, err := snapshot(tx)
	if err != nil {
		return err
	}
	// Read the clock after lock acquisition and snapshot work: time spent waiting
	// must count against the token lifetime.
	now := clock()
	claims, err := s.verifyPlanTokenLocked(token, binding, now)
	if err != nil {
		return err
	}
	nonce, err := base64.RawURLEncoding.Strict().DecodeString(claims.Nonce)
	if err != nil {
		return ErrInvalidPlan
	}
	hash := sha256.Sum256(nonce)
	_, err = tx.ExecContext(ctx, `INSERT INTO consumed_plan_tokens(nonce_hash,expires_at,consumed_at) VALUES(?,?,?)`,
		hash[:], time.Unix(claims.ExpiresAt, 0).UTC().Format(time.RFC3339Nano), now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		if isUniqueConstraint(err) {
			return ErrPlanAlreadyUsed
		}
		return fmt.Errorf("consume plan token: %w", err)
	}
	if err := apply(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		// A commit error has an uncertain outcome. Prevent retries on this open
		// store until reopening checks its durable state.
		s.unavailable = true
		return fmt.Errorf("commit plan transaction: %w", err)
	}
	return nil
}
