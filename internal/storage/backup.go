package storage

import (
	"context"
	"fmt"
	"os"
	"time"

	sqlite "modernc.org/sqlite"
)

// ExportedSecret is the stable identity and lifecycle metadata needed to
// reconstruct a secret without breaking credential references during restore.
type ExportedSecret struct {
	ID              string
	Name            string
	ResourceVersion int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type sqliteBackuper interface {
	NewBackup(string) (*sqlite.Backup, error)
}

// SnapshotSQLite creates a consistent online SQLite snapshot. The installation
// lock already excludes other ModelCairn processes; SQLite's backup API also
// includes committed WAL content without copying sidecar files.
func (i *Installation) SnapshotSQLite(ctx context.Context, destination string) error {
	if destination == "" {
		return fmt.Errorf("snapshot destination is required")
	}
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf("snapshot destination already exists")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect snapshot destination: %w", err)
	}
	conn, err := i.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("reserve sqlite connection: %w", err)
	}
	defer conn.Close()
	err = conn.Raw(func(raw any) error {
		backuper, ok := raw.(sqliteBackuper)
		if !ok {
			return fmt.Errorf("sqlite driver does not support online backup")
		}
		backup, err := backuper.NewBackup(destination)
		if err != nil {
			return fmt.Errorf("start sqlite backup: %w", err)
		}
		_, stepErr := backup.Step(-1)
		finishErr := backup.Finish()
		if stepErr != nil {
			return fmt.Errorf("copy sqlite snapshot: %w", stepErr)
		}
		if finishErr != nil {
			return fmt.Errorf("finish sqlite snapshot: %w", finishErr)
		}
		return nil
	})
	if err != nil {
		_ = os.Remove(destination)
		return err
	}
	if err := os.Chmod(destination, 0o600); err != nil {
		_ = os.Remove(destination)
		return fmt.Errorf("protect sqlite snapshot: %w", err)
	}
	file, err := os.OpenFile(destination, os.O_RDWR, 0)
	if err != nil {
		_ = os.Remove(destination)
		return fmt.Errorf("open sqlite snapshot for sync: %w", err)
	}
	syncErr := file.Sync()
	closeErr := file.Close()
	if syncErr != nil {
		_ = os.Remove(destination)
		return fmt.Errorf("sync sqlite snapshot: %w", syncErr)
	}
	if closeErr != nil {
		_ = os.Remove(destination)
		return fmt.Errorf("close sqlite snapshot: %w", closeErr)
	}
	return nil
}

// StreamSecrets decrypts one authenticated secret at a time and clears each
// plaintext buffer immediately after callback returns. Callbacks must not retain
// value. The read lock keeps metadata and key state stable for the whole stream.
func (s *SecretStore) StreamSecrets(ctx context.Context, callback func(ExportedSecret, []byte) error) error {
	if callback == nil {
		return &RepositoryError{Code: CodeInvalidResource}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.unavailable {
		return errKeyMaterialUnavailable
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,key_version,algorithm,nonce,ciphertext,
		fingerprint,resource_version,created_at,updated_at FROM secrets ORDER BY id`)
	if err != nil {
		return fmt.Errorf("read secrets for export: %w", err)
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
		var item ExportedSecret
		var keyVersion int64
		var algorithm, fingerprint, created, updated string
		var nonce, ciphertext []byte
		if err := rows.Scan(&item.ID, &item.Name, &keyVersion, &algorithm, &nonce, &ciphertext,
			&fingerprint, &item.ResourceVersion, &created, &updated); err != nil {
			return fmt.Errorf("scan secret for export: %w", err)
		}
		if algorithm != "XCHACHA20-POLY1305" {
			return fmt.Errorf("secret authentication failed")
		}
		item.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
		if err == nil {
			item.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated)
		}
		if err != nil {
			return fmt.Errorf("parse secret lifecycle metadata: %w", err)
		}
		key := loaded[keyVersion]
		if key == nil {
			key, err = s.keyring.load(keyVersion)
			if err != nil {
				return err
			}
			loaded[keyVersion] = key
		}
		plain, err := openSecret(key, nonce, ciphertext, secretContext{s.installationID, item.ID, item.ResourceVersion, keyVersion})
		if err != nil {
			return fmt.Errorf("secret authentication failed")
		}
		actualFingerprint, fingerprintErr := secretFingerprint(key, plain, s.installationID)
		if fingerprintErr != nil || actualFingerprint != fingerprint {
			clear(plain)
			return fmt.Errorf("secret authentication failed")
		}
		callbackErr := callback(item, plain)
		clear(plain)
		if callbackErr != nil {
			return callbackErr
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate secrets for export: %w", err)
	}
	return nil
}
