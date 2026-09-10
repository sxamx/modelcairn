package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/sxamx/modelcairn/internal/adminsettings"
)

func TestAdminSettingsCreateAndRead(t *testing.T) {
	ctx := context.Background()
	db, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadAdminSettings(ctx, db); !IsRepositoryCode(err, CodeNotFound) {
		t.Fatalf("missing error=%v", err)
	}
	spec := adminsettings.Defaults()
	spec.PublicOrigin = "http://127.0.0.1:8080"
	now := time.Date(2026, 9, 10, 1, 2, 3, 4, time.FixedZone("offset", -3*60*60))
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	created, err := InsertAdminSettingsTx(ctx, tx, spec, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if created.ResourceVersion != 1 || !created.UpdatedAt.Equal(now.UTC()) || created.Spec.TrustedProxyCIDRs == nil {
		t.Fatalf("created=%+v", created)
	}
	read, err := ReadAdminSettings(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if read.ResourceVersion != 1 || !read.UpdatedAt.Equal(now.UTC()) || read.Spec.PublicOrigin != spec.PublicOrigin {
		t.Fatalf("read=%+v", read)
	}
	read.Spec.TrustedProxyCIDRs = append(read.Spec.TrustedProxyCIDRs, "127.0.0.1/32")
	again, err := ReadAdminSettings(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Spec.TrustedProxyCIDRs) != 0 {
		t.Fatal("read result aliased persistence")
	}
}

func TestAdminSettingsCreationRequiresValidSpecAndTransaction(t *testing.T) {
	ctx := context.Background()
	db, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	invalid := adminsettings.Defaults()
	tx, _ := db.BeginTx(ctx, nil)
	if _, err := InsertAdminSettingsTx(ctx, tx, invalid, time.Now()); err == nil {
		t.Fatal("invalid settings accepted")
	}
	_ = tx.Rollback()
	valid := adminsettings.Defaults()
	valid.PublicOrigin = "http://127.0.0.1:8080"
	tx, _ = db.BeginTx(ctx, nil)
	if _, err := InsertAdminSettingsTx(ctx, tx, valid, time.Now()); err != nil {
		t.Fatal(err)
	}
	_ = tx.Rollback()
	if _, err := ReadAdminSettings(ctx, db); !IsRepositoryCode(err, CodeNotFound) {
		t.Fatalf("rollback error=%v", err)
	}
	tx, _ = db.BeginTx(ctx, nil)
	if _, err := InsertAdminSettingsTx(ctx, tx, valid, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	tx, _ = db.BeginTx(ctx, nil)
	_, err = InsertAdminSettingsTx(ctx, tx, valid, time.Now())
	_ = tx.Rollback()
	if !IsRepositoryCode(err, CodeAlreadyExists) {
		t.Fatalf("duplicate error=%v", err)
	}
}

func TestAdminSettingsReadFailsClosedOnCorruption(t *testing.T) {
	ctx := context.Background()
	db, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO admin_settings VALUES(1,1,'{}','not-a-time')"); err != nil {
		t.Fatal(err)
	}
	_, err = ReadAdminSettings(ctx, db)
	if err == nil {
		t.Fatal("corrupt settings accepted")
	}
	var repo *RepositoryError
	if errors.As(err, &repo) {
		t.Fatalf("corruption reported as repository state: %v", err)
	}
}
