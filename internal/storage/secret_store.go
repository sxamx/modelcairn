package storage

import (
	"context"
	"crypto/hmac"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/sxamx/modelcairn/internal/redact"
)

var secretNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

type SecretMetadata struct {
	Name            string
	Fingerprint     string
	ResourceVersion int64
	KeyVersion      int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type PutSecret struct {
	Name            string
	Value           []byte
	ExpectedVersion int64 // zero creates; positive updates exactly that version
}

// SecretStore owns the active master key in memory while an Installation is open.
// Public operations expose metadata; plaintext access will remain an internal
// routing concern.
type SecretStore struct {
	mu               sync.RWMutex
	db               *sql.DB
	keyring          *keyring
	installationID   string
	activeKeyVersion int64
	activeKey        []byte
	redactor         *redact.Redactor
	registrationMu   sync.Mutex
	registrations    map[string]func()
}

func (s *SecretStore) GetMetadata(ctx context.Context, name string) (SecretMetadata, error) {
	if !secretNamePattern.MatchString(name) {
		return SecretMetadata{}, &RepositoryError{Code: CodeInvalidResource}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return scanSecretMetadata(s.db.QueryRowContext(ctx, `SELECT name,fingerprint,resource_version,key_version,created_at,updated_at
		FROM secrets WHERE name=?`, name))
}

func (s *SecretStore) ListMetadata(ctx context.Context) ([]SecretMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.QueryContext(ctx, `SELECT name,fingerprint,resource_version,key_version,created_at,updated_at
		FROM secrets ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list secret metadata: %w", err)
	}
	defer rows.Close()
	var result []SecretMetadata
	for rows.Next() {
		item, err := scanSecretMetadata(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate secret metadata: %w", err)
	}
	return result, nil
}

func scanSecretMetadata(row rowScanner) (SecretMetadata, error) {
	var item SecretMetadata
	var created, updated string
	if err := row.Scan(&item.Name, &item.Fingerprint, &item.ResourceVersion, &item.KeyVersion, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SecretMetadata{}, &RepositoryError{Code: CodeNotFound}
		}
		return SecretMetadata{}, fmt.Errorf("scan secret metadata: %w", err)
	}
	var err error
	if item.CreatedAt, err = time.Parse(time.RFC3339Nano, created); err != nil {
		return SecretMetadata{}, fmt.Errorf("parse secret created_at: %w", err)
	}
	if item.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated); err != nil {
		return SecretMetadata{}, fmt.Errorf("parse secret updated_at: %w", err)
	}
	return item, nil
}

func (s *SecretStore) Put(ctx context.Context, input PutSecret, actor Actor) (SecretMetadata, error) {
	if err := validateActor(actor); err != nil {
		return SecretMetadata{}, err
	}
	if !secretNamePattern.MatchString(input.Name) || input.ExpectedVersion < 0 {
		return SecretMetadata{}, &RepositoryError{Code: CodeInvalidResource}
	}
	// sealSecret validates byte length and UTF-8 without including the rejected
	// value in any error.
	s.mu.RLock()
	defer s.mu.RUnlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return SecretMetadata{}, fmt.Errorf("begin secret mutation: %w", err)
	}
	metadata, secretID, err := s.putTx(ctx, tx, input)
	if err == nil {
		action := "secret.create"
		if input.ExpectedVersion > 0 {
			action = "secret.update"
		}
		err = insertAudit(ctx, tx, time.Now(), actor, auditRecord{Action: action, Kind: "Secret", ResourceID: secretID, Result: "success", Version: metadata.ResourceVersion})
	}
	if err != nil {
		_ = tx.Rollback()
		return SecretMetadata{}, err
	}
	if err := tx.Commit(); err != nil {
		return SecretMetadata{}, fmt.Errorf("commit secret mutation: %w", err)
	}
	if err := s.replaceRegistration(secretID, input.Value); err != nil {
		return SecretMetadata{}, fmt.Errorf("register secret redaction: %w", err)
	}
	return metadata, nil
}

func (s *SecretStore) Delete(ctx context.Context, name string, expectedVersion int64, actor Actor) error {
	if err := validateActor(actor); err != nil {
		return err
	}
	if !secretNamePattern.MatchString(name) || expectedVersion < 1 {
		return &RepositoryError{Code: CodeInvalidResource}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin secret deletion: %w", err)
	}
	var id string
	var current int64
	if err := tx.QueryRowContext(ctx, "SELECT id,resource_version FROM secrets WHERE name=?", name).Scan(&id, &current); err != nil {
		_ = tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return &RepositoryError{Code: CodeNotFound}
		}
		return fmt.Errorf("read secret for deletion: %w", err)
	}
	if current != expectedVersion {
		_ = tx.Rollback()
		return &RepositoryError{Code: CodeVersionConflict}
	}
	var references int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM credentials WHERE secret_id=?", id).Scan(&references); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("check secret references: %w", err)
	}
	if references != 0 {
		_ = tx.Rollback()
		return &RepositoryError{Code: CodeResourceInUse}
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM secrets WHERE id=? AND resource_version=?", id, expectedVersion); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("delete secret: %w", err)
	}
	if err := insertAudit(ctx, tx, time.Now(), actor, auditRecord{Action: "secret.delete", Kind: "Secret", ResourceID: id, Result: "success", Version: expectedVersion}); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("audit secret deletion: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit secret deletion: %w", err)
	}
	s.removeRegistration(id)
	return nil
}

func (s *SecretStore) putTx(ctx context.Context, tx *sql.Tx, input PutSecret) (SecretMetadata, string, error) {
	now := time.Now().UTC()
	stamp := now.Format(time.RFC3339Nano)
	secretID, err := newUUID()
	resourceVersion := int64(1)
	created := stamp
	if input.ExpectedVersion == 0 {
		var exists int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM secrets WHERE name=?", input.Name).Scan(&exists); err != nil {
			return SecretMetadata{}, "", err
		}
		if exists != 0 {
			return SecretMetadata{}, "", &RepositoryError{Code: CodeAlreadyExists}
		}
	} else {
		var current int64
		if err := tx.QueryRowContext(ctx, "SELECT id,resource_version,created_at FROM secrets WHERE name=?", input.Name).Scan(&secretID, &current, &created); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return SecretMetadata{}, "", &RepositoryError{Code: CodeNotFound}
			}
			return SecretMetadata{}, "", err
		}
		if current != input.ExpectedVersion {
			return SecretMetadata{}, secretID, &RepositoryError{Code: CodeVersionConflict}
		}
		resourceVersion = current + 1
	}
	context := secretContext{s.installationID, secretID, resourceVersion, s.activeKeyVersion}
	nonce, ciphertext, err := sealSecret(s.activeKey, input.Value, context)
	if err != nil {
		return SecretMetadata{}, secretID, err
	}
	fingerprint, err := secretFingerprint(s.activeKey, input.Value, s.installationID)
	if err != nil {
		return SecretMetadata{}, secretID, err
	}
	if input.ExpectedVersion == 0 {
		_, err = tx.ExecContext(ctx, `INSERT INTO secrets
			(id,name,key_version,algorithm,nonce,ciphertext,fingerprint,resource_version,created_at,updated_at)
			VALUES(?,?,?,'XCHACHA20-POLY1305',?,?,?,?,?,?)`, secretID, input.Name, s.activeKeyVersion, nonce, ciphertext, fingerprint, resourceVersion, created, stamp)
	} else {
		result, updateErr := tx.ExecContext(ctx, `UPDATE secrets SET key_version=?,algorithm='XCHACHA20-POLY1305',nonce=?,ciphertext=?,fingerprint=?,resource_version=?,updated_at=?
			WHERE id=? AND resource_version=?`, s.activeKeyVersion, nonce, ciphertext, fingerprint, resourceVersion, stamp, secretID, input.ExpectedVersion)
		err = updateErr
		if err == nil {
			changed, changedErr := result.RowsAffected()
			if changedErr != nil || changed != 1 {
				err = &RepositoryError{Code: CodeVersionConflict}
			}
		}
	}
	if err != nil {
		return SecretMetadata{}, secretID, fmt.Errorf("persist encrypted secret: %w", err)
	}
	createdAt, err := time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return SecretMetadata{}, secretID, err
	}
	return SecretMetadata{input.Name, fingerprint, resourceVersion, s.activeKeyVersion, createdAt, now}, secretID, nil
}

// Use resolves a secret only for the duration of callback, then clears its
// plaintext buffer. The value is never returned or retained by the store.
func (s *SecretStore) Use(ctx context.Context, name string, callback func([]byte) error) error {
	if !secretNamePattern.MatchString(name) || callback == nil {
		return &RepositoryError{Code: CodeInvalidResource}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var id string
	var keyVersion, resourceVersion int64
	var nonce, ciphertext []byte
	err := s.db.QueryRowContext(ctx, `SELECT id,key_version,nonce,ciphertext,resource_version FROM secrets WHERE name=?`, name).
		Scan(&id, &keyVersion, &nonce, &ciphertext, &resourceVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return &RepositoryError{Code: CodeNotFound}
	}
	if err != nil {
		return fmt.Errorf("resolve encrypted secret: %w", err)
	}
	key := s.activeKey
	clearKey := false
	if keyVersion != s.activeKeyVersion {
		key, err = s.keyring.load(keyVersion)
		if err != nil {
			return err
		}
		clearKey = true
	}
	if clearKey {
		defer clear(key)
	}
	plain, err := openSecret(key, nonce, ciphertext, secretContext{s.installationID, id, resourceVersion, keyVersion})
	if err != nil {
		return fmt.Errorf("secret authentication failed")
	}
	defer clear(plain)
	return callback(plain)
}

func openSecretStore(ctx context.Context, db *sql.DB, keys *keyring) (*SecretStore, error) {
	var installationID string
	var activeVersion int64
	var keyCheck []byte
	err := db.QueryRowContext(ctx, `SELECT installation_id, active_key_version, key_check
		FROM installation_state WHERE singleton=1`).Scan(&installationID, &activeVersion, &keyCheck)
	if errors.Is(err, sql.ErrNoRows) {
		return bootstrapSecretStore(ctx, db, keys)
	}
	if err != nil {
		return nil, fmt.Errorf("read installation key state: %w", err)
	}
	activeKey, err := keys.load(activeVersion)
	if err != nil {
		return nil, err
	}
	store := newSecretStore(db, keys, installationID, activeVersion, activeKey)
	if len(keyCheck) == 0 {
		if err := store.finishLegacyBootstrap(ctx); err != nil {
			store.close()
			return nil, err
		}
	} else {
		expected, err := masterKeyCheck(activeKey, installationID)
		if err != nil || !hmac.Equal(expected, keyCheck) {
			store.close()
			return nil, fmt.Errorf("%w: active master key version %d", errKeyMaterialUnavailable, activeVersion)
		}
	}
	if err := store.verifyAll(ctx); err != nil {
		store.close()
		return nil, err
	}
	return store, nil
}

func bootstrapSecretStore(ctx context.Context, db *sql.DB, keys *keyring) (*SecretStore, error) {
	var secrets int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM secrets").Scan(&secrets); err != nil {
		return nil, fmt.Errorf("check secret-store bootstrap: %w", err)
	}
	if secrets != 0 {
		return nil, fmt.Errorf("secret store has data without installation identity")
	}
	versions, err := keys.versions()
	if err != nil {
		return nil, err
	}
	version := int64(1)
	if len(versions) > 0 {
		version = versions[len(versions)-1] + 1
		if version < 1 {
			return nil, errKeyMaterialUnavailable
		}
	}
	key, err := keys.create(version)
	if err != nil {
		return nil, err
	}
	installationID, err := newUUID()
	if err != nil {
		clear(key)
		return nil, fmt.Errorf("generate installation identity: %w", err)
	}
	check, err := masterKeyCheck(key, installationID)
	if err != nil {
		clear(key)
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.ExecContext(ctx, `INSERT INTO installation_state
		(singleton, installation_id, active_key_version, key_check, config_revision, created_at, updated_at)
		VALUES(1,?,?,?,?,?,?)`, installationID, version, check, 1, now, now); err != nil {
		clear(key)
		return nil, fmt.Errorf("commit installation key state: %w", err)
	}
	return newSecretStore(db, keys, installationID, version, key), nil
}

func newSecretStore(db *sql.DB, keys *keyring, installationID string, version int64, key []byte) *SecretStore {
	return &SecretStore{
		db: db, keyring: keys, installationID: installationID,
		activeKeyVersion: version, activeKey: key,
		redactor: redact.New(), registrations: make(map[string]func()),
	}
}

func (s *SecretStore) finishLegacyBootstrap(ctx context.Context) error {
	var secrets int
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM secrets").Scan(&secrets); err != nil {
		return fmt.Errorf("check legacy key state: %w", err)
	}
	if secrets != 0 {
		return fmt.Errorf("%w: unverified legacy key state", errKeyMaterialUnavailable)
	}
	check, err := masterKeyCheck(s.activeKey, s.installationID)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE installation_state SET key_check=?, updated_at=?
		WHERE singleton=1 AND key_check IS NULL`, check, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("commit installation key check: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return fmt.Errorf("commit installation key check: state changed concurrently")
	}
	return nil
}

func (s *SecretStore) verifyAll(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id, key_version, algorithm, nonce, ciphertext,
		fingerprint, resource_version FROM secrets ORDER BY id`)
	if err != nil {
		return fmt.Errorf("read encrypted secrets: %w", err)
	}
	defer rows.Close()
	loaded := map[int64][]byte{s.activeKeyVersion: s.activeKey}
	defer func() {
		for version, key := range loaded {
			if version != s.activeKeyVersion {
				clear(key)
			}
		}
	}()
	for rows.Next() {
		var id, algorithm, storedFingerprint string
		var keyVersion, resourceVersion int64
		var nonce, ciphertext []byte
		if err := rows.Scan(&id, &keyVersion, &algorithm, &nonce, &ciphertext, &storedFingerprint, &resourceVersion); err != nil {
			return fmt.Errorf("scan encrypted secret metadata: %w", err)
		}
		if algorithm != "XCHACHA20-POLY1305" {
			return fmt.Errorf("secret authentication failed")
		}
		key := loaded[keyVersion]
		if key == nil {
			key, err = s.keyring.load(keyVersion)
			if err != nil {
				return err
			}
			loaded[keyVersion] = key
		}
		plain, err := openSecret(key, nonce, ciphertext, secretContext{s.installationID, id, resourceVersion, keyVersion})
		if err != nil {
			return fmt.Errorf("secret authentication failed")
		}
		fingerprint, fpErr := secretFingerprint(key, plain, s.installationID)
		if fpErr != nil || fingerprint != storedFingerprint {
			clear(plain)
			return fmt.Errorf("secret authentication failed")
		}
		if err := s.replaceRegistration(id, plain); err != nil {
			clear(plain)
			return fmt.Errorf("register secret redaction: %w", err)
		}
		clear(plain)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate encrypted secrets: %w", err)
	}
	return nil
}

func (s *SecretStore) close() {
	if s != nil {
		s.registrationMu.Lock()
		for id, remove := range s.registrations {
			remove()
			delete(s.registrations, id)
		}
		s.registrationMu.Unlock()
		clear(s.activeKey)
		s.activeKey = nil
	}
}

func (s *SecretStore) replaceRegistration(id string, value []byte) error {
	remove, err := s.redactor.Register(value)
	if err != nil {
		return err
	}
	s.registrationMu.Lock()
	previous := s.registrations[id]
	s.registrations[id] = remove
	s.registrationMu.Unlock()
	if previous != nil {
		previous()
	}
	return nil
}

func (s *SecretStore) removeRegistration(id string) {
	s.registrationMu.Lock()
	remove := s.registrations[id]
	delete(s.registrations, id)
	s.registrationMu.Unlock()
	if remove != nil {
		remove()
	}
}

// Redactor returns the installation-scoped output sanitizer.
func (s *SecretStore) Redactor() *redact.Redactor { return s.redactor }

// InstallationID is non-secret stable metadata for this installation.
func (s *SecretStore) InstallationID() string { return s.installationID }

// ActiveKeyVersion is non-secret operational metadata.
func (s *SecretStore) ActiveKeyVersion() int64 { return s.activeKeyVersion }
