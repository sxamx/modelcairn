package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type ResourceKind string

const (
	KindProvider           ResourceKind = "Provider"
	KindProviderAccount    ResourceKind = "ProviderAccount"
	KindProviderConnection ResourceKind = "ProviderConnection"
	KindCredential         ResourceKind = "Credential"
	KindEgress             ResourceKind = "Egress"
	KindModel              ResourceKind = "Model"
	KindDestination        ResourceKind = "Destination"
	KindStrategy           ResourceKind = "Strategy"
	KindRoute              ResourceKind = "Route"
	KindAgentToken         ResourceKind = "AgentToken"
)

var validKinds = map[ResourceKind]struct{}{
	KindProvider: {}, KindProviderAccount: {}, KindProviderConnection: {},
	KindCredential: {}, KindEgress: {}, KindModel: {}, KindDestination: {},
	KindStrategy: {}, KindRoute: {}, KindAgentToken: {},
}

type Resource struct {
	ID              string
	Kind            ResourceKind
	Name            string
	DisplayName     *string
	Description     *string
	ResourceVersion int64
	Spec            json.RawMessage
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type PutResource struct {
	Kind            ResourceKind
	Name            string
	DisplayName     *string
	Description     *string
	Spec            json.RawMessage
	ExpectedVersion int64 // zero creates; a positive value updates exactly that version
}

type Actor struct{ Type, ID string }

const (
	CodeAlreadyExists    = "already_exists"
	CodeInvalidActor     = "invalid_actor"
	CodeInvalidResource  = "invalid_resource"
	CodeNotFound         = "not_found"
	CodeReferenceMissing = "reference_not_found"
	CodeResourceInUse    = "resource_in_use"
	CodeVersionConflict  = "version_conflict"
	CodeDeleteNotAllowed = "delete_not_allowed"
	CodeProviderMismatch = "provider_mismatch"
)

type RepositoryError struct {
	Code string
	Err  error
}

func (e *RepositoryError) Error() string {
	if e.Err == nil {
		return e.Code
	}
	return e.Code + ": " + e.Err.Error()
}
func (e *RepositoryError) Unwrap() error { return e.Err }
func IsRepositoryCode(err error, code string) bool {
	var target *RepositoryError
	return errors.As(err, &target) && target.Code == code
}

// MutationAuditError reports a mutation failure together with an inability to
// durably record its failure audit. Both errors are intentionally secret-free.
type MutationAuditError struct{ Mutation, Audit error }

func (e *MutationAuditError) Error() string {
	return fmt.Sprintf("mutation failed: %v; failure audit unavailable: %v", e.Mutation, e.Audit)
}
func (e *MutationAuditError) Unwrap() error { return e.Mutation }

type Repository struct {
	db  *sql.DB
	now func() time.Time
}

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db, now: time.Now} }

func (r *Repository) Get(ctx context.Context, kind ResourceKind, name string) (Resource, error) {
	return scanResource(r.db.QueryRowContext(ctx, `SELECT id,kind,name,display_name,description,resource_version,spec_json,created_at,updated_at
		FROM resources WHERE kind=? AND name=?`, kind, name))
}

func (r *Repository) List(ctx context.Context) ([]Resource, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,kind,name,display_name,description,resource_version,spec_json,created_at,updated_at
		FROM resources ORDER BY kind,name`)
	if err != nil {
		return nil, fmt.Errorf("list resources: %w", err)
	}
	defer rows.Close()
	var result []Resource
	for rows.Next() {
		item, err := scanResource(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list resources: %w", err)
	}
	return result, nil
}

type rowScanner interface{ Scan(...any) error }

func scanResource(row rowScanner) (Resource, error) {
	var item Resource
	var kind string
	var spec, created, updated string
	if err := row.Scan(&item.ID, &kind, &item.Name, &item.DisplayName, &item.Description, &item.ResourceVersion, &spec, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Resource{}, &RepositoryError{Code: CodeNotFound}
		}
		return Resource{}, fmt.Errorf("scan resource: %w", err)
	}
	item.Kind = ResourceKind(kind)
	item.Spec = json.RawMessage(spec)
	var err error
	if item.CreatedAt, err = time.Parse(time.RFC3339Nano, created); err != nil {
		return Resource{}, fmt.Errorf("parse created_at: %w", err)
	}
	if item.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated); err != nil {
		return Resource{}, fmt.Errorf("parse updated_at: %w", err)
	}
	return item, nil
}

func (r *Repository) Put(ctx context.Context, input PutResource, actor Actor) (Resource, error) {
	if err := validateActor(actor); err != nil {
		return Resource{}, err
	}
	if err := validatePut(input); err != nil {
		return Resource{}, r.fail(ctx, actor, actionFor(input.ExpectedVersion), input.Kind, "", err)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Resource{}, fmt.Errorf("begin resource put: %w", err)
	}
	item, mutationErr := r.putTx(ctx, tx, input)
	if mutationErr == nil {
		mutationErr = insertAudit(ctx, tx, r.now(), actor, auditRecord{Action: actionFor(input.ExpectedVersion), Kind: input.Kind, ResourceID: item.ID, Result: "success", Version: item.ResourceVersion})
	}
	if mutationErr == nil {
		mutationErr = bumpConfigRevisionTx(ctx, tx, r.now())
	}
	if mutationErr != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return Resource{}, fmt.Errorf("mutation failed (%v) and rollback failed: %w", mutationErr, rollbackErr)
		}
		return Resource{}, r.fail(ctx, actor, actionFor(input.ExpectedVersion), input.Kind, item.ID, mutationErr)
	}
	if err := tx.Commit(); err != nil {
		// A commit error has an indeterminate outcome at this boundary. Do not
		// append a misleading failure audit in a second transaction.
		return Resource{}, fmt.Errorf("commit resource mutation: %w", err)
	}
	return item, nil
}

func (r *Repository) putTx(ctx context.Context, tx *sql.Tx, input PutResource) (Resource, error) {
	now := r.now().UTC()
	timestamp := now.Format(time.RFC3339Nano)
	if input.ExpectedVersion == 0 {
		id, err := newUUID()
		if err != nil {
			return Resource{}, fmt.Errorf("generate resource id: %w", err)
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO resources(id,kind,name,display_name,description,resource_version,spec_json,created_at,updated_at)
			VALUES(?,?,?,?,?,1,?,?,?)`, id, input.Kind, input.Name, input.DisplayName, input.Description, string(input.Spec), timestamp, timestamp)
		if err != nil {
			if isUniqueConstraint(err) {
				return Resource{}, &RepositoryError{Code: CodeAlreadyExists}
			}
			return Resource{}, fmt.Errorf("insert resource envelope: %w", err)
		}
		if err := upsertTyped(ctx, tx, id, input.Kind, input.Spec, now); err != nil {
			return Resource{}, err
		}
		return Resource{ID: id, Kind: input.Kind, Name: input.Name, DisplayName: input.DisplayName, Description: input.Description, ResourceVersion: 1, Spec: append(json.RawMessage(nil), input.Spec...), CreatedAt: now, UpdatedAt: now}, nil
	}
	var id string
	var current int64
	var created string
	err := tx.QueryRowContext(ctx, "SELECT id,resource_version,created_at FROM resources WHERE kind=? AND name=?", input.Kind, input.Name).Scan(&id, &current, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return Resource{}, &RepositoryError{Code: CodeNotFound}
	}
	if err != nil {
		return Resource{}, fmt.Errorf("read resource version: %w", err)
	}
	if current != input.ExpectedVersion {
		return Resource{ID: id}, &RepositoryError{Code: CodeVersionConflict}
	}
	result, err := tx.ExecContext(ctx, `UPDATE resources SET display_name=?,description=?,resource_version=resource_version+1,spec_json=?,updated_at=?
		WHERE id=? AND resource_version=?`, input.DisplayName, input.Description, string(input.Spec), timestamp, id, current)
	if err != nil {
		return Resource{}, fmt.Errorf("update resource envelope: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return Resource{ID: id}, &RepositoryError{Code: CodeVersionConflict}
	}
	if err := upsertTyped(ctx, tx, id, input.Kind, input.Spec, now); err != nil {
		return Resource{ID: id}, err
	}
	createdAt, err := time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return Resource{}, err
	}
	return Resource{ID: id, Kind: input.Kind, Name: input.Name, DisplayName: input.DisplayName, Description: input.Description, ResourceVersion: current + 1, Spec: append(json.RawMessage(nil), input.Spec...), CreatedAt: createdAt, UpdatedAt: now}, nil
}

func (r *Repository) Delete(ctx context.Context, kind ResourceKind, name string, expectedVersion int64, allowDelete bool, actor Actor) error {
	if err := validateActor(actor); err != nil {
		return err
	}
	if _, ok := validKinds[kind]; !ok || name == "" || expectedVersion < 1 {
		return r.fail(ctx, actor, "resource.delete", kind, "", &RepositoryError{Code: CodeInvalidResource})
	}
	if !allowDelete {
		return r.fail(ctx, actor, "resource.delete", kind, "", &RepositoryError{Code: CodeDeleteNotAllowed})
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	var id string
	var current int64
	err = tx.QueryRowContext(ctx, "SELECT id,resource_version FROM resources WHERE kind=? AND name=?", kind, name).Scan(&id, &current)
	if errors.Is(err, sql.ErrNoRows) {
		err = &RepositoryError{Code: CodeNotFound}
	}
	if err == nil && current != expectedVersion {
		err = &RepositoryError{Code: CodeVersionConflict}
	}
	if err == nil {
		err = ensureNoLogicalReferences(ctx, tx, kind, name, id)
	}
	if err == nil {
		var result sql.Result
		result, err = tx.ExecContext(ctx, "DELETE FROM resources WHERE id=? AND resource_version=?", id, current)
		if err == nil {
			if count, _ := result.RowsAffected(); count != 1 {
				err = &RepositoryError{Code: CodeVersionConflict}
			}
		}
	}
	if err != nil && isForeignKeyConstraint(err) {
		err = &RepositoryError{Code: CodeResourceInUse}
	}
	if err == nil {
		err = insertAudit(ctx, tx, r.now(), actor, auditRecord{Action: "resource.delete", Kind: kind, ResourceID: id, Result: "success", Version: current})
	}
	if err == nil {
		err = bumpConfigRevisionTx(ctx, tx, r.now())
	}
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("delete failed (%v) and rollback failed: %w", err, rollbackErr)
		}
		return r.fail(ctx, actor, "resource.delete", kind, id, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit resource deletion: %w", err)
	}
	return nil
}

func ensureNoLogicalReferences(ctx context.Context, tx *sql.Tx, kind ResourceKind, name, id string) error {
	var exists int
	switch kind {
	case KindDestination:
		err := tx.QueryRowContext(ctx, `SELECT EXISTS(
			SELECT 1 FROM resources r, json_each(json_extract(r.spec_json,'$.destinations')) d
			WHERE r.kind='Strategy' AND json_extract(d.value,'$.name')=?
		)`, name).Scan(&exists)
		if err != nil {
			return err
		}
	case KindRoute:
		err := tx.QueryRowContext(ctx, `SELECT EXISTS(
			SELECT 1 FROM agent_tokens a, json_each(a.allowed_routes_json) route_id
			WHERE route_id.value=?
		)`, id).Scan(&exists)
		if err != nil {
			return err
		}
	}
	if exists != 0 {
		return &RepositoryError{Code: CodeResourceInUse}
	}
	return nil
}

func (r *Repository) fail(ctx context.Context, actor Actor, action string, kind ResourceKind, id string, mutation error) error {
	code := "operation_failed"
	var repositoryErr *RepositoryError
	if errors.As(mutation, &repositoryErr) {
		code = repositoryErr.Code
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err == nil {
		err = insertAudit(ctx, tx, r.now(), actor, auditRecord{Action: action, Kind: kind, ResourceID: id, Result: "failure", Code: code})
		if err == nil {
			err = tx.Commit()
		} else {
			_ = tx.Rollback()
		}
	}
	if err != nil {
		return &MutationAuditError{Mutation: mutation, Audit: err}
	}
	return mutation
}

func actionFor(expected int64) string {
	if expected <= 0 {
		return "resource.create"
	}
	return "resource.update"
}
func validateActor(actor Actor) error {
	if actor.Type != "admin" && actor.Type != "cli" && actor.Type != "system" {
		return &RepositoryError{Code: CodeInvalidActor}
	}
	return nil
}
func validatePut(input PutResource) error {
	if _, ok := validKinds[input.Kind]; !ok || input.Name == "" || input.ExpectedVersion < 0 || !json.Valid(input.Spec) {
		return &RepositoryError{Code: CodeInvalidResource}
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(input.Spec, &object); err != nil || object == nil {
		return &RepositoryError{Code: CodeInvalidResource}
	}
	return nil
}

type reference struct {
	Name string `json:"name"`
}
type providerAccountSpec struct {
	ProviderRef reference `json:"providerRef"`
}
type providerConnectionSpec struct {
	ProviderRef  reference `json:"providerRef"`
	BaseURL      string    `json:"baseUrl"`
	AllowPrivate bool      `json:"allowPrivateNetwork"`
	Enabled      bool      `json:"enabled"`
}
type credentialSpec struct {
	ProviderAccountRef reference `json:"providerAccountRef"`
	EgressRef          reference `json:"egressRef"`
	SecretRef          reference `json:"secretRef"`
	Enabled            bool      `json:"enabled"`
}
type egressSpec struct {
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
}
type modelSpec struct {
	ConnectionRef   reference `json:"connectionRef"`
	ProviderModelID string    `json:"providerModelId"`
	Capabilities    []string  `json:"capabilities"`
	Enabled         bool      `json:"enabled"`
}
type destinationSpec struct {
	ModelRef      reference `json:"modelRef"`
	CredentialRef reference `json:"credentialRef"`
	Weight        int       `json:"weight"`
	Enabled       bool      `json:"enabled"`
}
type strategySpec struct {
	Destinations []reference `json:"destinations"`
}
type routeSpec struct {
	ModelAlias  string    `json:"modelAlias"`
	StrategyRef reference `json:"strategyRef"`
	Enabled     bool      `json:"enabled"`
}
type agentTokenSpec struct {
	AllowedRoutes []reference `json:"allowedRouteRefs"`
	ExpiresAt     *string     `json:"expiresAt"`
	Enabled       bool        `json:"enabled"`
}

func decodeSpec(raw json.RawMessage, target any) error {
	if err := json.Unmarshal(raw, target); err != nil {
		return &RepositoryError{Code: CodeInvalidResource}
	}
	return nil
}
func resolve(ctx context.Context, tx *sql.Tx, kind ResourceKind, ref reference) (string, error) {
	if ref.Name == "" {
		return "", &RepositoryError{Code: CodeReferenceMissing}
	}
	var id string
	err := tx.QueryRowContext(ctx, "SELECT id FROM resources WHERE kind=? AND name=?", kind, ref.Name).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", &RepositoryError{Code: CodeReferenceMissing}
	}
	if err != nil {
		return "", err
	}
	return id, nil
}

func upsertTyped(ctx context.Context, tx *sql.Tx, id string, kind ResourceKind, raw json.RawMessage, now time.Time) error {
	switch kind {
	case KindProvider:
		return nil
	case KindProviderAccount:
		var s providerAccountSpec
		if err := decodeSpec(raw, &s); err != nil {
			return err
		}
		provider, err := resolve(ctx, tx, KindProvider, s.ProviderRef)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO provider_accounts VALUES(?,?) ON CONFLICT(resource_id) DO NOTHING", id, provider); err != nil {
			return mapConstraint(err)
		}
		_, err = tx.ExecContext(ctx, "UPDATE provider_accounts SET provider_id=? WHERE resource_id=? AND provider_id<>?", provider, id, provider)
		return mapConstraint(err)
	case KindProviderConnection:
		var s providerConnectionSpec
		if err := decodeSpec(raw, &s); err != nil {
			return err
		}
		provider, err := resolve(ctx, tx, KindProvider, s.ProviderRef)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO provider_connections VALUES(?,?,?,?,?) ON CONFLICT(resource_id) DO UPDATE SET base_url=excluded.base_url,allow_private_network=excluded.allow_private_network,enabled=excluded.enabled`, id, provider, s.BaseURL, s.AllowPrivate, s.Enabled); err != nil {
			return mapConstraint(err)
		}
		_, err = tx.ExecContext(ctx, "UPDATE provider_connections SET provider_id=? WHERE resource_id=? AND provider_id<>?", provider, id, provider)
		return mapConstraint(err)
	case KindEgress:
		var s egressSpec
		if err := decodeSpec(raw, &s); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO egresses VALUES(?,?,?) ON CONFLICT(resource_id) DO UPDATE SET egress_type=excluded.egress_type,enabled=excluded.enabled`, id, s.Type, s.Enabled)
		return mapConstraint(err)
	case KindCredential:
		var s credentialSpec
		if err := decodeSpec(raw, &s); err != nil {
			return err
		}
		account, err := resolve(ctx, tx, KindProviderAccount, s.ProviderAccountRef)
		if err != nil {
			return err
		}
		egress, err := resolve(ctx, tx, KindEgress, s.EgressRef)
		if err != nil {
			return err
		}
		var secret string
		err = tx.QueryRowContext(ctx, "SELECT id FROM secrets WHERE name=?", s.SecretRef.Name).Scan(&secret)
		if errors.Is(err, sql.ErrNoRows) {
			return &RepositoryError{Code: CodeReferenceMissing}
		}
		if err != nil {
			return err
		}
		status := "disabled"
		if s.Enabled {
			status = "active"
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO credentials(resource_id,provider_account_id,egress_id,secret_id,status) VALUES(?,?,?,?,?) ON CONFLICT(resource_id) DO UPDATE SET egress_id=excluded.egress_id,secret_id=excluded.secret_id,status=excluded.status,blocked_reason=NULL`, id, account, egress, secret, status); err != nil {
			return mapConstraint(err)
		}
		_, err = tx.ExecContext(ctx, "UPDATE credentials SET provider_account_id=? WHERE resource_id=? AND provider_account_id<>?", account, id, account)
		return mapConstraint(err)
	case KindModel:
		var s modelSpec
		if err := decodeSpec(raw, &s); err != nil {
			return err
		}
		connection, err := resolve(ctx, tx, KindProviderConnection, s.ConnectionRef)
		if err != nil {
			return err
		}
		capabilities, _ := json.Marshal(s.Capabilities)
		if _, err = tx.ExecContext(ctx, `INSERT INTO models VALUES(?,?,?,?,?) ON CONFLICT(resource_id) DO UPDATE SET provider_model_id=excluded.provider_model_id,capabilities_json=excluded.capabilities_json,enabled=excluded.enabled`, id, connection, s.ProviderModelID, string(capabilities), s.Enabled); err != nil {
			return mapConstraint(err)
		}
		_, err = tx.ExecContext(ctx, "UPDATE models SET connection_id=? WHERE resource_id=? AND connection_id<>?", connection, id, connection)
		return mapConstraint(err)
	case KindDestination:
		var s destinationSpec
		if err := decodeSpec(raw, &s); err != nil {
			return err
		}
		model, err := resolve(ctx, tx, KindModel, s.ModelRef)
		if err != nil {
			return err
		}
		credential, err := resolve(ctx, tx, KindCredential, s.CredentialRef)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO destinations VALUES(?,?,?,?,?) ON CONFLICT(resource_id) DO UPDATE SET model_id=excluded.model_id,credential_id=excluded.credential_id,weight=excluded.weight,enabled=excluded.enabled`, id, model, credential, s.Weight, s.Enabled)
		return mapConstraint(err)
	case KindStrategy:
		var s strategySpec
		if err := decodeSpec(raw, &s); err != nil {
			return err
		}
		for _, ref := range s.Destinations {
			if _, err := resolve(ctx, tx, KindDestination, ref); err != nil {
				return err
			}
		}
		return nil
	case KindRoute:
		var s routeSpec
		if err := decodeSpec(raw, &s); err != nil {
			return err
		}
		strategy, err := resolve(ctx, tx, KindStrategy, s.StrategyRef)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO routes(resource_id,model_alias,strategy_id,active_strategy_version_id,enabled) VALUES(?,?,?,NULL,?) ON CONFLICT(resource_id) DO UPDATE SET model_alias=excluded.model_alias,strategy_id=excluded.strategy_id,enabled=excluded.enabled`, id, s.ModelAlias, strategy, s.Enabled)
		return mapConstraint(err)
	case KindAgentToken:
		var s agentTokenSpec
		if err := decodeSpec(raw, &s); err != nil {
			return err
		}
		routeIDs := make([]string, 0, len(s.AllowedRoutes))
		for _, ref := range s.AllowedRoutes {
			route, err := resolve(ctx, tx, KindRoute, ref)
			if err != nil {
				return err
			}
			routeIDs = append(routeIDs, route)
		}
		allowed, _ := json.Marshal(routeIDs)
		var existingRevoked sql.NullString
		err := tx.QueryRowContext(ctx, "SELECT revoked_at FROM agent_tokens WHERE resource_id=?", id).Scan(&existingRevoked)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if s.Enabled && existingRevoked.Valid {
			return &RepositoryError{Code: CodeInvalidResource}
		}
		var revoked *string
		if !s.Enabled && !existingRevoked.Valid {
			value := now.Format(time.RFC3339Nano)
			revoked = &value
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO agent_tokens(resource_id,expires_at,revoked_at,allowed_routes_json) VALUES(?,?,?,?) ON CONFLICT(resource_id) DO UPDATE SET expires_at=excluded.expires_at,revoked_at=CASE WHEN excluded.revoked_at IS NOT NULL THEN excluded.revoked_at ELSE agent_tokens.revoked_at END,allowed_routes_json=excluded.allowed_routes_json`, id, s.ExpiresAt, revoked, string(allowed))
		return mapConstraint(err)
	default:
		return &RepositoryError{Code: CodeInvalidResource}
	}
}

func mapConstraint(err error) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	if strings.Contains(message, "provider_mismatch") {
		return &RepositoryError{Code: CodeProviderMismatch}
	}
	if strings.Contains(message, "_in_use") {
		return &RepositoryError{Code: CodeResourceInUse}
	}
	if isForeignKeyConstraint(err) {
		return &RepositoryError{Code: CodeReferenceMissing}
	}
	if isUniqueConstraint(err) {
		return &RepositoryError{Code: CodeAlreadyExists}
	}
	return err
}
func isUniqueConstraint(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique constraint")
}
func isForeignKeyConstraint(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "foreign key constraint")
}

type auditRecord struct {
	Action                   string
	Kind                     ResourceKind
	ResourceID, Result, Code string
	Version                  int64
}

func insertAudit(ctx context.Context, tx *sql.Tx, now time.Time, actor Actor, record auditRecord) error {
	details := struct {
		Version int64  `json:"version,omitempty"`
		Code    string `json:"code,omitempty"`
	}{record.Version, record.Code}
	encoded, err := json.Marshal(details)
	if err != nil {
		return err
	}
	id, err := newUUID()
	if err != nil {
		return err
	}
	var actorID *string
	if actor.ID != "" {
		actorID = &actor.ID
	}
	var resourceID *string
	if record.ResourceID != "" {
		resourceID = &record.ResourceID
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_events(id,actor_type,actor_id,action,resource_kind,resource_id,result,details_json,occurred_at) VALUES(?,?,?,?,?,?,?,?,?)`, id, actor.Type, actorID, record.Action, record.Kind, resourceID, record.Result, string(encoded), now.UTC().Format(time.RFC3339Nano))
	return err
}

func newUUID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(value[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:], nil
}
