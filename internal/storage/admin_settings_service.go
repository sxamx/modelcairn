package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/sxamx/modelcairn/internal/adminsettings"
)

// AdminSettingsService is an internal application service. Its caller must
// authenticate/authorize first. The effective snapshot is captured at startup.
type AdminSettingsService struct {
	installation *Installation
	effective    AdminSettingsRecord
}

type AdminSettingsState struct {
	Desired         AdminSettingsRecord
	Effective       AdminSettingsRecord
	RestartRequired bool
}

type AdminSettingsPlan struct {
	Desired       AdminSettingsRecord
	ChangedFields []string
	Token         string
	ExpiresAt     time.Time
}

type AdminSettingsApplyResult struct {
	State     AdminSettingsState
	AppliedAt time.Time
}

func NewAdminSettingsService(i *Installation) (*AdminSettingsService, error) {
	if i == nil {
		return nil, errors.New("installation_required")
	}
	current, err := ReadAdminSettings(context.Background(), i.DB())
	if err != nil {
		return nil, err
	}
	return &AdminSettingsService{installation: i, effective: current}, nil
}

func (s *AdminSettingsService) State(ctx context.Context) (AdminSettingsState, error) {
	current, err := ReadAdminSettings(ctx, s.installation.DB())
	if err != nil {
		return AdminSettingsState{}, err
	}
	return s.state(current), nil
}

func (s *AdminSettingsService) state(desired AdminSettingsRecord) AdminSettingsState {
	effective := s.effective
	effective.Spec.TrustedProxyCIDRs = append([]string{}, effective.Spec.TrustedProxyCIDRs...)
	a, _ := adminsettings.CanonicalJSON(desired.Spec)
	b, _ := adminsettings.CanonicalJSON(effective.Spec)
	return AdminSettingsState{Desired: desired, Effective: effective, RestartRequired: !bytes.Equal(a, b)}
}

func (s *AdminSettingsService) Plan(ctx context.Context, input []byte) (AdminSettingsPlan, error) {
	doc, err := adminsettings.Parse(input, adminsettings.Update)
	if err != nil {
		return AdminSettingsPlan{}, err
	}
	var result AdminSettingsPlan
	issued, err := s.installation.Secrets().CreateSettingsPlan(ctx, func(tx *sql.Tx) (PlanBinding, error) {
		current, desired, binding, err := settingsSnapshot(ctx, tx, doc)
		if err != nil {
			return PlanBinding{}, err
		}
		result.Desired = current
		result.Desired.Spec = desired
		result.ChangedFields = settingsChangedFields(current.Spec, desired)
		return binding, nil
	})
	if err != nil {
		return AdminSettingsPlan{}, err
	}
	result.Token, result.ExpiresAt = issued.Token, issued.ExpiresAt
	return result, nil
}

func (s *AdminSettingsService) Apply(ctx context.Context, input []byte, token string, actor Actor) (AdminSettingsApplyResult, error) {
	if err := validateActor(actor); err != nil {
		return AdminSettingsApplyResult{}, err
	}
	doc, err := adminsettings.Parse(input, adminsettings.Update)
	if err != nil {
		return AdminSettingsApplyResult{}, err
	}
	var desired adminsettings.Resolved
	var result AdminSettingsApplyResult
	err = s.installation.Secrets().ExecuteSettingsPlan(ctx, token, func(tx *sql.Tx) (PlanBinding, error) {
		_, resolved, binding, err := settingsSnapshot(ctx, tx, doc)
		desired = resolved
		return binding, err
	}, func(tx *sql.Tx) error {
		at := time.Now().UTC()
		record, err := UpdateAdminSettingsTx(ctx, tx, *doc.ResourceVersion, desired, actor, at)
		if err != nil {
			return err
		}
		result = AdminSettingsApplyResult{State: s.state(record), AppliedAt: at}
		return nil
	})
	if err != nil {
		return AdminSettingsApplyResult{}, err
	}
	return result, nil
}

func settingsSnapshot(ctx context.Context, tx *sql.Tx, doc *adminsettings.Document) (AdminSettingsRecord, adminsettings.Resolved, PlanBinding, error) {
	current, err := ReadAdminSettingsTx(ctx, tx)
	if err != nil {
		return current, adminsettings.Resolved{}, PlanBinding{}, err
	}
	if doc.ResourceVersion == nil || *doc.ResourceVersion != current.ResourceVersion {
		return current, adminsettings.Resolved{}, PlanBinding{}, &RepositoryError{Code: CodeVersionConflict}
	}
	desired, err := adminsettings.ResolveUpdate(doc, current.Spec)
	if err != nil {
		return current, desired, PlanBinding{}, err
	}
	canonical, err := adminsettings.CanonicalJSON(desired)
	if err != nil {
		return current, desired, PlanBinding{}, err
	}
	digest := sha256.Sum256(canonical)
	return current, desired, PlanBinding{Revision: current.ResourceVersion, Digest: base64.RawURLEncoding.EncodeToString(digest[:])}, nil
}

func settingsChangedFields(before, after adminsettings.Resolved) []string {
	a, _ := adminsettings.CanonicalJSON(before)
	b, _ := adminsettings.CanonicalJSON(after)
	var oldFields, newFields map[string]json.RawMessage
	_ = json.Unmarshal(a, &oldFields)
	_ = json.Unmarshal(b, &newFields)
	changed := []string{}
	for key, value := range newFields {
		if !bytes.Equal(oldFields[key], value) {
			changed = append(changed, key)
		}
	}
	sort.Strings(changed)
	return changed
}
