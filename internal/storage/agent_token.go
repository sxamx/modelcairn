package storage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const agentTokenPrefix = "mc_at_v1_"

var (
	ErrAgentTokenInvalid   = errors.New("agent_token_invalid")
	ErrAgentRouteForbidden = errors.New("agent_route_forbidden")
)

type AgentTokenStatus struct {
	State     string     `json:"state"`
	Prefix    *string    `json:"prefix,omitempty"`
	IssuedAt  *time.Time `json:"issuedAt,omitempty"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	RevokedAt *time.Time `json:"revokedAt,omitempty"`
}

type IssuedAgentToken struct {
	Token  string           `json:"token"`
	Status AgentTokenStatus `json:"tokenStatus"`
}

type AgentIdentity struct {
	ResourceID string
	Name       string
}

type AgentTokenService struct {
	db      *sql.DB
	secrets *SecretStore
	now     func() time.Time
}

func NewAgentTokenService(i *Installation) (*AgentTokenService, error) {
	if i == nil || i.DB() == nil || i.Secrets() == nil {
		return nil, errors.New("installation_required")
	}
	return &AgentTokenService{db: i.DB(), secrets: i.Secrets(), now: time.Now}, nil
}

func (s *AgentTokenService) IssueSession(ctx context.Context, name, sessionToken, csrfToken string) (IssuedAgentToken, error) {
	var result IssuedAgentToken
	err := s.secrets.ExecuteAdminMutation(ctx, sessionToken, csrfToken, func(tx *sql.Tx, actor Actor) error {
		resource, err := GetResourceTx(ctx, tx, KindAgentToken, name)
		if err != nil {
			return err
		}
		row, err := readAgentTokenRow(ctx, tx, resource.ID)
		if err != nil {
			return err
		}
		now := s.now().UTC()
		if row.verifier != nil || row.issuedAt != nil || row.revokedAt != nil || (row.expiresAt != nil && !now.Before(*row.expiresAt)) {
			return &RepositoryError{Code: CodeAlreadyExists}
		}
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			return fmt.Errorf("generate agent token: %w", err)
		}
		encoded := base64.RawURLEncoding.EncodeToString(raw)
		clear(raw)
		bearer := agentTokenPrefix + encoded
		verifier := sha256.Sum256([]byte(bearer))
		prefix := agentTokenPrefix + encoded[:8]
		stamp := now.Format(time.RFC3339Nano)
		updated, err := tx.ExecContext(ctx, `UPDATE agent_tokens SET verifier_sha256=?,token_prefix=?,issued_at=?
			WHERE resource_id=? AND verifier_sha256 IS NULL AND issued_at IS NULL AND revoked_at IS NULL`, verifier[:], prefix, stamp, resource.ID)
		if err != nil {
			return fmt.Errorf("issue agent token: %w", err)
		}
		if count, _ := updated.RowsAffected(); count != 1 {
			return &RepositoryError{Code: CodeAlreadyExists}
		}
		if err := insertAudit(ctx, tx, now, actor, auditRecord{Action: "agent_token.issue", Kind: KindAgentToken, ResourceID: resource.ID, Result: "success", Version: resource.ResourceVersion}); err != nil {
			return err
		}
		result = IssuedAgentToken{Token: bearer, Status: statusFromAgentRowAt(agentTokenRow{prefix: &prefix, issuedAt: &now, expiresAt: row.expiresAt}, now)}
		return nil
	})
	if err != nil {
		return IssuedAgentToken{}, err
	}
	return result, nil
}

func (s *AgentTokenService) RevokeSession(ctx context.Context, name, sessionToken, csrfToken string) error {
	return s.secrets.ExecuteAdminMutation(ctx, sessionToken, csrfToken, func(tx *sql.Tx, actor Actor) error {
		resource, err := GetResourceTx(ctx, tx, KindAgentToken, name)
		if err != nil {
			return err
		}
		row, err := readAgentTokenRow(ctx, tx, resource.ID)
		if err != nil {
			return err
		}
		if row.revokedAt != nil {
			return nil
		}
		now := s.now().UTC()
		stamp := now.Format(time.RFC3339Nano)
		updated, err := tx.ExecContext(ctx, "UPDATE agent_tokens SET revoked_at=? WHERE resource_id=? AND revoked_at IS NULL", stamp, resource.ID)
		if err != nil {
			return fmt.Errorf("revoke agent token: %w", err)
		}
		if count, _ := updated.RowsAffected(); count != 1 {
			return &RepositoryError{Code: CodeNotFound}
		}
		return insertAudit(ctx, tx, now, actor, auditRecord{Action: "agent_token.revoke", Kind: KindAgentToken, ResourceID: resource.ID, Result: "success", Version: resource.ResourceVersion})
	})
}

func (s *AgentTokenService) Status(ctx context.Context, name string) (AgentTokenStatus, error) {
	resource, err := NewRepository(s.db).Get(ctx, KindAgentToken, name)
	if err != nil {
		return AgentTokenStatus{}, err
	}
	row, err := readAgentTokenRowDB(ctx, s.db, resource.ID)
	if err != nil {
		return AgentTokenStatus{}, err
	}
	return statusFromAgentRowAt(row, s.now().UTC()), nil
}

// Authenticate verifies a bearer and authorizes one resolved route identity.
// All invalid lifecycle states intentionally collapse to ErrAgentTokenInvalid.
func (s *AgentTokenService) Authenticate(ctx context.Context, bearer, routeID string) (AgentIdentity, error) {
	if routeID == "" {
		return AgentIdentity{}, &RepositoryError{Code: CodeInvalidResource}
	}
	verifier, ok := parseAgentToken(bearer)
	if !ok {
		return AgentIdentity{}, ErrAgentTokenInvalid
	}
	var identity AgentIdentity
	var expiresAt, revokedAt sql.NullString
	var allowed string
	err := s.db.QueryRowContext(ctx, `SELECT r.id,r.name,a.expires_at,a.revoked_at,a.allowed_routes_json
		FROM agent_tokens a JOIN resources r ON r.id=a.resource_id
		WHERE a.verifier_sha256=? AND r.kind=?`, verifier[:], KindAgentToken).Scan(&identity.ResourceID, &identity.Name, &expiresAt, &revokedAt, &allowed)
	if errors.Is(err, sql.ErrNoRows) {
		return AgentIdentity{}, ErrAgentTokenInvalid
	}
	if err != nil {
		return AgentIdentity{}, fmt.Errorf("authenticate agent token: %w", err)
	}
	if revokedAt.Valid {
		return AgentIdentity{}, ErrAgentTokenInvalid
	}
	if expiresAt.Valid {
		expires, err := time.Parse(time.RFC3339Nano, expiresAt.String)
		if err != nil || !s.now().UTC().Before(expires) {
			return AgentIdentity{}, ErrAgentTokenInvalid
		}
	}
	var routes []string
	if err := json.Unmarshal([]byte(allowed), &routes); err != nil {
		return AgentIdentity{}, fmt.Errorf("decode agent routes: %w", err)
	}
	for _, allowedID := range routes {
		if allowedID == routeID {
			return identity, nil
		}
	}
	return AgentIdentity{}, ErrAgentRouteForbidden
}

type agentTokenRow struct {
	verifier  []byte
	prefix    *string
	issuedAt  *time.Time
	expiresAt *time.Time
	revokedAt *time.Time
}

func readAgentTokenRow(ctx context.Context, tx *sql.Tx, resourceID string) (agentTokenRow, error) {
	return scanAgentTokenRow(tx.QueryRowContext(ctx, "SELECT verifier_sha256,token_prefix,issued_at,expires_at,revoked_at FROM agent_tokens WHERE resource_id=?", resourceID))
}

func readAgentTokenRowDB(ctx context.Context, db *sql.DB, resourceID string) (agentTokenRow, error) {
	return scanAgentTokenRow(db.QueryRowContext(ctx, "SELECT verifier_sha256,token_prefix,issued_at,expires_at,revoked_at FROM agent_tokens WHERE resource_id=?", resourceID))
}

func scanAgentTokenRow(row rowScanner) (agentTokenRow, error) {
	var verifier []byte
	var prefix, issued, expires, revoked sql.NullString
	if err := row.Scan(&verifier, &prefix, &issued, &expires, &revoked); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return agentTokenRow{}, &RepositoryError{Code: CodeNotFound}
		}
		return agentTokenRow{}, err
	}
	result := agentTokenRow{verifier: verifier}
	if prefix.Valid {
		result.prefix = &prefix.String
	}
	parseOptional := func(value sql.NullString, target **time.Time) error {
		if !value.Valid {
			return nil
		}
		parsed, err := time.Parse(time.RFC3339Nano, value.String)
		if err != nil {
			return err
		}
		*target = &parsed
		return nil
	}
	if err := parseOptional(issued, &result.issuedAt); err != nil {
		return agentTokenRow{}, err
	}
	if err := parseOptional(expires, &result.expiresAt); err != nil {
		return agentTokenRow{}, err
	}
	if err := parseOptional(revoked, &result.revokedAt); err != nil {
		return agentTokenRow{}, err
	}
	return result, nil
}

func statusFromAgentRowAt(row agentTokenRow, now time.Time) AgentTokenStatus {
	state := "unissued"
	if row.issuedAt != nil {
		state = "active"
	}
	if row.expiresAt != nil && !now.Before(*row.expiresAt) {
		state = "expired"
	}
	if row.revokedAt != nil {
		state = "revoked"
	}
	return AgentTokenStatus{State: state, Prefix: row.prefix, IssuedAt: row.issuedAt, ExpiresAt: row.expiresAt, RevokedAt: row.revokedAt}
}

func parseAgentToken(value string) ([sha256.Size]byte, bool) {
	var verifier [sha256.Size]byte
	if len(value) != len(agentTokenPrefix)+43 || !strings.HasPrefix(value, agentTokenPrefix) {
		return verifier, false
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(value[len(agentTokenPrefix):])
	if err != nil || len(raw) != 32 {
		return verifier, false
	}
	clear(raw)
	return sha256.Sum256([]byte(value)), true
}
