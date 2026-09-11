package config

import (
	"context"
	"database/sql"

	"github.com/sxamx/modelcairn/internal/storage"
)

type ResourceMutation string

const (
	CreateResource ResourceMutation = "create"
	UpdateResource ResourceMutation = "update"
	DeleteResource ResourceMutation = "delete"
)

type ResourceMutationInput struct {
	Operation       ResourceMutation
	Kind            Kind
	Name            string
	Resource        *Resource
	ExpectedVersion int64
	SessionToken    string
	CSRFToken       string
}

type ResourceMutationResult struct {
	Resource storage.Resource
	Noop     bool
}

// MutateResourceSession validates an individual mutation against the complete
// stored graph and persists it with session authorization and audit atomically.
func (m *Manager) MutateResourceSession(ctx context.Context, input ResourceMutationInput) (ResourceMutationResult, error) {
	var result ResourceMutationResult
	err := m.secrets.ExecuteAdminMutation(ctx, input.SessionToken, input.CSRFToken, func(tx *sql.Tx, actor storage.Actor) error {
		kind := storage.ResourceKind(input.Kind)
		current, getErr := storage.GetResourceTx(ctx, tx, kind, input.Name)
		exists := getErr == nil
		if getErr != nil && !storage.IsRepositoryCode(getErr, storage.CodeNotFound) {
			return getErr
		}
		switch input.Operation {
		case CreateResource:
			if exists {
				return &storage.RepositoryError{Code: storage.CodeAlreadyExists}
			}
			if input.ExpectedVersion != 0 || input.Resource == nil {
				return &storage.RepositoryError{Code: storage.CodeInvalidResource}
			}
		case UpdateResource, DeleteResource:
			if !exists {
				return &storage.RepositoryError{Code: storage.CodeNotFound}
			}
			if input.ExpectedVersion < 1 || current.ResourceVersion != input.ExpectedVersion {
				return &storage.RepositoryError{Code: storage.CodeVersionConflict}
			}
			if input.Operation == UpdateResource && input.Resource == nil {
				return &storage.RepositoryError{Code: storage.CodeInvalidResource}
			}
		default:
			return &storage.RepositoryError{Code: storage.CodeInvalidResource}
		}

		var resource Resource
		if input.Operation == DeleteResource {
			resource = Resource{Kind: input.Kind, State: Absent, Metadata: Metadata{Name: input.Name}}
		} else {
			resource = *input.Resource
			if resource.State != Present || resource.Kind != input.Kind || resource.Metadata.Name != input.Name {
				return &storage.RepositoryError{Code: storage.CodeInvalidResource}
			}
			if input.Operation == UpdateResource {
				if resource.Metadata.UID != nil && *resource.Metadata.UID != current.ID {
					return &storage.RepositoryError{Code: storage.CodeVersionConflict}
				}
				if resource.Metadata.ResourceVersion != nil && *resource.Metadata.ResourceVersion != current.ResourceVersion {
					return &storage.RepositoryError{Code: storage.CodeVersionConflict}
				}
			}
		}
		doc := &Document{APIVersion: APIVersion, Kind: DocumentKind, Resources: []Resource{resource}}
		prepared, _, mutations, err := m.prepareTx(ctx, tx, doc, input.Operation == DeleteResource)
		if err != nil {
			return err
		}
		if input.Operation == UpdateResource && len(prepared.Changes) == 1 && prepared.Changes[0].Action == "noop" {
			if err := storage.AuditResourceNoopTx(ctx, tx, current, actor); err != nil {
				return err
			}
			result = ResourceMutationResult{Resource: current, Noop: true}
			return nil
		}
		if err := m.repository.ApplyConfigTx(ctx, tx, mutations, actor); err != nil {
			return err
		}
		if input.Operation != DeleteResource {
			committed, err := storage.GetResourceTx(ctx, tx, kind, input.Name)
			if err != nil {
				return err
			}
			result.Resource = committed
		}
		return nil
	})
	if err != nil {
		return ResourceMutationResult{}, err
	}
	return result, nil
}
