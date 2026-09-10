package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/sxamx/modelcairn/internal/adminsettings"
)

type AdminSettingsRecord struct {
	ResourceVersion int64
	Spec            adminsettings.Resolved
	UpdatedAt       time.Time
}

// ReadAdminSettings validates the persisted canonical settings snapshot. Missing
// settings and corrupt/noncanonical settings fail closed.
func ReadAdminSettings(ctx context.Context, db *sql.DB) (AdminSettingsRecord, error) {
	return scanAdminSettings(db.QueryRowContext(ctx, "SELECT resource_version,spec_json,updated_at FROM admin_settings WHERE singleton=1"))
}

// InsertAdminSettingsTx creates revision one. The bootstrap service must call this
// inside the same transaction that creates the administrator and success audit.
func InsertAdminSettingsTx(ctx context.Context, tx *sql.Tx, spec adminsettings.Resolved, now time.Time) (AdminSettingsRecord, error) {
	encoded, err := adminsettings.CanonicalJSON(spec)
	if err != nil {
		return AdminSettingsRecord{}, fmt.Errorf("encode admin settings: %w", err)
	}
	validated, err := adminsettings.DecodeCanonicalJSON(encoded)
	if err != nil {
		return AdminSettingsRecord{}, fmt.Errorf("validate admin settings: %w", err)
	}
	stamp := now.UTC().Format(time.RFC3339Nano)
	if _, err = tx.ExecContext(ctx, "INSERT INTO admin_settings(singleton,resource_version,spec_json,updated_at) VALUES(1,1,?,?)", string(encoded), stamp); err != nil {
		if isUniqueConstraint(err) {
			return AdminSettingsRecord{}, &RepositoryError{Code: CodeAlreadyExists}
		}
		return AdminSettingsRecord{}, fmt.Errorf("insert admin settings: %w", err)
	}
	return AdminSettingsRecord{ResourceVersion: 1, Spec: validated, UpdatedAt: now.UTC()}, nil
}

func scanAdminSettings(row rowScanner) (AdminSettingsRecord, error) {
	var record AdminSettingsRecord
	var encoded, updated string
	if err := row.Scan(&record.ResourceVersion, &encoded, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return record, &RepositoryError{Code: CodeNotFound}
		}
		return record, fmt.Errorf("scan admin settings: %w", err)
	}
	if record.ResourceVersion < 1 {
		return record, errors.New("invalid_admin_settings_version")
	}
	spec, err := adminsettings.DecodeCanonicalJSON([]byte(encoded))
	if err != nil {
		return record, fmt.Errorf("invalid persisted admin settings: %w", err)
	}
	record.Spec = spec
	record.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated)
	if err != nil {
		return AdminSettingsRecord{}, fmt.Errorf("parse admin settings updated_at: %w", err)
	}
	return record, nil
}
