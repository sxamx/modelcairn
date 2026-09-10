package storage

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
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

func ReadAdminSettingsTx(ctx context.Context, tx *sql.Tx) (AdminSettingsRecord, error) {
	return scanAdminSettings(tx.QueryRowContext(ctx, "SELECT resource_version,spec_json,updated_at FROM admin_settings WHERE singleton=1"))
}

// UpdateAdminSettingsTx must run inside ExecuteSettingsPlan. The caller must
// roll back on any error, including an audit failure. No-op updates still audit
// the operation but preserve the stored revision and timestamp.
func UpdateAdminSettingsTx(ctx context.Context, tx *sql.Tx, expected int64, spec adminsettings.Resolved, actor Actor, now time.Time) (AdminSettingsRecord, error) {
	if err := validateActor(actor); err != nil {
		return AdminSettingsRecord{}, err
	}
	if expected < 1 {
		return AdminSettingsRecord{}, &RepositoryError{Code: CodeVersionConflict}
	}
	encoded, err := adminsettings.CanonicalJSON(spec)
	if err != nil {
		return AdminSettingsRecord{}, errors.New("invalid_admin_settings")
	}
	validated, err := adminsettings.DecodeCanonicalJSON(encoded)
	if err != nil {
		return AdminSettingsRecord{}, err
	}
	current, err := ReadAdminSettingsTx(ctx, tx)
	if err != nil {
		return AdminSettingsRecord{}, err
	}
	if current.ResourceVersion != expected {
		return AdminSettingsRecord{}, &RepositoryError{Code: CodeVersionConflict}
	}
	before, err := adminsettings.CanonicalJSON(current.Spec)
	if err != nil {
		return AdminSettingsRecord{}, err
	}
	changed := []string{}
	result := current
	if !bytes.Equal(before, encoded) {
		if expected == math.MaxInt64 {
			return AdminSettingsRecord{}, errors.New("admin_settings_version_exhausted")
		}
		var oldFields, newFields map[string]json.RawMessage
		if json.Unmarshal(before, &oldFields) != nil || json.Unmarshal(encoded, &newFields) != nil {
			return AdminSettingsRecord{}, errors.New("invalid_admin_settings")
		}
		for name, value := range newFields {
			if !bytes.Equal(oldFields[name], value) {
				changed = append(changed, name)
			}
		}
		sort.Strings(changed)
		update, err := tx.ExecContext(ctx, "UPDATE admin_settings SET resource_version=?,spec_json=?,updated_at=? WHERE singleton=1 AND resource_version=?", expected+1, string(encoded), now.UTC().Format(time.RFC3339Nano), expected)
		if err != nil {
			return AdminSettingsRecord{}, errors.New("admin_settings_write_failed")
		}
		count, err := update.RowsAffected()
		if err != nil {
			return AdminSettingsRecord{}, errors.New("admin_settings_write_failed")
		}
		if count != 1 {
			return AdminSettingsRecord{}, &RepositoryError{Code: CodeVersionConflict}
		}
		result = AdminSettingsRecord{ResourceVersion: expected + 1, Spec: validated, UpdatedAt: now.UTC()}
	}
	// Details are derived from validated typed settings; no values or caller maps.
	details, err := json.Marshal(struct {
		PreviousVersion int64    `json:"previousVersion"`
		Version         int64    `json:"version"`
		ChangedFields   []string `json:"changedFields"`
	}{expected, result.ResourceVersion, changed})
	if err != nil {
		return AdminSettingsRecord{}, errors.New("admin_settings_audit_failed")
	}
	id, err := newUUID()
	if err != nil {
		return AdminSettingsRecord{}, err
	}
	var actorID any
	if actor.ID != "" {
		actorID = actor.ID
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_events(id,actor_type,actor_id,action,resource_kind,resource_id,result,details_json,occurred_at)
		VALUES(?,?,?,'admin_settings.apply','AdminSettings',NULL,'success',?,?)`, id, actor.Type, actorID, string(details), now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return AdminSettingsRecord{}, errors.New("admin_settings_audit_failed")
	}
	return result, nil
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
