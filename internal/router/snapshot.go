package router

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/sxamx/modelcairn/internal/chatcompletions"
)

const (
	CodeRouteNotFound = "route_not_found"
	CodeRouteDisabled = "route_disabled"
	CodeInvalidGraph  = "invalid_router_graph"
)

type LoadError struct{ Code string }

func (e *LoadError) Error() string { return e.Code }

type Snapshot struct {
	ConfigRevision    int64
	RouteID           string
	Alias             string
	StrategyID        string
	StrategyVersionID string
	StrategyVersion   int
	MaxAttempts       int
	AttemptTimeout    time.Duration
	TotalTimeout      time.Duration
	Destinations      []Destination
}

type Destination struct {
	ID, Name, ModelID, ProviderModelID string
	ConnectionID, BaseURL, Adapter     string
	AllowPrivateNetwork                bool
	CredentialID, SecretID, SecretName string
	EgressID                           string
	EgressType                         string
	Weight                             int
	Capabilities                       []chatcompletions.Capability
	ExclusionReasons                   []string
}

func (d Destination) Eligible() bool { return len(d.ExclusionReasons) == 0 }

type Loader struct {
	db  *sql.DB
	now func() time.Time
}

func NewLoader(db *sql.DB) *Loader { return &Loader{db: db, now: time.Now} }

type strategyDefinition struct {
	Destinations []struct {
		Name string `json:"name"`
	} `json:"destinations"`
	MaxAttempts      int `json:"maxAttempts"`
	AttemptTimeoutMS int `json:"attemptTimeoutMs"`
	TotalTimeoutMS   int `json:"totalTimeoutMs"`
}

func (l *Loader) Load(ctx context.Context, alias string, required []chatcompletions.Capability) (Snapshot, error) {
	tx, err := l.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return Snapshot{}, fmt.Errorf("begin router snapshot: %w", err)
	}
	defer tx.Rollback()

	var snapshot Snapshot
	var routeEnabled bool
	var versionID, definitionText sql.NullString
	var strategyVersion sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT installation_state.config_revision,r.resource_id,r.model_alias,r.strategy_id,
		r.active_strategy_version_id,r.enabled,sv.version,sv.definition_json
		FROM installation_state CROSS JOIN routes r
		LEFT JOIN strategy_versions sv ON sv.id=r.active_strategy_version_id AND sv.strategy_id=r.strategy_id
		WHERE installation_state.singleton=1 AND r.model_alias=?`, alias).Scan(
		&snapshot.ConfigRevision, &snapshot.RouteID, &snapshot.Alias, &snapshot.StrategyID,
		&versionID, &routeEnabled, &strategyVersion, &definitionText)
	if errors.Is(err, sql.ErrNoRows) {
		return Snapshot{}, &LoadError{Code: CodeRouteNotFound}
	}
	if err != nil {
		return Snapshot{}, fmt.Errorf("load route snapshot: %w", err)
	}
	if !routeEnabled {
		return Snapshot{}, &LoadError{Code: CodeRouteDisabled}
	}
	if !versionID.Valid || !strategyVersion.Valid || !definitionText.Valid {
		return Snapshot{}, &LoadError{Code: CodeInvalidGraph}
	}
	snapshot.StrategyVersionID = versionID.String
	snapshot.StrategyVersion = int(strategyVersion.Int64)

	var strategy strategyDefinition
	decoder := json.NewDecoder(bytes.NewReader([]byte(definitionText.String)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&strategy) != nil || len(strategy.Destinations) < 1 || len(strategy.Destinations) > 32 ||
		strategy.MaxAttempts < 1 || strategy.MaxAttempts > 32 || strategy.AttemptTimeoutMS < 100 ||
		strategy.AttemptTimeoutMS > 600000 || strategy.TotalTimeoutMS < 100 || strategy.TotalTimeoutMS > 900000 {
		return Snapshot{}, &LoadError{Code: CodeInvalidGraph}
	}
	snapshot.MaxAttempts = strategy.MaxAttempts
	snapshot.AttemptTimeout = time.Duration(strategy.AttemptTimeoutMS) * time.Millisecond
	snapshot.TotalTimeout = time.Duration(strategy.TotalTimeoutMS) * time.Millisecond
	requiredSet := make(map[chatcompletions.Capability]bool, len(required))
	for _, capability := range required {
		requiredSet[capability] = true
	}
	seen := map[string]bool{}
	snapshotTime := l.now().UTC()
	for _, ref := range strategy.Destinations {
		if ref.Name == "" || seen[ref.Name] {
			return Snapshot{}, &LoadError{Code: CodeInvalidGraph}
		}
		seen[ref.Name] = true
		destination, loadErr := l.loadDestination(ctx, tx, ref.Name, requiredSet, snapshotTime)
		if loadErr != nil {
			return Snapshot{}, loadErr
		}
		snapshot.Destinations = append(snapshot.Destinations, destination)
	}
	if err := tx.Commit(); err != nil {
		return Snapshot{}, fmt.Errorf("commit router snapshot: %w", err)
	}
	return snapshot, nil
}

func (l *Loader) loadDestination(ctx context.Context, tx *sql.Tx, name string, required map[chatcompletions.Capability]bool, snapshotTime time.Time) (Destination, error) {
	var d Destination
	var capabilitiesJSON string
	var destinationEnabled, modelEnabled, connectionEnabled, egressEnabled bool
	var credentialStatus, providerID string
	err := tx.QueryRowContext(ctx, `SELECT dr.id,dr.name,d.weight,d.enabled,
		m.resource_id,m.provider_model_id,m.capabilities_json,m.enabled,
		pc.resource_id,pc.base_url,json_extract(cr.spec_json,'$.adapter'),pc.allow_private_network,pc.enabled,
		c.resource_id,c.secret_id,s.name,c.status,e.resource_id,e.egress_type,e.enabled,pa.provider_id
		FROM resources dr JOIN destinations d ON d.resource_id=dr.id
		JOIN models m ON m.resource_id=d.model_id
		JOIN resources cr ON cr.id=m.connection_id
		JOIN provider_connections pc ON pc.resource_id=m.connection_id
		JOIN credentials c ON c.resource_id=d.credential_id
		JOIN secrets s ON s.id=c.secret_id
		JOIN egresses e ON e.resource_id=c.egress_id
		JOIN provider_accounts pa ON pa.resource_id=c.provider_account_id
		WHERE dr.kind='Destination' AND dr.name=?`, name).Scan(
		&d.ID, &d.Name, &d.Weight, &destinationEnabled,
		&d.ModelID, &d.ProviderModelID, &capabilitiesJSON, &modelEnabled,
		&d.ConnectionID, &d.BaseURL, &d.Adapter, &d.AllowPrivateNetwork, &connectionEnabled,
		&d.CredentialID, &d.SecretID, &d.SecretName, &credentialStatus, &d.EgressID, &d.EgressType, &egressEnabled, &providerID)
	if errors.Is(err, sql.ErrNoRows) {
		return Destination{}, &LoadError{Code: CodeInvalidGraph}
	}
	if err != nil {
		return Destination{}, fmt.Errorf("load destination: %w", err)
	}
	var capabilityNames []string
	if json.Unmarshal([]byte(capabilitiesJSON), &capabilityNames) != nil {
		return Destination{}, &LoadError{Code: CodeInvalidGraph}
	}
	available := map[chatcompletions.Capability]bool{}
	for _, name := range capabilityNames {
		capability := chatcompletions.Capability(name)
		if available[capability] || !knownCapability(capability) {
			return Destination{}, &LoadError{Code: CodeInvalidGraph}
		}
		available[capability] = true
		d.Capabilities = append(d.Capabilities, capability)
	}
	sort.Slice(d.Capabilities, func(i, j int) bool { return d.Capabilities[i] < d.Capabilities[j] })
	if !destinationEnabled {
		d.ExclusionReasons = append(d.ExclusionReasons, "destination_disabled")
	}
	if !modelEnabled {
		d.ExclusionReasons = append(d.ExclusionReasons, "model_disabled")
	}
	if !connectionEnabled {
		d.ExclusionReasons = append(d.ExclusionReasons, "connection_disabled")
	}
	if credentialStatus != "active" {
		d.ExclusionReasons = append(d.ExclusionReasons, "credential_"+credentialStatus)
	}
	if !egressEnabled {
		d.ExclusionReasons = append(d.ExclusionReasons, "egress_disabled")
	}
	for capability := range required {
		if !available[capability] {
			d.ExclusionReasons = append(d.ExclusionReasons, "capability_missing:"+string(capability))
		}
	}
	var cooling bool
	now := snapshotTime.Format(time.RFC3339Nano)
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM cooldowns WHERE ends_at>? AND (
		(scope_kind='destination' AND scope_resource_id=?) OR (scope_kind='model' AND scope_resource_id=?) OR
		(scope_kind='credential' AND scope_resource_id=?) OR (scope_kind='account' AND scope_resource_id=(SELECT provider_account_id FROM credentials WHERE resource_id=?)) OR
		(scope_kind='connection' AND scope_resource_id=?) OR (scope_kind='provider' AND scope_resource_id=?)))`,
		now, d.ID, d.ModelID, d.CredentialID, d.CredentialID, d.ConnectionID, providerID).Scan(&cooling)
	if err != nil {
		return Destination{}, fmt.Errorf("load cooldown: %w", err)
	}
	if cooling {
		d.ExclusionReasons = append(d.ExclusionReasons, "cooldown_active")
	}
	sort.Strings(d.ExclusionReasons)
	return d, nil
}

func knownCapability(capability chatcompletions.Capability) bool {
	switch capability {
	case chatcompletions.CapabilityText, chatcompletions.CapabilityStream,
		chatcompletions.CapabilityTools, chatcompletions.CapabilityParallelTools,
		chatcompletions.CapabilityDeveloperRole, "json-schema", "logprobs":
		return true
	default:
		return false
	}
}
