package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

type ConfigMutation struct {
	Action          string
	Put             PutResource
	Kind            ResourceKind
	Name            string
	ExpectedVersion int64
}

func ConfigRevisionTx(ctx context.Context, tx *sql.Tx) (int64, error) {
	var revision int64
	if err := tx.QueryRowContext(ctx, "SELECT config_revision FROM installation_state WHERE singleton=1").Scan(&revision); err != nil {
		return 0, fmt.Errorf("read configuration revision: %w", err)
	}
	return revision, nil
}

func ListResourcesTx(ctx context.Context, tx *sql.Tx) ([]Resource, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id,kind,name,display_name,description,resource_version,spec_json,created_at,updated_at FROM resources ORDER BY kind,name`)
	if err != nil {
		return nil, fmt.Errorf("list resources in plan: %w", err)
	}
	defer rows.Close()
	var items []Resource
	for rows.Next() {
		item, err := scanResource(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate resources in plan: %w", err)
	}
	return items, nil
}

func SecretNamesTx(ctx context.Context, tx *sql.Tx) (map[string]bool, error) {
	rows, err := tx.QueryContext(ctx, "SELECT name FROM secrets ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("list secret names in plan: %w", err)
	}
	defer rows.Close()
	result := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		result[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// ApplyConfigTx applies a validated graph in one transaction. Deletions are
// ordered from dependents to dependencies, followed by creates and updates.
func (r *Repository) ApplyConfigTx(ctx context.Context, tx *sql.Tx, mutations []ConfigMutation, actor Actor) error {
	if err := validateActor(actor); err != nil {
		return err
	}
	if len(mutations) == 0 {
		return nil
	}
	ordered := append([]ConfigMutation{}, mutations...)
	sort.SliceStable(ordered, func(i, j int) bool { return mutationRank(ordered[i]) < mutationRank(ordered[j]) })
	for _, mutation := range ordered {
		switch mutation.Action {
		case "noop":
			continue
		case "create", "update":
			if mutation.Put.Kind != mutation.Kind || mutation.Put.Name != mutation.Name ||
				(mutation.Action == "create" && mutation.Put.ExpectedVersion != 0) ||
				(mutation.Action == "update" && mutation.Put.ExpectedVersion < 1) {
				return &RepositoryError{Code: CodeInvalidResource}
			}
			if err := validatePut(mutation.Put); err != nil {
				return err
			}
			item, err := r.putEnvelopeTx(ctx, tx, mutation.Put)
			if err != nil {
				return err
			}
			if err := upsertTyped(ctx, tx, item.ID, item.Kind, item.Spec, r.now().UTC()); err != nil {
				return err
			}
			if err := insertAudit(ctx, tx, r.now(), actor, auditRecord{Action: actionFor(mutation.Put.ExpectedVersion), Kind: item.Kind, ResourceID: item.ID, Result: "success", Version: item.ResourceVersion}); err != nil {
				return err
			}
		case "delete":
			if _, ok := validKinds[mutation.Kind]; !ok || mutation.Name == "" || mutation.ExpectedVersion < 1 {
				return &RepositoryError{Code: CodeInvalidResource}
			}
			var id string
			var version int64
			err := tx.QueryRowContext(ctx, "SELECT id,resource_version FROM resources WHERE kind=? AND name=?", mutation.Kind, mutation.Name).Scan(&id, &version)
			if errors.Is(err, sql.ErrNoRows) {
				return &RepositoryError{Code: CodeVersionConflict}
			}
			if err != nil {
				return err
			}
			if version != mutation.ExpectedVersion {
				return &RepositoryError{Code: CodeVersionConflict}
			}
			if mutation.Kind == KindStrategy {
				if _, err := tx.ExecContext(ctx, "DELETE FROM strategy_versions WHERE strategy_id=?", id); err != nil {
					return mapConstraint(err)
				}
			}
			result, err := tx.ExecContext(ctx, "DELETE FROM resources WHERE id=? AND resource_version=?", id, version)
			if err != nil {
				return mapConstraint(err)
			}
			if n, _ := result.RowsAffected(); n != 1 {
				return &RepositoryError{Code: CodeVersionConflict}
			}
			if err := insertAudit(ctx, tx, r.now(), actor, auditRecord{Action: "resource.delete", Kind: mutation.Kind, ResourceID: id, Result: "success", Version: version}); err != nil {
				return err
			}
		default:
			return &RepositoryError{Code: CodeInvalidResource}
		}
	}
	return bumpConfigRevisionTx(ctx, tx, r.now())
}

func bumpConfigRevisionTx(ctx context.Context, tx *sql.Tx, now time.Time) error {
	result, err := tx.ExecContext(ctx, "UPDATE installation_state SET config_revision=config_revision+1,updated_at=? WHERE singleton=1", now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return errors.New("installation_state_missing")
	}
	return nil
}

func mutationRank(m ConfigMutation) int {
	if m.Action == "delete" {
		return 100 - kindRank(m.Kind)
	}
	return kindRank(m.Put.Kind)
}
func kindRank(k ResourceKind) int {
	switch k {
	case KindProvider:
		return 1
	case KindEgress:
		return 2
	case KindProviderAccount, KindProviderConnection:
		return 3
	case KindCredential, KindModel:
		return 4
	case KindDestination:
		return 5
	case KindStrategy:
		return 6
	case KindRoute:
		return 7
	case KindAgentToken:
		return 8
	}
	return 50
}

func (r *Repository) putEnvelopeTx(ctx context.Context, tx *sql.Tx, input PutResource) (Resource, error) {
	now := r.now().UTC()
	stamp := now.Format(time.RFC3339Nano)
	if input.ExpectedVersion == 0 {
		id, err := newUUID()
		if err != nil {
			return Resource{}, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO resources(id,kind,name,display_name,description,resource_version,spec_json,created_at,updated_at) VALUES(?,?,?,?,?,1,?,?,?)`, id, input.Kind, input.Name, input.DisplayName, input.Description, string(input.Spec), stamp, stamp)
		if err != nil {
			if isUniqueConstraint(err) {
				return Resource{}, &RepositoryError{Code: CodeAlreadyExists}
			}
			return Resource{}, err
		}
		return Resource{ID: id, Kind: input.Kind, Name: input.Name, DisplayName: input.DisplayName, Description: input.Description, ResourceVersion: 1, Spec: append(json.RawMessage(nil), input.Spec...), CreatedAt: now, UpdatedAt: now}, nil
	}
	var id, created string
	var version int64
	if err := tx.QueryRowContext(ctx, "SELECT id,resource_version,created_at FROM resources WHERE kind=? AND name=?", input.Kind, input.Name).Scan(&id, &version, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Resource{}, &RepositoryError{Code: CodeVersionConflict}
		}
		return Resource{}, err
	}
	if version != input.ExpectedVersion {
		return Resource{}, &RepositoryError{Code: CodeVersionConflict}
	}
	result, err := tx.ExecContext(ctx, `UPDATE resources SET display_name=?,description=?,resource_version=resource_version+1,spec_json=?,updated_at=? WHERE id=? AND resource_version=?`, input.DisplayName, input.Description, string(input.Spec), stamp, id, version)
	if err != nil {
		return Resource{}, err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return Resource{}, &RepositoryError{Code: CodeVersionConflict}
	}
	createdAt, err := time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return Resource{}, err
	}
	return Resource{ID: id, Kind: input.Kind, Name: input.Name, DisplayName: input.DisplayName, Description: input.Description, ResourceVersion: version + 1, Spec: append(json.RawMessage(nil), input.Spec...), CreatedAt: createdAt, UpdatedAt: now}, nil
}
