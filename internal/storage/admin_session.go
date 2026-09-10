package storage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"time"
)

const (
	adminTokenBytes    = 32
	previousCSRFWindow = 60 * time.Second
)

var (
	ErrAdminSessionInvalid      = errors.New("admin_session_invalid")
	ErrAdminSessionStateInvalid = errors.New("admin_session_state_invalid")
)

type VerifiedAdmin struct {
	ID          string
	Username    string
	AuthVersion int64
}

type AdminSessionCredentials struct {
	SessionToken string
	CSRFToken    string
	Admin        AdminIdentity
	ExpiresAt    time.Time
}

type AdminSessionContext struct {
	Admin     AdminIdentity
	ExpiresAt time.Time
	CSRFToken string
}

type sessionRow struct {
	idHash            []byte
	admin             AdminIdentity
	csrfHash          []byte
	previousHash      []byte
	csrfRotatedAt     time.Time
	createdAt         time.Time
	lastSeenAt        time.Time
	absoluteExpiresAt time.Time
	idleSeconds       int
}

// CreateAdminSession rechecks auth_version inside the write transaction. The
// VerifiedAdmin must come from the internal password-verification service.
func CreateAdminSession(ctx context.Context, i *Installation, verified VerifiedAdmin, idleSeconds, absoluteSeconds int, now time.Time) (AdminSessionCredentials, error) {
	if i == nil {
		return AdminSessionCredentials{}, errors.New("installation_required")
	}
	if verified.ID == "" || verified.Username == "" || verified.AuthVersion < 1 || idleSeconds < 300 || idleSeconds > 86400 || absoluteSeconds < 300 || absoluteSeconds > 604800 || idleSeconds > absoluteSeconds {
		return AdminSessionCredentials{}, ErrAdminSessionStateInvalid
	}
	sessionToken, sessionHash, err := newAdminToken()
	if err != nil {
		return AdminSessionCredentials{}, err
	}
	csrfToken, csrfHash, err := newAdminToken()
	if err != nil {
		return AdminSessionCredentials{}, err
	}
	now = now.UTC()
	expires := now.Add(time.Duration(absoluteSeconds) * time.Second)
	tx, err := i.DB().BeginTx(ctx, nil)
	if err != nil {
		return AdminSessionCredentials{}, err
	}
	defer tx.Rollback()
	var username, created, updated string
	var authVersion int64
	err = tx.QueryRowContext(ctx, "SELECT username,auth_version,created_at,updated_at FROM admin_users WHERE id=?", verified.ID).Scan(&username, &authVersion, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) || err == nil && (username != verified.Username || authVersion != verified.AuthVersion) {
		return AdminSessionCredentials{}, &RepositoryError{Code: CodeVersionConflict}
	}
	if err != nil {
		return AdminSessionCredentials{}, err
	}
	createdAt, e1 := time.Parse(time.RFC3339Nano, created)
	updatedAt, e2 := time.Parse(time.RFC3339Nano, updated)
	if e1 != nil || e2 != nil {
		return AdminSessionCredentials{}, ErrAdminSessionStateInvalid
	}
	if now.Before(createdAt) {
		return AdminSessionCredentials{}, ErrAdminSessionStateInvalid
	}
	stamp := now.Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `INSERT INTO admin_sessions(id_hash,admin_id,auth_version,csrf_hash,csrf_previous_hash,csrf_rotated_at,created_at,last_seen_at,expires_at,revoked_at,idle_seconds) VALUES(?,?,?,?,NULL,?,?,?,?,NULL,?)`, sessionHash[:], verified.ID, verified.AuthVersion, csrfHash[:], stamp, stamp, stamp, expires.Format(time.RFC3339Nano), idleSeconds)
	if err != nil {
		return AdminSessionCredentials{}, errors.New("admin_session_create_failed")
	}
	actor := Actor{Type: "admin", ID: verified.ID}
	if err := insertAudit(ctx, tx, now, actor, auditRecord{Action: "admin.session_create", Kind: ResourceKind("AdminSession"), Result: "success", Version: verified.AuthVersion}); err != nil {
		return AdminSessionCredentials{}, errors.New("admin_session_audit_failed")
	}
	if err := tx.Commit(); err != nil {
		return AdminSessionCredentials{}, errors.New("admin_session_commit_failed")
	}
	admin := AdminIdentity{ID: verified.ID, Username: username, AuthVersion: authVersion, CreatedAt: createdAt, UpdatedAt: updatedAt}
	return AdminSessionCredentials{SessionToken: sessionToken, CSRFToken: csrfToken, Admin: admin, ExpiresAt: minTime(expires, now.Add(time.Duration(idleSeconds)*time.Second))}, nil
}

// UseAdminSession authenticates a session and optionally its CSRF token. A valid
// request advances last_seen only after every required check succeeds.
func UseAdminSession(ctx context.Context, i *Installation, sessionToken, csrfToken string, requireCSRF bool, now time.Time) (AdminSessionContext, error) {
	if i == nil {
		return AdminSessionContext{}, errors.New("installation_required")
	}
	sessionHash, err := decodeAdminToken(sessionToken)
	if err != nil {
		return AdminSessionContext{}, ErrAdminSessionInvalid
	}
	var csrfHash [sha256.Size]byte
	if requireCSRF {
		decoded, err := decodeAdminToken(csrfToken)
		if err != nil {
			return AdminSessionContext{}, ErrAdminSessionInvalid
		}
		csrfHash = decoded
	}
	tx, err := i.DB().BeginTx(ctx, nil)
	if err != nil {
		return AdminSessionContext{}, err
	}
	defer tx.Rollback()
	row, err := readLiveSession(ctx, tx, sessionHash, now)
	if err != nil {
		return AdminSessionContext{}, err
	}
	if requireCSRF && !validCSRF(row, csrfHash, now.UTC()) {
		return AdminSessionContext{}, ErrAdminSessionInvalid
	}
	now = now.UTC()
	activityAt := now
	if activityAt.Before(row.lastSeenAt) {
		activityAt = row.lastSeenAt
	}
	result, err := tx.ExecContext(ctx, "UPDATE admin_sessions SET last_seen_at=? WHERE id_hash=? AND revoked_at IS NULL", activityAt.Format(time.RFC3339Nano), sessionHash[:])
	if err != nil {
		return AdminSessionContext{}, errors.New("admin_session_touch_failed")
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return AdminSessionContext{}, ErrAdminSessionInvalid
	}
	if err := tx.Commit(); err != nil {
		return AdminSessionContext{}, errors.New("admin_session_commit_failed")
	}
	return AdminSessionContext{Admin: row.admin, ExpiresAt: minTime(row.absoluteExpiresAt, activityAt.Add(time.Duration(row.idleSeconds)*time.Second))}, nil
}

// RotateAdminSessionCSRF implements session/me recovery. It retains exactly one
// previous hash; callers enforce the same-origin browser boundary.
func RotateAdminSessionCSRF(ctx context.Context, i *Installation, sessionToken string, now time.Time) (AdminSessionContext, error) {
	if i == nil {
		return AdminSessionContext{}, errors.New("installation_required")
	}
	sessionHash, err := decodeAdminToken(sessionToken)
	if err != nil {
		return AdminSessionContext{}, ErrAdminSessionInvalid
	}
	csrfToken, csrfHash, err := newAdminToken()
	if err != nil {
		return AdminSessionContext{}, err
	}
	tx, err := i.DB().BeginTx(ctx, nil)
	if err != nil {
		return AdminSessionContext{}, err
	}
	defer tx.Rollback()
	row, err := readLiveSession(ctx, tx, sessionHash, now)
	if err != nil {
		return AdminSessionContext{}, err
	}
	now = now.UTC()
	activityAt := now
	if activityAt.Before(row.lastSeenAt) {
		activityAt = row.lastSeenAt
	}
	rotationAt := activityAt
	if rotationAt.Before(row.csrfRotatedAt) {
		rotationAt = row.csrfRotatedAt
	}
	result, err := tx.ExecContext(ctx, "UPDATE admin_sessions SET csrf_previous_hash=csrf_hash,csrf_hash=?,csrf_rotated_at=?,last_seen_at=? WHERE id_hash=? AND revoked_at IS NULL", csrfHash[:], rotationAt.Format(time.RFC3339Nano), activityAt.Format(time.RFC3339Nano), sessionHash[:])
	if err != nil {
		return AdminSessionContext{}, errors.New("admin_session_rotate_failed")
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return AdminSessionContext{}, ErrAdminSessionInvalid
	}
	if err := tx.Commit(); err != nil {
		return AdminSessionContext{}, errors.New("admin_session_commit_failed")
	}
	return AdminSessionContext{Admin: row.admin, ExpiresAt: minTime(row.absoluteExpiresAt, activityAt.Add(time.Duration(row.idleSeconds)*time.Second)), CSRFToken: csrfToken}, nil
}

func RevokeAdminSession(ctx context.Context, i *Installation, sessionToken, csrfToken string, now time.Time) error {
	if i == nil {
		return errors.New("installation_required")
	}
	sessionHash, err := decodeAdminToken(sessionToken)
	if err != nil {
		return ErrAdminSessionInvalid
	}
	provided, err := decodeAdminToken(csrfToken)
	if err != nil {
		return ErrAdminSessionInvalid
	}
	tx, err := i.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	row, err := readLiveSession(ctx, tx, sessionHash, now)
	if err != nil {
		return err
	}
	if !validCSRF(row, provided, now.UTC()) {
		return ErrAdminSessionInvalid
	}
	now = now.UTC()
	stamp := now.Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, "UPDATE admin_sessions SET revoked_at=? WHERE id_hash=? AND revoked_at IS NULL", stamp, sessionHash[:])
	if err != nil {
		return errors.New("admin_session_revoke_failed")
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return ErrAdminSessionInvalid
	}
	if err := insertAudit(ctx, tx, now, Actor{Type: "admin", ID: row.admin.ID}, auditRecord{Action: "admin.session_logout", Kind: ResourceKind("AdminSession"), Result: "success", Version: row.admin.AuthVersion}); err != nil {
		return errors.New("admin_session_audit_failed")
	}
	if err := tx.Commit(); err != nil {
		return errors.New("admin_session_commit_failed")
	}
	return nil
}

func readLiveSession(ctx context.Context, tx *sql.Tx, idHash [sha256.Size]byte, now time.Time) (sessionRow, error) {
	var row sessionRow
	var rotated, last, expires, created, updated, sessionCreated string
	var revoked sql.NullString
	var idle sql.NullInt64
	var sessionAuth, currentAuth int64
	err := tx.QueryRowContext(ctx, `SELECT s.id_hash,u.id,u.username,s.auth_version,u.auth_version,u.created_at,u.updated_at,s.csrf_hash,s.csrf_previous_hash,s.csrf_rotated_at,s.created_at,s.last_seen_at,s.expires_at,s.revoked_at,s.idle_seconds FROM admin_sessions s JOIN admin_users u ON u.id=s.admin_id WHERE s.id_hash=?`, idHash[:]).Scan(&row.idHash, &row.admin.ID, &row.admin.Username, &sessionAuth, &currentAuth, &created, &updated, &row.csrfHash, &row.previousHash, &rotated, &sessionCreated, &last, &expires, &revoked, &idle)
	if errors.Is(err, sql.ErrNoRows) {
		return row, ErrAdminSessionInvalid
	}
	if err != nil {
		return row, err
	}
	row.admin.AuthVersion = currentAuth
	var e error
	row.admin.CreatedAt, e = time.Parse(time.RFC3339Nano, created)
	if e != nil {
		return row, ErrAdminSessionStateInvalid
	}
	row.admin.UpdatedAt, e = time.Parse(time.RFC3339Nano, updated)
	if e != nil {
		return row, ErrAdminSessionStateInvalid
	}
	row.csrfRotatedAt, e = time.Parse(time.RFC3339Nano, rotated)
	if e != nil {
		return row, ErrAdminSessionStateInvalid
	}
	row.createdAt, e = time.Parse(time.RFC3339Nano, sessionCreated)
	if e != nil {
		return row, ErrAdminSessionStateInvalid
	}
	row.lastSeenAt, e = time.Parse(time.RFC3339Nano, last)
	if e != nil {
		return row, ErrAdminSessionStateInvalid
	}
	row.absoluteExpiresAt, e = time.Parse(time.RFC3339Nano, expires)
	if e != nil {
		return row, ErrAdminSessionStateInvalid
	}
	if revoked.Valid || !idle.Valid || idle.Int64 < 300 || idle.Int64 > 86400 || currentAuth < 1 || sessionAuth != currentAuth || len(row.idHash) != sha256.Size || len(row.csrfHash) != sha256.Size || (row.previousHash != nil && len(row.previousHash) != sha256.Size) {
		return row, ErrAdminSessionInvalid
	}
	row.idleSeconds = int(idle.Int64)
	now = now.UTC()
	if row.createdAt.Before(row.admin.CreatedAt) || row.lastSeenAt.Before(row.createdAt) || !row.absoluteExpiresAt.After(row.createdAt) || row.csrfRotatedAt.Before(row.createdAt) {
		return row, ErrAdminSessionStateInvalid
	}
	if !now.Before(row.absoluteExpiresAt) || !now.Before(row.lastSeenAt.Add(time.Duration(row.idleSeconds)*time.Second)) {
		return row, ErrAdminSessionInvalid
	}
	return row, nil
}

func validCSRF(row sessionRow, provided [sha256.Size]byte, now time.Time) bool {
	if subtle.ConstantTimeCompare(row.csrfHash, provided[:]) == 1 {
		return true
	}
	return len(row.previousHash) == sha256.Size && !now.Before(row.csrfRotatedAt) && !now.After(row.csrfRotatedAt.Add(previousCSRFWindow)) && subtle.ConstantTimeCompare(row.previousHash, provided[:]) == 1
}

func newAdminToken() (string, [sha256.Size]byte, error) {
	var raw [adminTokenBytes]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", [sha256.Size]byte{}, errors.New("admin_token_random_unavailable")
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	clear(raw[:])
	return token, sha256.Sum256([]byte(token)), nil
}
func decodeAdminToken(token string) ([sha256.Size]byte, error) {
	if len(token) != base64.RawURLEncoding.EncodedLen(adminTokenBytes) {
		return [sha256.Size]byte{}, ErrAdminSessionInvalid
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(raw) != adminTokenBytes {
		return [sha256.Size]byte{}, ErrAdminSessionInvalid
	}
	canonical := base64.RawURLEncoding.EncodeToString(raw)
	clear(raw)
	if canonical != token {
		return [sha256.Size]byte{}, ErrAdminSessionInvalid
	}
	return sha256.Sum256([]byte(token)), nil
}
func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
