package storage

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const defaultBusyTimeout = 5 * time.Second

// OpenSQLite opens SQLite with ModelCairn's required connection invariants. Code
// that owns installation state must use OpenInstallation so the process lock is
// acquired before this function is reached.
func OpenSQLite(ctx context.Context, path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	for _, statement := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		fmt.Sprintf("PRAGMA busy_timeout = %d", defaultBusyTimeout.Milliseconds()),
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			db.Close()
			return nil, fmt.Errorf("configure sqlite: %w", err)
		}
	}
	return db, nil
}

// Installation is the exclusive owner of one local data directory. Closing it
// closes SQLite before releasing the operating-system lock.
type Installation struct {
	db      *sql.DB
	lock    *Lock
	secrets *SecretStore
}

// OpenInstallation exclusively owns dataDir, migrates its database, and checks
// integrity before returning it to the caller.
func OpenInstallation(ctx context.Context, dataDir string) (*Installation, error) {
	lock, err := AcquireLock(dataDir)
	if err != nil {
		return nil, err
	}
	db, err := OpenSQLite(ctx, filepath.Join(dataDir, "modelcairn.db"))
	if err != nil {
		_ = lock.Close()
		return nil, err
	}
	if err := Migrate(ctx, db); err != nil {
		_ = db.Close()
		_ = lock.Close()
		return nil, err
	}
	if err := CheckIntegrity(ctx, db); err != nil {
		_ = db.Close()
		_ = lock.Close()
		return nil, err
	}
	if err := CheckSchemaCompatibility(ctx, db); err != nil {
		_ = db.Close()
		_ = lock.Close()
		return nil, err
	}
	keys, err := openKeyring(dataDir)
	if err != nil {
		_ = db.Close()
		_ = lock.Close()
		return nil, err
	}
	secrets, err := openSecretStore(ctx, db, keys)
	if err != nil {
		_ = db.Close()
		_ = lock.Close()
		return nil, err
	}
	return &Installation{db: db, lock: lock, secrets: secrets}, nil
}

// DB returns the installation database. The caller must not close it directly.
func (i *Installation) DB() *sql.DB { return i.db }

// Secrets returns the verified secret-store owner for internal application use.
func (i *Installation) Secrets() *SecretStore { return i.secrets }

// Close releases durable resources in the required order.
func (i *Installation) Close() error {
	i.secrets.close()
	dbErr := i.db.Close()
	lockErr := i.lock.Close()
	if dbErr != nil {
		return fmt.Errorf("close sqlite: %w", dbErr)
	}
	return lockErr
}
