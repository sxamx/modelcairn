package storage

import (
	"context"
	"crypto/hmac"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

var temporaryKeyName = regexp.MustCompile(`^\.new-[0-9a-f]{32}\.tmp$`)

// RotateMasterKey durably publishes a new key before atomically changing all
// ciphertext, fingerprints and the installation key check. An uncertain commit
// or failed post-commit verification disables the store until it is reopened.
func (s *SecretStore) RotateMasterKey(ctx context.Context, actor Actor) (int64, error) {
	if err := validateActor(actor); err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unavailable {
		return 0, errKeyMaterialUnavailable
	}
	if err := s.verifyKeyState(ctx); err != nil {
		s.unavailable = true
		return 0, err
	}
	if err := s.verifyAll(ctx); err != nil {
		s.unavailable = true
		return 0, err
	}
	versions, err := s.keyring.versions()
	if err != nil {
		return 0, err
	}
	next := s.activeKeyVersion
	for _, version := range versions {
		if version > next {
			next = version
		}
	}
	if next == math.MaxInt64 {
		return 0, errKeyMaterialUnavailable
	}
	next++
	newKey, err := s.keyring.create(next)
	if err != nil {
		return 0, err
	}
	defer clear(newKey)
	// Read the published file back before any database mutation.
	published, err := s.keyring.load(next)
	if err != nil {
		return 0, err
	}
	valid := hmac.Equal(newKey, published)
	clear(published)
	if !valid {
		return 0, errKeyMaterialUnavailable
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	if err = s.rotateTx(ctx, tx, next, newKey, actor); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			s.unavailable = true
		}
		return 0, err
	}
	s.keyring.checkpoint("before-commit")
	if err := tx.Commit(); err != nil {
		s.unavailable = true
		return 0, fmt.Errorf("rotation commit outcome uncertain: %w", err)
	}
	s.unavailable = true
	s.keyring.checkpoint("database-committed")
	verified, err := s.keyring.load(next)
	if err != nil {
		return next, err
	}
	clear(s.activeKey)
	s.activeKey, s.activeKeyVersion = verified, next
	if err := s.verifyKeyState(ctx); err != nil {
		return next, err
	}
	if err := s.verifyAll(ctx); err != nil {
		return next, err
	}
	s.keyring.checkpoint("rows-verified")
	s.unavailable = false
	if err := s.collectKeys(ctx); err != nil {
		return next, err
	}
	return next, nil
}

func (s *SecretStore) rotateTx(ctx context.Context, tx *sql.Tx, next int64, newKey []byte, actor Actor) error {
	// Keyset iteration closes each query before its update. Only one secret is
	// held at a time, and the primary-key ordering is not changed by rotation.
	last := ""
	for {
		var id, fingerprint string
		var version, resourceVersion int64
		var nonce, ciphertext []byte
		err := tx.QueryRowContext(ctx, `SELECT id,key_version,resource_version,nonce,ciphertext,fingerprint
			FROM secrets WHERE id>? ORDER BY id LIMIT 1`, last).Scan(&id, &version, &resourceVersion, &nonce, &ciphertext, &fingerprint)
		if errors.Is(err, sql.ErrNoRows) {
			break
		}
		if err != nil {
			return err
		}
		key, err := s.keyring.load(version)
		if err != nil {
			return err
		}
		plain, err := openSecret(key, nonce, ciphertext, secretContext{s.installationID, id, resourceVersion, version})
		if err != nil {
			clear(key)
			return err
		}
		expected, err := secretFingerprint(key, plain, s.installationID)
		clear(key)
		if err != nil || expected != fingerprint {
			clear(plain)
			return errSecretAuthentication
		}
		newNonce, encrypted, err := sealSecret(newKey, plain, secretContext{s.installationID, id, resourceVersion, next})
		if err != nil {
			clear(plain)
			return err
		}
		newFingerprint, err := secretFingerprint(newKey, plain, s.installationID)
		clear(plain)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE secrets SET key_version=?,nonce=?,ciphertext=?,fingerprint=? WHERE id=?`, next, newNonce, encrypted, newFingerprint, id); err != nil {
			return err
		}
		s.keyring.checkpoint("row-reencrypted")
		last = id
	}
	check, err := masterKeyCheck(newKey, s.installationID)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE installation_state SET active_key_version=?,key_check=?,updated_at=? WHERE singleton=1 AND active_key_version=?`, next, check, time.Now().UTC().Format(time.RFC3339Nano), s.activeKeyVersion)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return errKeyMaterialUnavailable
	}
	return insertAudit(ctx, tx, time.Now(), actor, auditRecord{Action: "secret.rotate", Kind: "Secret", Result: "success", Version: next})
}

// CollectUnusedKeys can be retried after interrupted rotation. It verifies all
// committed secrets first and never deletes the active version, even when empty.
func (s *SecretStore) CollectUnusedKeys(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unavailable {
		return errKeyMaterialUnavailable
	}
	if err := s.verifyKeyState(ctx); err != nil {
		s.unavailable = true
		return err
	}
	if err := s.verifyAll(ctx); err != nil {
		s.unavailable = true
		return err
	}
	return s.collectKeys(ctx)
}

func (s *SecretStore) verifyKeyState(ctx context.Context) error {
	var id string
	var version int64
	var check []byte
	if err := s.db.QueryRowContext(ctx, `SELECT installation_id,active_key_version,key_check FROM installation_state WHERE singleton=1`).Scan(&id, &version, &check); err != nil {
		return errKeyMaterialUnavailable
	}
	if id != s.installationID || version != s.activeKeyVersion {
		return errKeyMaterialUnavailable
	}
	key, err := s.keyring.load(version)
	if err != nil {
		return err
	}
	defer clear(key)
	expected, err := masterKeyCheck(key, id)
	if err != nil || !hmac.Equal(check, expected) || !hmac.Equal(key, s.activeKey) {
		return errKeyMaterialUnavailable
	}
	return nil
}

func (s *SecretStore) collectKeys(ctx context.Context) error {
	versions, err := s.keyring.versions()
	if err != nil {
		return err
	}
	for _, version := range versions {
		var references int
		if err := s.db.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM secrets WHERE key_version=?)+(SELECT count(*) FROM installation_state WHERE active_key_version=?)`, version, version).Scan(&references); err != nil {
			return err
		}
		if references != 0 {
			continue
		}
		path := filepath.Join(s.keyring.dir, keyFilename(version))
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return errKeyMaterialUnavailable
		}
		if err := os.Remove(path); err != nil {
			return err
		}
		s.keyring.checkpoint("old-key-removed")
		if err := syncDirectory(s.keyring.dir); err != nil {
			return err
		}
		s.keyring.checkpoint("gc-directory-synced")
	}
	entries, err := os.ReadDir(s.keyring.dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !temporaryKeyName.MatchString(entry.Name()) {
			continue
		}
		path := filepath.Join(s.keyring.dir, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return errKeyMaterialUnavailable
		}
		if err := os.Remove(path); err != nil {
			return err
		}
		s.keyring.checkpoint("temporary-removed")
	}
	// Also retries durability if a previous cleanup failed after unlinking.
	if err := syncDirectory(s.keyring.dir); err != nil {
		return err
	}
	s.keyring.checkpoint("cleanup-synced")
	return nil
}
