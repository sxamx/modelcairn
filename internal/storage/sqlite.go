package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

const defaultBusyTimeout = 5 * time.Second

// OpenSQLite opens a single-process SQLite database with the invariants required
// by the Phase 1 contract. Milestone 2 will replace the single-connection spike
// limit with a measured repository concurrency policy.
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

// ApplySchema applies the complete documentation contract to an empty spike
// database. Versioned migrations replace this helper in Milestone 2.
func ApplySchema(ctx context.Context, db *sql.DB, schema string) error {
	if _, err := db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("apply sqlite schema: %w", err)
	}
	return nil
}
