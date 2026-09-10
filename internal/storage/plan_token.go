package storage

import (
	"bytes"
	"context"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

const planPurpose = "modelcairn/plan-token/v1"
const settingsPlanPurpose = "modelcairn/admin-settings-plan/v1"
const maxPlanTokenBytes = 8 << 20

type IssuedSettingsPlan struct {
	Token     string
	ExpiresAt time.Time
}

// CreateSettingsPlan returns issuer-owned expiry metadata, so application
// services never have to parse opaque tokens to build their response.
func (s *SecretStore) CreateSettingsPlan(ctx context.Context, snapshot func(*sql.Tx) (PlanBinding, error)) (IssuedSettingsPlan, error) {
	now := time.Now()
	token, err := s.CreateSettingsPlanToken(ctx, now, snapshot)
	if err != nil {
		return IssuedSettingsPlan{}, err
	}
	return IssuedSettingsPlan{Token: token, ExpiresAt: time.Unix(now.Unix()+600, 0).UTC()}, nil
}

var ErrInvalidPlan = errors.New("invalid_plan")
var ErrPlanExpired = errors.New("plan_expired")

// ObservedResource binds both existing identities and observed absences. An
// absent resource has an empty ID and version zero.
type ObservedResource struct {
	Kind    ResourceKind `json:"kind"`
	Name    string       `json:"name"`
	ID      string       `json:"id"`
	Version int64        `json:"version"`
}

// PlanBinding describes the snapshot and requested operation. Digest must cover
// the desired configuration AND operation options such as permission to delete.
type PlanBinding struct {
	Revision int64              `json:"revision"`
	Digest   string             `json:"digest"`
	Observed []ObservedResource `json:"observed"`
}

type planClaims struct {
	Version        int         `json:"version"`
	Purpose        string      `json:"purpose"`
	InstallationID string      `json:"installationId"`
	KeyVersion     int64       `json:"keyVersion"`
	Binding        PlanBinding `json:"binding"`
	Nonce          string      `json:"nonce"`
	IssuedAt       int64       `json:"issuedAt"`
	ExpiresAt      int64       `json:"expiresAt"`
}

// CreatePlanToken reads and signs one consistent database snapshot while key
// rotation is excluded. The callback must use only the supplied transaction.
func (s *SecretStore) CreatePlanToken(ctx context.Context, now time.Time, snapshot func(*sql.Tx) (PlanBinding, error)) (string, error) {
	return s.createPlanTokenForPurpose(ctx, planPurpose, now, snapshot)
}

// CreateSettingsPlanToken signs an administrative settings snapshot using a
// separate derived key. Callers bind the settings revision and resolved digest.
func (s *SecretStore) CreateSettingsPlanToken(ctx context.Context, now time.Time, snapshot func(*sql.Tx) (PlanBinding, error)) (string, error) {
	return s.createPlanTokenForPurpose(ctx, settingsPlanPurpose, now, snapshot)
}

func (s *SecretStore) createPlanTokenForPurpose(ctx context.Context, purpose string, now time.Time, snapshot func(*sql.Tx) (PlanBinding, error)) (string, error) {
	if snapshot == nil {
		return "", ErrInvalidPlan
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.unavailable {
		return "", errKeyMaterialUnavailable
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	binding, err := snapshot(tx)
	if err != nil {
		return "", err
	}
	return s.issuePlanTokenForPurposeLocked(purpose, binding, now)
}

// IssuePlanToken signs a bounded, purpose-specific plan without exposing key
// material. The caller must obtain Binding from a consistent database snapshot.
func (s *SecretStore) IssuePlanToken(binding PlanBinding, now time.Time) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.unavailable {
		return "", errKeyMaterialUnavailable
	}
	return s.issuePlanTokenLocked(binding, now)
}

func (s *SecretStore) issuePlanTokenLocked(binding PlanBinding, now time.Time) (string, error) {
	return s.issuePlanTokenForPurposeLocked(planPurpose, binding, now)
}

func (s *SecretStore) issuePlanTokenForPurposeLocked(purpose string, binding PlanBinding, now time.Time) (string, error) {
	if !validPlanPurpose(purpose) {
		return "", ErrInvalidPlan
	}
	binding, err := canonicalBinding(binding)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return "", errors.New("plan_randomness_unavailable")
	}
	claims := planClaims{1, purpose, s.installationID, s.activeKeyVersion, binding,
		base64.RawURLEncoding.EncodeToString(nonce), now.Unix(), now.Add(10 * time.Minute).Unix()}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", ErrInvalidPlan
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	if len(encoded)+44 > maxPlanTokenBytes {
		return "", ErrInvalidPlan
	}
	mac, err := s.planMACForPurpose(purpose, encoded)
	if err != nil {
		return "", err
	}
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac), nil
}

// verifyPlanTokenLocked authenticates before decoding claims. Its caller holds
// s.mu through nonce consumption and transaction commit, preventing rotation
// between verification and application. This function alone does not authorize
// execution: the nonce must be consumed atomically with the configuration.
func (s *SecretStore) verifyPlanTokenLocked(token string, binding PlanBinding, now time.Time) (planClaims, error) {
	return s.verifyPlanTokenForPurposeLocked(planPurpose, token, binding, now)
}

func (s *SecretStore) verifyPlanTokenForPurposeLocked(purpose, token string, binding PlanBinding, now time.Time) (planClaims, error) {
	claims, err := s.authenticatePlanTokenForPurposeLocked(purpose, token, now)
	if err != nil {
		return planClaims{}, err
	}
	return verifyPlanBinding(claims, binding)
}

// Authentication alone never authorizes mutation; binding and nonce checks follow.
func (s *SecretStore) authenticatePlanTokenForPurposeLocked(purpose, token string, now time.Time) (planClaims, error) {
	var claims planClaims
	if !validPlanPurpose(purpose) {
		return claims, ErrInvalidPlan
	}
	if s.unavailable {
		return claims, errKeyMaterialUnavailable
	}
	if len(token) > maxPlanTokenBytes {
		return claims, ErrInvalidPlan
	}
	encoded, signature, ok := strings.Cut(token, ".")
	if !ok || len(signature) != 43 {
		return claims, ErrInvalidPlan
	}
	mac, err := s.planMACForPurpose(purpose, encoded)
	if err != nil {
		return claims, err
	}
	provided, err := base64.RawURLEncoding.Strict().DecodeString(signature)
	if err != nil || !hmac.Equal(mac, provided) {
		return claims, ErrInvalidPlan
	}
	payload, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil || json.Unmarshal(payload, &claims) != nil {
		return planClaims{}, ErrInvalidPlan
	}
	canonical, err := json.Marshal(claims)
	if err != nil || !bytes.Equal(payload, canonical) {
		return planClaims{}, ErrInvalidPlan
	}
	if claims.Version != 1 || claims.Purpose != purpose || claims.InstallationID != s.installationID || claims.KeyVersion != s.activeKeyVersion {
		return planClaims{}, ErrInvalidPlan
	}
	nonce, err := base64.RawURLEncoding.Strict().DecodeString(claims.Nonce)
	if err != nil || len(nonce) != 32 {
		return planClaims{}, ErrInvalidPlan
	}
	// Bound arithmetic by comparing against the current clock before computing
	// expiry, so hostile signed timestamps cannot overflow duration arithmetic.
	if claims.IssuedAt > now.Add(30*time.Second).Unix() || claims.IssuedAt < 0 || claims.ExpiresAt <= claims.IssuedAt || claims.ExpiresAt-claims.IssuedAt != 600 {
		return planClaims{}, ErrInvalidPlan
	}
	if now.Unix() >= claims.ExpiresAt {
		return planClaims{}, ErrPlanExpired
	}
	return claims, nil
}

func verifyPlanBinding(claims planClaims, binding PlanBinding) (planClaims, error) {
	binding, err := canonicalBinding(binding)
	if err != nil {
		return planClaims{}, err
	}
	actual, err := canonicalBinding(claims.Binding)
	if err != nil {
		return planClaims{}, err
	}
	if actual.Digest != binding.Digest {
		return claims, ErrInvalidPlan
	}
	want, _ := json.Marshal(binding)
	got, _ := json.Marshal(actual)
	if !bytes.Equal(want, got) {
		return claims, &RepositoryError{Code: CodeVersionConflict}
	}
	return claims, nil
}

func canonicalBinding(binding PlanBinding) (PlanBinding, error) {
	digest, err := base64.RawURLEncoding.Strict().DecodeString(binding.Digest)
	if err != nil || len(digest) != sha256.Size || binding.Revision < 1 {
		return PlanBinding{}, ErrInvalidPlan
	}
	binding.Observed = append([]ObservedResource{}, binding.Observed...)
	sort.Slice(binding.Observed, func(i, j int) bool {
		if binding.Observed[i].Kind == binding.Observed[j].Kind {
			return binding.Observed[i].Name < binding.Observed[j].Name
		}
		return binding.Observed[i].Kind < binding.Observed[j].Kind
	})
	for i, item := range binding.Observed {
		if _, ok := validKinds[item.Kind]; !ok {
			return PlanBinding{}, ErrInvalidPlan
		}
		if !secretNamePattern.MatchString(item.Name) || item.Version < 0 || (item.ID == "") != (item.Version == 0) || len(item.ID) > 128 {
			return PlanBinding{}, ErrInvalidPlan
		}
		if i > 0 && item.Kind == binding.Observed[i-1].Kind && item.Name == binding.Observed[i-1].Name {
			return PlanBinding{}, ErrInvalidPlan
		}
	}
	return binding, nil
}

// Caller holds the store lock; the derived key is never retained or returned.
func (s *SecretStore) planMAC(encoded string) ([]byte, error) {
	return s.planMACForPurpose(planPurpose, encoded)
}

func validPlanPurpose(purpose string) bool {
	return purpose == planPurpose || purpose == settingsPlanPurpose
}

// The expected purpose comes from the operation, never from unverified claims.
func (s *SecretStore) planMACForPurpose(purpose, encoded string) ([]byte, error) {
	if !validPlanPurpose(purpose) {
		return nil, ErrInvalidPlan
	}
	key, err := hkdf.Key(sha256.New, s.activeKey, []byte(s.installationID), purpose, 32)
	if err != nil {
		return nil, ErrInvalidPlan
	}
	defer clear(key)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(encoded))
	return mac.Sum(nil), nil
}
