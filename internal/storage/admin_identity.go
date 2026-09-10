package storage

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"time"
	"unicode/utf8"

	"github.com/sxamx/modelcairn/internal/adminauth"
	"github.com/sxamx/modelcairn/internal/adminsettings"
)

const (
	CodeInstallationIncomplete = "installation_incomplete"
	CodeInvalidUsername        = "invalid_username"
)

type AdminIdentity struct {
	ID          string
	Username    string
	AuthVersion int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func validateUsername(username string) error {
	if !utf8.ValidString(username) || utf8.RuneCountInString(username) < 1 || utf8.RuneCountInString(username) > 120 {
		return &RepositoryError{Code: CodeInvalidUsername}
	}
	return nil
}

func passwordParameters(spec adminsettings.Resolved) adminauth.Parameters {
	return adminauth.Parameters{MemoryKiB: uint32(spec.ArgonMemoryKiB), Iterations: uint32(spec.ArgonIterations)}
}

// BootstrapAdmin atomically creates the sole administrator, initial settings and
// typed audit record. Hashing happens before the write transaction.
func BootstrapAdmin(ctx context.Context, i *Installation, username string, password []byte, spec adminsettings.Resolved) (AdminIdentity, AdminSettingsRecord, error) {
	if i == nil {
		return AdminIdentity{}, AdminSettingsRecord{}, errors.New("installation_required")
	}
	if err := validateUsername(username); err != nil {
		return AdminIdentity{}, AdminSettingsRecord{}, err
	}
	if err := adminsettings.Validate(spec); err != nil {
		return AdminIdentity{}, AdminSettingsRecord{}, err
	}
	phc, err := adminauth.Hash(password, passwordParameters(spec))
	if err != nil {
		return AdminIdentity{}, AdminSettingsRecord{}, err
	}
	tx, err := i.DB().BeginTx(ctx, nil)
	if err != nil {
		return AdminIdentity{}, AdminSettingsRecord{}, err
	}
	defer tx.Rollback()
	var admins, settings int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM admin_users").Scan(&admins); err != nil {
		return AdminIdentity{}, AdminSettingsRecord{}, err
	}
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM admin_settings").Scan(&settings); err != nil {
		return AdminIdentity{}, AdminSettingsRecord{}, err
	}
	if admins != 0 || settings != 0 {
		if admins == 1 && settings == 1 {
			return AdminIdentity{}, AdminSettingsRecord{}, &RepositoryError{Code: CodeAlreadyExists}
		}
		return AdminIdentity{}, AdminSettingsRecord{}, &RepositoryError{Code: CodeInstallationIncomplete}
	}
	id, err := newUUID()
	if err != nil {
		return AdminIdentity{}, AdminSettingsRecord{}, err
	}
	now := time.Now().UTC()
	stamp := now.Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `INSERT INTO admin_users(id,username,password_phc,auth_version,created_at,updated_at) VALUES(?,?,?,1,?,?)`, id, username, phc, stamp, stamp); err != nil {
		if isUniqueConstraint(err) {
			return AdminIdentity{}, AdminSettingsRecord{}, &RepositoryError{Code: CodeAlreadyExists}
		}
		return AdminIdentity{}, AdminSettingsRecord{}, errors.New("admin_bootstrap_failed")
	}
	settingsRecord, err := InsertAdminSettingsTx(ctx, tx, spec, now)
	if err != nil {
		return AdminIdentity{}, AdminSettingsRecord{}, err
	}
	actor := Actor{Type: "cli", ID: id}
	if err := insertAudit(ctx, tx, now, actor, auditRecord{Action: "admin.bootstrap", Kind: ResourceKind("Admin"), ResourceID: id, Result: "success", Version: 1}); err != nil {
		return AdminIdentity{}, AdminSettingsRecord{}, errors.New("admin_bootstrap_audit_failed")
	}
	if err := tx.Commit(); err != nil {
		return AdminIdentity{}, AdminSettingsRecord{}, errors.New("admin_bootstrap_commit_failed")
	}
	return AdminIdentity{ID: id, Username: username, AuthVersion: 1, CreatedAt: now, UpdatedAt: now}, settingsRecord, nil
}

// ResetAdminPassword updates the sole administrator and revokes every session in
// one transaction. The caller must own the installation through the offline lock.
func ResetAdminPassword(ctx context.Context, i *Installation, password []byte) (AdminIdentity, error) {
	if i == nil {
		return AdminIdentity{}, errors.New("installation_required")
	}
	settings, err := ReadAdminSettings(ctx, i.DB())
	if err != nil {
		return AdminIdentity{}, err
	}
	phc, err := adminauth.Hash(password, passwordParameters(settings.Spec))
	if err != nil {
		return AdminIdentity{}, err
	}
	tx, err := i.DB().BeginTx(ctx, nil)
	if err != nil {
		return AdminIdentity{}, err
	}
	defer tx.Rollback()
	currentSettings, err := ReadAdminSettingsTx(ctx, tx)
	if err != nil {
		return AdminIdentity{}, err
	}
	if currentSettings.ResourceVersion != settings.ResourceVersion {
		return AdminIdentity{}, &RepositoryError{Code: CodeVersionConflict}
	}
	var result AdminIdentity
	var created, updated string
	err = tx.QueryRowContext(ctx, "SELECT id,username,auth_version,created_at,updated_at FROM admin_users").Scan(&result.ID, &result.Username, &result.AuthVersion, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return AdminIdentity{}, &RepositoryError{Code: CodeInstallationIncomplete}
	}
	if err != nil {
		return AdminIdentity{}, err
	}
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM admin_users").Scan(&count); err != nil {
		return AdminIdentity{}, err
	}
	if count != 1 || result.AuthVersion < 1 || result.AuthVersion == math.MaxInt64 {
		return AdminIdentity{}, &RepositoryError{Code: CodeInstallationIncomplete}
	}
	result.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return AdminIdentity{}, &RepositoryError{Code: CodeInstallationIncomplete}
	}
	if _, err := time.Parse(time.RFC3339Nano, updated); err != nil {
		return AdminIdentity{}, &RepositoryError{Code: CodeInstallationIncomplete}
	}
	now := time.Now().UTC()
	stamp := now.Format(time.RFC3339Nano)
	update, err := tx.ExecContext(ctx, "UPDATE admin_users SET password_phc=?,auth_version=auth_version+1,updated_at=? WHERE id=? AND auth_version=?", phc, stamp, result.ID, result.AuthVersion)
	if err != nil {
		return AdminIdentity{}, errors.New("admin_password_reset_failed")
	}
	if affected, _ := update.RowsAffected(); affected != 1 {
		return AdminIdentity{}, &RepositoryError{Code: CodeVersionConflict}
	}
	if _, err := tx.ExecContext(ctx, "UPDATE admin_sessions SET revoked_at=? WHERE admin_id=? AND revoked_at IS NULL", stamp, result.ID); err != nil {
		return AdminIdentity{}, errors.New("admin_session_revocation_failed")
	}
	actor := Actor{Type: "cli", ID: result.ID}
	if err := insertAudit(ctx, tx, now, actor, auditRecord{Action: "admin.password_reset", Kind: ResourceKind("Admin"), ResourceID: result.ID, Result: "success", Version: result.AuthVersion + 1}); err != nil {
		return AdminIdentity{}, errors.New("admin_password_reset_audit_failed")
	}
	if err := tx.Commit(); err != nil {
		return AdminIdentity{}, errors.New("admin_password_reset_commit_failed")
	}
	result.AuthVersion++
	result.UpdatedAt = now
	return result, nil
}
