package config

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/sxamx/modelcairn/internal/storage"
)

type PersistedPlan struct {
	Token         string          `json:"token"`
	ExpiresAt     time.Time       `json:"expiresAt"`
	Changes       []Change        `json:"changes"`
	Configuration json.RawMessage `json:"configuration"`
}

type Manager struct {
	secrets    *storage.SecretStore
	repository *storage.Repository
	clock      func() time.Time
}

func NewManager(installation *storage.Installation) *Manager {
	return &Manager{secrets: installation.Secrets(), repository: storage.NewRepository(installation.DB()), clock: time.Now}
}

func (m *Manager) Plan(ctx context.Context, desired *Document, allowDelete bool) (*PersistedPlan, error) {
	var prepared *Prepared
	issuedAt := m.clock()
	token, err := m.secrets.CreatePlanToken(ctx, issuedAt, func(tx *sql.Tx) (storage.PlanBinding, error) {
		var binding storage.PlanBinding
		var prepareErr error
		prepared, binding, _, prepareErr = m.prepareTx(ctx, tx, desired, allowDelete)
		return binding, prepareErr
	})
	if err != nil {
		return nil, err
	}
	safe, err := CanonicalJSON(prepared.Resolved, m.secrets.Redactor())
	if err != nil {
		return nil, err
	}
	return &PersistedPlan{Token: token, ExpiresAt: time.Unix(issuedAt.Unix()+600, 0).UTC(), Changes: prepared.Changes, Configuration: safe}, nil
}

type ApplyResult struct {
	Changes   []Change  `json:"changes"`
	AppliedAt time.Time `json:"appliedAt"`
}

func (m *Manager) Apply(ctx context.Context, token string, desired *Document, allowDelete bool, actor storage.Actor) error {
	_, err := m.apply(ctx, token, desired, allowDelete, func(*sql.Tx) (storage.Actor, error) { return actor, nil })
	return err
}

// ApplySession derives its audit actor from a live session in the same database
// transaction as plan consumption and the complete graph update.
func (m *Manager) ApplySession(ctx context.Context, token string, desired *Document, allowDelete bool, sessionToken, csrfToken string) (ApplyResult, error) {
	return m.apply(ctx, token, desired, allowDelete, func(tx *sql.Tx) (storage.Actor, error) {
		return storage.AuthorizeAdminMutationTx(ctx, tx, sessionToken, csrfToken)
	})
}

func (m *Manager) apply(ctx context.Context, token string, desired *Document, allowDelete bool, authorize func(*sql.Tx) (storage.Actor, error)) (ApplyResult, error) {
	var mutations []storage.ConfigMutation
	var result ApplyResult
	var actor storage.Actor
	err := m.secrets.ExecutePlan(ctx, token, func(tx *sql.Tx) (storage.PlanBinding, error) {
		var err error
		actor, err = authorize(tx)
		if err != nil {
			return storage.PlanBinding{}, err
		}
		prepared, binding, preparedMutations, err := m.prepareTx(ctx, tx, desired, allowDelete)
		mutations = preparedMutations
		if err == nil {
			result.Changes = append([]Change{}, prepared.Changes...)
		}
		return binding, err
	}, func(tx *sql.Tx) error {
		if err := m.repository.ApplyConfigTx(ctx, tx, mutations, actor); err != nil {
			return err
		}
		result.AppliedAt = m.clock().UTC()
		return nil
	})
	if err != nil {
		return ApplyResult{}, err
	}
	return result, nil
}

// Export returns the complete persisted configuration in deterministic form.
// Secret values are never configuration fields; the shared redactor is also
// applied as a final output boundary.
func (m *Manager) Export(ctx context.Context) ([]byte, error) {
	items, err := m.repository.List(ctx)
	if err != nil {
		return nil, err
	}
	catalog, err := catalogFromStorage(items, nil)
	if err != nil {
		return nil, err
	}
	doc := &Document{APIVersion: APIVersion, Kind: DocumentKind, Resources: catalog.items}
	return CanonicalJSON(doc, m.secrets.Redactor())
}

func (m *Manager) prepareTx(ctx context.Context, tx *sql.Tx, desired *Document, allowDelete bool) (*Prepared, storage.PlanBinding, []storage.ConfigMutation, error) {
	current, err := storage.ListResourcesTx(ctx, tx)
	if err != nil {
		return nil, storage.PlanBinding{}, nil, err
	}
	secrets, err := storage.SecretNamesTx(ctx, tx)
	if err != nil {
		return nil, storage.PlanBinding{}, nil, err
	}
	catalog, err := catalogFromStorage(current, secrets)
	if err != nil {
		return nil, storage.PlanBinding{}, nil, err
	}
	prepared, err := Prepare(desired, catalog, allowDelete)
	if err != nil {
		return nil, storage.PlanBinding{}, nil, err
	}
	for index, resource := range desired.Resources {
		if m.secrets.Redactor().String(resource.Metadata.Name) != resource.Metadata.Name {
			return nil, storage.PlanBinding{}, nil, failure("sensitive_identity", resourcePath(index)+".metadata.name")
		}
	}
	revision, err := storage.ConfigRevisionTx(ctx, tx)
	if err != nil {
		return nil, storage.PlanBinding{}, nil, err
	}
	digest, err := operationDigest(desired, allowDelete)
	if err != nil {
		return nil, storage.PlanBinding{}, nil, err
	}
	index := map[string]storage.Resource{}
	for _, item := range current {
		index[string(item.Kind)+"\x00"+item.Name] = item
	}
	binding := storage.PlanBinding{Revision: revision, Digest: digest, Observed: make([]storage.ObservedResource, 0, len(desired.Resources))}
	actions := map[string]string{}
	for _, change := range prepared.Changes {
		actions[string(change.Kind)+"\x00"+change.Name] = change.Action
	}
	mutations := make([]storage.ConfigMutation, 0, len(desired.Resources))
	for _, resource := range prepared.Resolved.Resources {
		key := string(resource.Kind) + "\x00" + resource.Metadata.Name
		old, exists := index[key]
		observed := storage.ObservedResource{Kind: storage.ResourceKind(resource.Kind), Name: resource.Metadata.Name}
		if exists {
			observed.ID, observed.Version = old.ID, old.ResourceVersion
		}
		binding.Observed = append(binding.Observed, observed)
		action := actions[key]
		if action == "noop" {
			continue
		}
		mutation := storage.ConfigMutation{Action: action, Kind: storage.ResourceKind(resource.Kind), Name: resource.Metadata.Name}
		if action == "delete" {
			mutation.ExpectedVersion = old.ResourceVersion
		} else {
			spec, err := json.Marshal(resource.Spec)
			if err != nil {
				return nil, storage.PlanBinding{}, nil, err
			}
			mutation.Put = storage.PutResource{Kind: storage.ResourceKind(resource.Kind), Name: resource.Metadata.Name, DisplayName: resource.Metadata.DisplayName, Description: resource.Metadata.Description, Spec: spec}
			if exists {
				mutation.Put.ExpectedVersion = old.ResourceVersion
			}
		}
		mutations = append(mutations, mutation)
	}
	return prepared, binding, mutations, nil
}

type persistedCatalog struct {
	items   []Resource
	secrets map[string]bool
}

func (c persistedCatalog) Resources() []Resource         { return c.items }
func (c persistedCatalog) SecretExists(name string) bool { return c.secrets[name] }

func catalogFromStorage(items []storage.Resource, secrets map[string]bool) (persistedCatalog, error) {
	out := persistedCatalog{items: make([]Resource, 0, len(items)), secrets: secrets}
	for _, item := range items {
		metadata := Metadata{Name: item.Name, DisplayName: item.DisplayName, Description: item.Description, UID: &item.ID, ResourceVersion: &item.ResourceVersion}
		raw, err := json.Marshal(map[string]any{"kind": item.Kind, "state": Present, "metadata": metadata, "spec": json.RawMessage(item.Spec)})
		if err != nil {
			return persistedCatalog{}, err
		}
		resource, err := decodeResource(raw, "$")
		if err != nil {
			return persistedCatalog{}, err
		}
		out.items = append(out.items, resource)
	}
	return out, nil
}

func operationDigest(desired *Document, allowDelete bool) (string, error) {
	if desired == nil {
		return "", errors.New("invalid_plan")
	}
	canonical, err := CanonicalJSON(desired, nil)
	if err != nil {
		return "", err
	}
	wrapper, err := json.Marshal(struct {
		Document    json.RawMessage `json:"document"`
		AllowDelete bool            `json:"allowDelete"`
	}{json.RawMessage(canonical), allowDelete})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(wrapper)
	return base64.RawURLEncoding.EncodeToString(digest[:]), nil
}
