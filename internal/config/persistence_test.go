package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/sxamx/modelcairn/internal/storage"
)

func managerForTest(t *testing.T) (*Manager, *storage.Installation) {
	t.Helper()
	installation, err := storage.OpenInstallation(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = installation.Close() })
	return NewManager(installation), installation
}

func exampleDocument(t *testing.T) *Document {
	t.Helper()
	raw, err := os.ReadFile("../../docs/contratos/config/example-v1alpha1.yaml")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestManagerPlansAndAppliesCompleteGraph(t *testing.T) {
	ctx := context.Background()
	manager, installation := managerForTest(t)
	if _, err := installation.Secrets().Put(ctx, storage.PutSecret{Name: "example-key-secret", Value: []byte("test-secret-value")}, storage.Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	doc := exampleDocument(t)
	plan, err := manager.Plan(ctx, doc, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Changes) != 10 {
		t.Fatalf("changes=%d", len(plan.Changes))
	}
	for _, change := range plan.Changes {
		if change.Action != "create" {
			t.Fatalf("unexpected action %+v", change)
		}
	}
	if err := manager.Apply(ctx, plan.Token, doc, false, storage.Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	items, err := storage.NewRepository(installation.DB()).List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 10 {
		t.Fatalf("resources=%d", len(items))
	}
	var revision int
	if err := installation.DB().QueryRowContext(ctx, "SELECT config_revision FROM installation_state").Scan(&revision); err != nil {
		t.Fatal(err)
	}
	if revision != 2 {
		t.Fatalf("revision=%d", revision)
	}
	exported, err := manager.Export(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(exported, []byte("test-secret-value")) {
		t.Fatal("export disclosed secret")
	}
	roundTrip, err := Parse(exported)
	if err != nil {
		t.Fatal(err)
	}
	if len(roundTrip.Resources) != 10 {
		t.Fatalf("exported resources=%d", len(roundTrip.Resources))
	}
	noop, err := manager.Plan(ctx, doc, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range noop.Changes {
		if change.Action != "noop" {
			t.Fatalf("non-noop %+v", change)
		}
	}
	if err := manager.Apply(ctx, noop.Token, doc, false, storage.Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	if err := installation.DB().QueryRowContext(ctx, "SELECT config_revision FROM installation_state").Scan(&revision); err != nil {
		t.Fatal(err)
	}
	if revision != 2 {
		t.Fatalf("noop changed revision=%d", revision)
	}
	if err := manager.Apply(ctx, noop.Token, doc, false, storage.Actor{Type: "cli"}); !errors.Is(err, storage.ErrPlanAlreadyUsed) {
		t.Fatalf("reused noop=%v", err)
	}
}

func TestManagerDeletesCompleteGraphInDependencyOrder(t *testing.T) {
	ctx := context.Background()
	manager, installation := managerForTest(t)
	if _, err := installation.Secrets().Put(ctx, storage.PutSecret{Name: "example-key-secret", Value: []byte("test-secret-value")}, storage.Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	initial := exampleDocument(t)
	plan, err := manager.Plan(ctx, initial, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Apply(ctx, plan.Token, initial, false, storage.Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	deletions := &Document{APIVersion: APIVersion, Kind: DocumentKind, Resources: make([]Resource, 0, len(initial.Resources))}
	for i := len(initial.Resources) - 1; i >= 0; i-- {
		r := initial.Resources[i]
		deletions.Resources = append(deletions.Resources, Resource{Kind: r.Kind, State: Absent, Metadata: Metadata{Name: r.Metadata.Name}})
	}
	plan, err = manager.Plan(ctx, deletions, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Apply(ctx, plan.Token, deletions, true, storage.Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	items, err := storage.NewRepository(installation.DB()).List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("resources remain=%d", len(items))
	}
}

func TestManagerMovesReferenceBeforeDeletingDependency(t *testing.T) {
	ctx := context.Background()
	manager, installation := managerForTest(t)
	if _, err := installation.Secrets().Put(ctx, storage.PutSecret{Name: "example-key-secret", Value: []byte("test-secret-value")}, storage.Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	initial := exampleDocument(t)
	plan, err := manager.Plan(ctx, initial, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Apply(ctx, plan.Token, initial, false, storage.Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	change := mustParse(t, basePrefix+`resources:
- {kind: Model, metadata: {name: replacement-chat}, spec: {connectionRef: {name: example-api}, providerModelId: replacement-model, capabilities: [text]}}
- {kind: Destination, metadata: {name: example-primary}, spec: {modelRef: {name: replacement-chat}, credentialRef: {name: example-key}}}
- {kind: Model, state: absent, metadata: {name: example-chat}}`)
	plan, err = manager.Plan(ctx, change, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Apply(ctx, plan.Token, change, true, storage.Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	var modelName string
	if err := installation.DB().QueryRowContext(ctx, `SELECT r.name FROM destinations d JOIN resources r ON r.id=d.model_id`).Scan(&modelName); err != nil {
		t.Fatal(err)
	}
	if modelName != "replacement-chat" {
		t.Fatalf("destination model=%q", modelName)
	}
}

func TestManagerRejectsStaleAndOptionChangedPlans(t *testing.T) {
	ctx := context.Background()
	manager, installation := managerForTest(t)
	base := mustParse(t, basePrefix+`resources:
- {kind: Provider, metadata: {name: provider}, spec: {}}`)
	first, err := manager.Plan(ctx, base, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Apply(ctx, first.Token, base, true, storage.Actor{Type: "cli"}); !errors.Is(err, storage.ErrInvalidPlan) {
		t.Fatalf("changed option=%v", err)
	}
	if err := manager.Apply(ctx, first.Token, base, false, storage.Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	update := mustParse(t, basePrefix+`resources:
- {kind: Provider, metadata: {name: provider, description: changed}, spec: {}}`)
	a, err := manager.Plan(ctx, update, false)
	if err != nil {
		t.Fatal(err)
	}
	b, err := manager.Plan(ctx, update, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Apply(ctx, a.Token, update, false, storage.Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	if err := manager.Apply(ctx, b.Token, update, false, storage.Actor{Type: "cli"}); !storage.IsRepositoryCode(err, storage.CodeVersionConflict) {
		t.Fatalf("stale plan=%v", err)
	}
	current, err := manager.Plan(ctx, update, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.NewRepository(installation.DB()).Put(ctx, storage.PutResource{Kind: storage.KindProvider, Name: "unrelated", Spec: []byte(`{}`)}, storage.Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	if err := manager.Apply(ctx, current.Token, update, false, storage.Actor{Type: "cli"}); !storage.IsRepositoryCode(err, storage.CodeVersionConflict) {
		t.Fatalf("standalone mutation did not invalidate plan: %v", err)
	}
}

func TestBatchRejectsUnsafeIntermediateRelationWithoutPartialWrite(t *testing.T) {
	ctx := context.Background()
	manager, installation := managerForTest(t)
	if _, err := installation.Secrets().Put(ctx, storage.PutSecret{Name: "primary-secret", Value: []byte("test-secret-value")}, storage.Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	initial := mustParse(t, basePrefix+`resources:
- {kind: Provider, metadata: {name: one}, spec: {}}
- {kind: Provider, metadata: {name: two}, spec: {}}
- {kind: ProviderAccount, metadata: {name: account}, spec: {providerRef: {name: one}}}
- {kind: ProviderConnection, metadata: {name: connection}, spec: {providerRef: {name: one}, baseUrl: https://one.invalid/v1, adapter: openai-chat-v1}}
- {kind: Egress, metadata: {name: direct}, spec: {type: direct}}
- {kind: Credential, metadata: {name: credential}, spec: {providerAccountRef: {name: account}, egressRef: {name: direct}, secretRef: {name: primary-secret}}}
- {kind: Model, metadata: {name: model}, spec: {connectionRef: {name: connection}, providerModelId: model, capabilities: [text]}}
- {kind: Destination, metadata: {name: destination}, spec: {modelRef: {name: model}, credentialRef: {name: credential}}}`)
	plan, err := manager.Plan(ctx, initial, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Apply(ctx, plan.Token, initial, false, storage.Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	transition := mustParse(t, basePrefix+`resources:
- {kind: ProviderAccount, metadata: {name: account}, spec: {providerRef: {name: two}}}
- {kind: ProviderConnection, metadata: {name: connection}, spec: {providerRef: {name: two}, baseUrl: https://two.invalid/v1, adapter: openai-chat-v1}}`)
	plan, err = manager.Plan(ctx, transition, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Apply(ctx, plan.Token, transition, false, storage.Actor{Type: "cli"}); !storage.IsRepositoryCode(err, storage.CodeResourceInUse) {
		t.Fatalf("transition error=%v", err)
	}
	var count int
	if err := installation.DB().QueryRowContext(ctx, "SELECT count(*) FROM destinations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("destinations=%d", count)
	}
	var provider string
	if err := installation.DB().QueryRowContext(ctx, `SELECT p.name FROM provider_accounts a JOIN resources p ON p.id=a.provider_id JOIN resources account ON account.id=a.resource_id WHERE account.name='account'`).Scan(&provider); err != nil {
		t.Fatal(err)
	}
	if provider != "one" {
		t.Fatalf("partial provider transition=%q", provider)
	}
}

func TestPlanSerializationRedactsRegisteredSecrets(t *testing.T) {
	ctx := context.Background()
	manager, installation := managerForTest(t)
	secret := []byte("description-secret")
	if _, err := installation.Secrets().Put(ctx, storage.PutSecret{Name: "redaction-secret", Value: secret}, storage.Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	doc := mustParse(t, basePrefix+`resources:
- kind: Provider
  metadata: {name: provider, description: description-secret}
  spec: {}`)
	plan, err := manager.Plan(ctx, doc, false)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, secret) {
		t.Fatal("serialized plan disclosed registered secret")
	}
	if _, err := installation.Secrets().Put(ctx, storage.PutSecret{Name: "identity-secret", Value: []byte("provider")}, storage.Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	_, err = manager.Plan(ctx, doc, false)
	assertDiagnostic(t, err, "sensitive_identity", "$.resources[0].metadata.name")
}

func TestOrdinaryProjectionUpdatesDoNotTouchUnchangedRelations(t *testing.T) {
	ctx := context.Background()
	manager, installation := managerForTest(t)
	if _, err := installation.Secrets().Put(ctx, storage.PutSecret{Name: "example-key-secret", Value: []byte("test-secret-value")}, storage.Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	initial := exampleDocument(t)
	plan, err := manager.Plan(ctx, initial, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Apply(ctx, plan.Token, initial, false, storage.Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	updates := mustParse(t, basePrefix+`resources:
- {kind: ProviderAccount, metadata: {name: example-account, description: updated}, spec: {providerRef: {name: example-provider}}}
- {kind: ProviderConnection, metadata: {name: example-api}, spec: {providerRef: {name: example-provider}, baseUrl: https://openrouter.ai/api/v1, adapter: openai-chat-v1, enabled: false}}
- {kind: Credential, metadata: {name: example-key}, spec: {providerAccountRef: {name: example-account}, egressRef: {name: main-direct}, secretRef: {name: example-key-secret}, enabled: false}}
- {kind: Model, metadata: {name: example-chat}, spec: {connectionRef: {name: example-api}, providerModelId: example-chat-model, capabilities: [text, stream, tools], enabled: false}}`)
	plan, err = manager.Plan(ctx, updates, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Apply(ctx, plan.Token, updates, false, storage.Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
}

func TestUnrelatedApplyPreservesHistoricalDestinationLink(t *testing.T) {
	ctx := context.Background()
	manager, installation := managerForTest(t)
	if _, err := installation.Secrets().Put(ctx, storage.PutSecret{Name: "example-key-secret", Value: []byte("test-secret-value")}, storage.Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	initial := exampleDocument(t)
	plan, err := manager.Plan(ctx, initial, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Apply(ctx, plan.Token, initial, false, storage.Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	var destinationID string
	if err := installation.DB().QueryRowContext(ctx, "SELECT id FROM resources WHERE kind='Destination' AND name='example-primary'").Scan(&destinationID); err != nil {
		t.Fatal(err)
	}
	if _, err := installation.DB().ExecContext(ctx, `INSERT INTO requests(id,requested_alias,started_at) VALUES('request-history','example-assistant','2026-09-08T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := installation.DB().ExecContext(ctx, `INSERT INTO attempts(id,request_id,sequence,destination_id,started_at,outcome) VALUES('attempt-history','request-history',1,?,'2026-09-08T00:00:00Z','success')`, destinationID); err != nil {
		t.Fatal(err)
	}
	update := mustParse(t, basePrefix+`resources:
- {kind: Provider, metadata: {name: example-provider, description: updated}, spec: {}}`)
	plan, err = manager.Plan(ctx, update, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Apply(ctx, plan.Token, update, false, storage.Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	var linked *string
	if err := installation.DB().QueryRowContext(ctx, "SELECT destination_id FROM attempts WHERE id='attempt-history'").Scan(&linked); err != nil {
		t.Fatal(err)
	}
	if linked == nil || *linked != destinationID {
		t.Fatalf("historical destination link=%v", linked)
	}
}
