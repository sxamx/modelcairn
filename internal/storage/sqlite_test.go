package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func contractSchema(t *testing.T) string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join("..", "..", "docs", "contratos", "storage", "schema-v1.sql"))
	if err != nil {
		t.Fatalf("read contract schema: %v", err)
	}
	return string(contents)
}

func applyContractSchema(ctx context.Context, db *sql.DB, schema string) error {
	if _, err := db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("apply sqlite schema contract: %w", err)
	}
	return nil
}

func openContractDatabase(t *testing.T) *sql.DB {
	t.Helper()
	db, err := OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "contract.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := applyContractSchema(context.Background(), db, contractSchema(t)); err != nil {
		t.Fatalf("apply contract schema: %v", err)
	}
	return db
}

func TestSQLitePragmasAndSchema(t *testing.T) {
	db := openContractDatabase(t)
	var foreignKeys int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil || foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, err=%v; want 1", foreignKeys, err)
	}
	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil || journalMode != "wal" {
		t.Fatalf("journal_mode = %q, err=%v; want wal", journalMode, err)
	}
	var busyTimeout int
	if err := db.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil || busyTimeout != int(defaultBusyTimeout.Milliseconds()) {
		t.Fatalf("busy_timeout = %d, err=%v; want %d", busyTimeout, err, defaultBusyTimeout.Milliseconds())
	}
	if _, err := db.Exec("INSERT INTO provider_accounts VALUES('missing','also-missing')"); err == nil {
		t.Fatal("foreign-key violation was accepted")
	}
}

func TestWALAllowsReaderDuringUncommittedWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.db")
	writer, err := OpenSQLite(context.Background(), path)
	if err != nil {
		t.Fatalf("open writer: %v", err)
	}
	defer writer.Close()
	if err := applyContractSchema(context.Background(), writer, contractSchema(t)); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
	reader, err := OpenSQLite(context.Background(), path)
	if err != nil {
		t.Fatalf("open reader: %v", err)
	}
	defer reader.Close()

	tx, err := writer.Begin()
	if err != nil {
		t.Fatalf("begin writer: %v", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec("INSERT INTO resources(id,kind,name,spec_json,created_at,updated_at) VALUES('pending','Provider','pending','{}','2026-01-01','2026-01-01')"); err != nil {
		t.Fatalf("uncommitted insert: %v", err)
	}
	var count int
	if err := reader.QueryRow("SELECT count(*) FROM resources").Scan(&count); err != nil {
		t.Fatalf("concurrent WAL read: %v", err)
	}
	if count != 0 {
		t.Fatalf("reader observed %d uncommitted rows, want 0", count)
	}
}

func TestProviderAffinityMutationsAreRejected(t *testing.T) {
	mutations := []string{
		"UPDATE destinations SET credential_id='cr2' WHERE resource_id='d1'",
		"UPDATE destinations SET model_id='m2' WHERE resource_id='d1'",
		"UPDATE provider_accounts SET provider_id='p2' WHERE resource_id='a1'",
		"UPDATE provider_connections SET provider_id='p2' WHERE resource_id='c1'",
		"UPDATE credentials SET provider_account_id='a2' WHERE resource_id='cr1'",
		"UPDATE models SET connection_id='c2' WHERE resource_id='m1'",
	}
	for index, mutation := range mutations {
		t.Run(fmt.Sprintf("mutation_%d", index+1), func(t *testing.T) {
			db := openContractDatabase(t)
			seedAffinityFixture(t, db)
			if _, err := db.Exec(mutation); err == nil {
				t.Fatalf("unsafe mutation accepted: %s", mutation)
			}
		})
	}
}

func seedAffinityFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	statements := []string{
		`INSERT INTO resources(id,kind,name,spec_json,created_at,updated_at) VALUES
('p1','Provider','p1','{}','2026-01-01','2026-01-01'),('p2','Provider','p2','{}','2026-01-01','2026-01-01'),
('a1','ProviderAccount','a1','{}','2026-01-01','2026-01-01'),('a2','ProviderAccount','a2','{}','2026-01-01','2026-01-01'),
('c1','ProviderConnection','c1','{}','2026-01-01','2026-01-01'),('c2','ProviderConnection','c2','{}','2026-01-01','2026-01-01'),
('e1','Egress','e1','{}','2026-01-01','2026-01-01'),('cr1','Credential','cr1','{}','2026-01-01','2026-01-01'),
('cr2','Credential','cr2','{}','2026-01-01','2026-01-01'),('m1','Model','m1','{}','2026-01-01','2026-01-01'),
('m2','Model','m2','{}','2026-01-01','2026-01-01'),('d1','Destination','d1','{}','2026-01-01','2026-01-01')`,
		"INSERT INTO provider_accounts VALUES('a1','p1'),('a2','p2')",
		"INSERT INTO provider_connections VALUES('c1','p1','https://one.invalid',0,1),('c2','p2','https://two.invalid',0,1)",
		"INSERT INTO egresses VALUES('e1','direct',1)",
		"INSERT INTO secrets VALUES('s1','s1',1,'XCHACHA20-POLY1305',zeroblob(24),x'01','fp1',1,'2026-01-01','2026-01-01'),('s2','s2',1,'XCHACHA20-POLY1305',zeroblob(24),x'02','fp2',1,'2026-01-01','2026-01-01')",
		"INSERT INTO credentials VALUES('cr1','a1','e1','s1','active',NULL),('cr2','a2','e1','s2','active',NULL)",
		"INSERT INTO models VALUES('m1','c1','m1','[]',1),('m2','c2','m2','[]',1)",
		"INSERT INTO destinations VALUES('d1','m1','cr1',100,1)",
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("seed affinity fixture: %v", err)
		}
	}
}
