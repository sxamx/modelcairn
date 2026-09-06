package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

const migrationLedgerSQL = `CREATE TABLE IF NOT EXISTS schema_migrations (
	version INTEGER PRIMARY KEY, checksum TEXT NOT NULL, applied_at TEXT NOT NULL
) STRICT`

type migration struct {
	version  int
	checksum string
	sql      string
}

func embeddedMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}
	result := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		prefix, _, ok := strings.Cut(entry.Name(), "_")
		if !ok {
			return nil, fmt.Errorf("invalid migration filename %q", entry.Name())
		}
		version, err := strconv.Atoi(prefix)
		if err != nil || version < 1 {
			return nil, fmt.Errorf("invalid migration version in %q", entry.Name())
		}
		body, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", entry.Name(), err)
		}
		sum := sha256.Sum256(body)
		result = append(result, migration{version: version, checksum: hex.EncodeToString(sum[:]), sql: string(body)})
	}
	sort.Slice(result, func(a, b int) bool { return result[a].version < result[b].version })
	for index, item := range result {
		if item.version != index+1 {
			return nil, fmt.Errorf("migration sequence is not monotonic at version %d", item.version)
		}
	}
	return result, nil
}

// Migrate verifies immutable migration history and applies every pending
// migration in its own transaction.
func Migrate(ctx context.Context, db *sql.DB) error {
	migrations, err := embeddedMigrations()
	if err != nil {
		return err
	}
	return migrateWithSet(ctx, db, migrations)
}

func migrateWithSet(ctx context.Context, db *sql.DB, migrations []migration) error {
	if _, err := db.ExecContext(ctx, migrationLedgerSQL); err != nil {
		return fmt.Errorf("create migration ledger: %w", err)
	}

	rows, err := db.QueryContext(ctx, "SELECT version, checksum FROM schema_migrations ORDER BY version")
	if err != nil {
		return fmt.Errorf("read migration ledger: %w", err)
	}
	applied := map[int]string{}
	for rows.Next() {
		var version int
		var checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan migration ledger: %w", err)
		}
		applied[version] = checksum
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close migration ledger: %w", err)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate migration ledger: %w", err)
	}
	if err := validateAppliedMigrations(applied, migrations); err != nil {
		return err
	}

	for version, checksum := range applied {
		if version < 1 || version > len(migrations) {
			return fmt.Errorf("unsupported schema version %d", version)
		}
		if checksum != migrations[version-1].checksum {
			return fmt.Errorf("migration checksum mismatch at version %d", version)
		}
	}
	// Verify the schema at its currently applied version before executing newer
	// migrations. This prevents a future v2 from mutating an already-drifted v1.
	if err := checkSchemaAgainst(ctx, db, migrations[:len(applied)]); err != nil {
		return err
	}
	for _, item := range migrations {
		if _, ok := applied[item.version]; ok {
			continue
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", item.version, err)
		}
		if _, err = tx.ExecContext(ctx, item.sql); err == nil {
			_, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations(version, checksum, applied_at) VALUES(?,?,?)", item.version, item.checksum, time.Now().UTC().Format(time.RFC3339Nano))
		}
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %d: %w", item.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", item.version, err)
		}
	}
	return nil
}

func validateAppliedMigrations(applied map[int]string, migrations []migration) error {
	for version := range applied {
		if version < 1 || version > len(migrations) {
			return fmt.Errorf("unsupported schema version %d", version)
		}
	}
	for version := 1; version <= len(applied); version++ {
		if _, ok := applied[version]; !ok {
			return fmt.Errorf("non-contiguous migration history: missing version %d", version)
		}
	}
	return nil
}

// CheckIntegrity fails closed unless SQLite returns exactly one successful row.
func CheckIntegrity(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, "PRAGMA quick_check")
	if err != nil {
		return fmt.Errorf("run sqlite integrity check: %w", err)
	}
	defer rows.Close()
	var results []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return err
		}
		results = append(results, value)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(results) != 1 || results[0] != "ok" {
		return fmt.Errorf("sqlite integrity check failed: %s", strings.Join(results, "; "))
	}
	return nil
}

// CheckSchemaCompatibility compares every declared table, index, and trigger
// against a transient reference built from the immutable embedded migrations.
// SQLite integrity checks alone do not detect a logically altered schema.
func CheckSchemaCompatibility(ctx context.Context, db *sql.DB) error {
	migrations, err := embeddedMigrations()
	if err != nil {
		return err
	}
	return checkSchemaAgainst(ctx, db, migrations)
}

func checkSchemaAgainst(ctx context.Context, db *sql.DB, migrations []migration) error {
	actual, err := schemaFingerprint(ctx, db)
	if err != nil {
		return fmt.Errorf("fingerprint installed schema: %w", err)
	}
	reference, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return fmt.Errorf("open reference schema: %w", err)
	}
	reference.SetMaxOpenConns(1)
	defer reference.Close()
	if _, err := reference.ExecContext(ctx, migrationLedgerSQL); err != nil {
		return fmt.Errorf("build reference migration ledger: %w", err)
	}
	for _, item := range migrations {
		if _, err := reference.ExecContext(ctx, item.sql); err != nil {
			return fmt.Errorf("build reference schema at version %d: %w", item.version, err)
		}
	}
	expected, err := schemaFingerprint(ctx, reference)
	if err != nil {
		return fmt.Errorf("fingerprint reference schema: %w", err)
	}
	if actual != expected {
		return fmt.Errorf("incompatible sqlite schema: fingerprint mismatch")
	}
	return nil
}

func schemaFingerprint(ctx context.Context, db *sql.DB) (string, error) {
	rows, err := db.QueryContext(ctx, `SELECT type, name, tbl_name, coalesce(sql, '')
		FROM sqlite_schema WHERE name NOT LIKE 'sqlite_%' ORDER BY type, name`)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	hash := sha256.New()
	for rows.Next() {
		var objectType, name, table, statement string
		if err := rows.Scan(&objectType, &name, &table, &statement); err != nil {
			return "", err
		}
		for _, value := range []string{objectType, name, table, statement} {
			_, _ = fmt.Fprintf(hash, "%d:", len(value))
			_, _ = hash.Write([]byte(value))
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
