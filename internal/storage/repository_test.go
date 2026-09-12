package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

var testActor = Actor{Type: "cli", ID: "test-operator"}

func repositoryForTest(t *testing.T) (*Repository, *Installation) {
	t.Helper()
	installation, err := OpenInstallation(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = installation.Close() })
	repository := NewRepository(installation.DB())
	repository.now = func() time.Time { return time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC) }
	return repository, installation
}

func rawSpec(value string) json.RawMessage { return json.RawMessage(value) }

func putTestResource(t *testing.T, repository *Repository, kind ResourceKind, name, spec string) Resource {
	t.Helper()
	item, err := repository.Put(context.Background(), PutResource{Kind: kind, Name: name, Spec: rawSpec(spec)}, testActor)
	if err != nil {
		t.Fatalf("put %s/%s: %v", kind, name, err)
	}
	return item
}

func seedRepositoryGraph(t *testing.T, repository *Repository, installation *Installation) map[ResourceKind]Resource {
	t.Helper()
	ctx := context.Background()
	if _, err := installation.DB().ExecContext(ctx, `INSERT INTO secrets(id,name,key_version,algorithm,nonce,ciphertext,fingerprint,resource_version,created_at,updated_at)
		VALUES('secret-1','primary-secret',1,'XCHACHA20-POLY1305',zeroblob(24),x'01','mc_fp_test',1,'2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	items := map[ResourceKind]Resource{}
	items[KindProvider] = putTestResource(t, repository, KindProvider, "provider", `{}`)
	items[KindProviderAccount] = putTestResource(t, repository, KindProviderAccount, "account", `{"providerRef":{"name":"provider"}}`)
	items[KindProviderConnection] = putTestResource(t, repository, KindProviderConnection, "connection", `{"providerRef":{"name":"provider"},"baseUrl":"https://api.invalid/v1","adapter":"openai-chat-v1","allowPrivateNetwork":false,"enabled":true}`)
	items[KindEgress] = putTestResource(t, repository, KindEgress, "direct", `{"type":"direct","enabled":true}`)
	items[KindCredential] = putTestResource(t, repository, KindCredential, "credential", `{"providerAccountRef":{"name":"account"},"egressRef":{"name":"direct"},"secretRef":{"name":"primary-secret"},"enabled":true}`)
	items[KindModel] = putTestResource(t, repository, KindModel, "model", `{"connectionRef":{"name":"connection"},"providerModelId":"model-1","capabilities":["text","stream"],"enabled":true}`)
	items[KindDestination] = putTestResource(t, repository, KindDestination, "destination", `{"modelRef":{"name":"model"},"credentialRef":{"name":"credential"},"weight":100,"enabled":true}`)
	items[KindStrategy] = putTestResource(t, repository, KindStrategy, "strategy", `{"destinations":[{"name":"destination"}],"maxAttempts":1}`)
	items[KindRoute] = putTestResource(t, repository, KindRoute, "route", `{"modelAlias":"assistant","strategyRef":{"name":"strategy"},"enabled":true}`)
	items[KindAgentToken] = putTestResource(t, repository, KindAgentToken, "agent", `{"allowedRouteRefs":[{"name":"route"}],"expiresAt":null,"enabled":true}`)
	return items
}

func TestRepositoryRoundTripsEveryConfigurationResourceKind(t *testing.T) {
	repository, installation := repositoryForTest(t)
	created := seedRepositoryGraph(t, repository, installation)
	for kind, want := range created {
		got, err := repository.Get(context.Background(), kind, want.Name)
		if err != nil {
			t.Fatalf("get %s: %v", kind, err)
		}
		if got.ID != want.ID || got.ResourceVersion != 1 || string(got.Spec) != string(want.Spec) {
			t.Fatalf("round trip %s = %#v, want %#v", kind, got, want)
		}
	}
	listed, err := repository.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != len(validKinds) {
		t.Fatalf("listed %d resources, want %d", len(listed), len(validKinds))
	}
	for index := 1; index < len(listed); index++ {
		previous, current := fmt.Sprintf("%s/%s", listed[index-1].Kind, listed[index-1].Name), fmt.Sprintf("%s/%s", listed[index].Kind, listed[index].Name)
		if previous >= current {
			t.Fatalf("list is not deterministic at %q >= %q", previous, current)
		}
	}
	var successfulAudits int
	if err := installation.DB().QueryRow("SELECT count(*) FROM audit_events WHERE result='success'").Scan(&successfulAudits); err != nil {
		t.Fatal(err)
	}
	if successfulAudits != len(validKinds) {
		t.Fatalf("success audits = %d, want %d", successfulAudits, len(validKinds))
	}
}

func TestRepositoryUpdateIsAtomicAndOptimistic(t *testing.T) {
	repository, installation := repositoryForTest(t)
	items := seedRepositoryGraph(t, repository, installation)
	provider := items[KindProvider]
	display := "Updated Provider"
	updated, err := repository.Put(context.Background(), PutResource{Kind: KindProvider, Name: provider.Name, DisplayName: &display, Spec: rawSpec(`{}`), ExpectedVersion: 1}, testActor)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ResourceVersion != 2 || updated.DisplayName == nil || *updated.DisplayName != display {
		t.Fatalf("updated resource = %#v", updated)
	}
	_, err = repository.Put(context.Background(), PutResource{Kind: KindProvider, Name: provider.Name, Spec: rawSpec(`{"stale":true}`), ExpectedVersion: 1}, testActor)
	if !IsRepositoryCode(err, CodeVersionConflict) {
		t.Fatalf("stale update error = %v", err)
	}
	stored, err := repository.Get(context.Background(), KindProvider, provider.Name)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ResourceVersion != 2 || string(stored.Spec) != `{}` {
		t.Fatalf("stale update changed resource: %#v", stored)
	}
	var failures int
	if err := installation.DB().QueryRow("SELECT count(*) FROM audit_events WHERE action='resource.update' AND result='failure' AND json_extract(details_json,'$.code')='version_conflict'").Scan(&failures); err != nil {
		t.Fatal(err)
	}
	if failures != 1 {
		t.Fatalf("version-conflict audits = %d, want 1", failures)
	}
}

func TestConcurrentExpectedVersionAllowsExactlyOneWinner(t *testing.T) {
	repository, _ := repositoryForTest(t)
	putTestResource(t, repository, KindProvider, "provider", `{}`)
	start := make(chan struct{})
	results := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	for index := 0; index < 2; index++ {
		go func(index int) {
			ready.Done()
			<-start
			spec := rawSpec(fmt.Sprintf(`{"winner":%d}`, index))
			_, err := repository.Put(context.Background(), PutResource{Kind: KindProvider, Name: "provider", Spec: spec, ExpectedVersion: 1}, testActor)
			results <- err
		}(index)
	}
	ready.Wait()
	close(start)
	var success, conflict int
	for range 2 {
		err := <-results
		if err == nil {
			success++
		} else if IsRepositoryCode(err, CodeVersionConflict) {
			conflict++
		} else {
			t.Fatalf("unexpected update error: %v", err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("success=%d conflict=%d", success, conflict)
	}
}

func TestTypedFailureRollsBackEnvelopeAndAuditsSeparately(t *testing.T) {
	repository, installation := repositoryForTest(t)
	putTestResource(t, repository, KindProvider, "provider", `{}`)
	_, err := repository.Put(context.Background(), PutResource{Kind: KindProviderAccount, Name: "broken", Spec: rawSpec(`{"providerRef":{"name":"missing"}}`)}, testActor)
	if !IsRepositoryCode(err, CodeReferenceMissing) {
		t.Fatalf("put error = %v", err)
	}
	if _, err := repository.Get(context.Background(), KindProviderAccount, "broken"); !IsRepositoryCode(err, CodeNotFound) {
		t.Fatalf("rolled-back envelope exists: %v", err)
	}
	var success, failure int
	if err := installation.DB().QueryRow("SELECT count(*) FROM audit_events WHERE resource_kind='ProviderAccount' AND result='success'").Scan(&success); err != nil {
		t.Fatal(err)
	}
	if err := installation.DB().QueryRow("SELECT count(*) FROM audit_events WHERE resource_kind='ProviderAccount' AND result='failure'").Scan(&failure); err != nil {
		t.Fatal(err)
	}
	if success != 0 || failure != 1 {
		t.Fatalf("audits success=%d failure=%d", success, failure)
	}
}

func TestTypedUpdateFailureRollsBackEnvelopeVersionAndProjection(t *testing.T) {
	repository, installation := repositoryForTest(t)
	items := seedRepositoryGraph(t, repository, installation)
	putTestResource(t, repository, KindProvider, "other-provider", `{}`)
	otherConnection := putTestResource(t, repository, KindProviderConnection, "other-connection", `{"providerRef":{"name":"other-provider"},"baseUrl":"https://other.invalid/v1","adapter":"openai-chat-v1","allowPrivateNetwork":false,"enabled":true}`)
	original := items[KindModel]
	changedSpec := rawSpec(`{"connectionRef":{"name":"other-connection"},"providerModelId":"model-1","capabilities":["text"],"enabled":true}`)
	_, err := repository.Put(context.Background(), PutResource{Kind: KindModel, Name: original.Name, Spec: changedSpec, ExpectedVersion: 1}, testActor)
	if !IsRepositoryCode(err, CodeResourceInUse) {
		t.Fatalf("typed invariant error=%v", err)
	}
	stored, err := repository.Get(context.Background(), KindModel, original.Name)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ResourceVersion != 1 || string(stored.Spec) != string(original.Spec) {
		t.Fatalf("envelope changed after typed rollback: %#v", stored)
	}
	var connectionID string
	if err := installation.DB().QueryRow("SELECT connection_id FROM models WHERE resource_id=?", original.ID).Scan(&connectionID); err != nil {
		t.Fatal(err)
	}
	if connectionID == otherConnection.ID {
		t.Fatal("typed projection changed after rejected update")
	}
}

func TestTypedProjectionsPersistResolvedReferences(t *testing.T) {
	repository, installation := repositoryForTest(t)
	items := seedRepositoryGraph(t, repository, installation)
	var modelID, credentialID string
	if err := installation.DB().QueryRow("SELECT model_id,credential_id FROM destinations WHERE resource_id=?", items[KindDestination].ID).Scan(&modelID, &credentialID); err != nil {
		t.Fatal(err)
	}
	if modelID != items[KindModel].ID || credentialID != items[KindCredential].ID {
		t.Fatalf("destination projection model=%q credential=%q", modelID, credentialID)
	}
	var allowed string
	if err := installation.DB().QueryRow("SELECT allowed_routes_json FROM agent_tokens WHERE resource_id=?", items[KindAgentToken].ID).Scan(&allowed); err != nil {
		t.Fatal(err)
	}
	want, _ := json.Marshal([]string{items[KindRoute].ID})
	if allowed != string(want) {
		t.Fatalf("resolved routes=%s want=%s", allowed, want)
	}
}

func TestStrategyUpdatesPublishImmutableVersionsAndActivateRoutes(t *testing.T) {
	repository, installation := repositoryForTest(t)
	items := seedRepositoryGraph(t, repository, installation)
	var firstID, firstDefinition string
	var firstVersion int
	if err := installation.DB().QueryRow(`SELECT sv.id,sv.version,sv.definition_json
		FROM routes r JOIN strategy_versions sv ON sv.id=r.active_strategy_version_id
		WHERE r.resource_id=?`, items[KindRoute].ID).Scan(&firstID, &firstVersion, &firstDefinition); err != nil {
		t.Fatal(err)
	}
	if firstVersion != 1 || firstDefinition != string(items[KindStrategy].Spec) {
		t.Fatalf("first version=%d definition=%s", firstVersion, firstDefinition)
	}

	updatedDefinition := rawSpec(`{"destinations":[{"name":"destination"}],"maxAttempts":2,"attemptTimeoutMs":3000,"totalTimeoutMs":5000}`)
	if _, err := repository.Put(context.Background(), PutResource{
		Kind: KindStrategy, Name: items[KindStrategy].Name, Spec: updatedDefinition, ExpectedVersion: 1,
	}, testActor); err != nil {
		t.Fatal(err)
	}
	var activeID, activeDefinition string
	var activeVersion, versionCount int
	if err := installation.DB().QueryRow(`SELECT sv.id,sv.version,sv.definition_json
		FROM routes r JOIN strategy_versions sv ON sv.id=r.active_strategy_version_id
		WHERE r.resource_id=?`, items[KindRoute].ID).Scan(&activeID, &activeVersion, &activeDefinition); err != nil {
		t.Fatal(err)
	}
	if err := installation.DB().QueryRow("SELECT count(*) FROM strategy_versions WHERE strategy_id=?", items[KindStrategy].ID).Scan(&versionCount); err != nil {
		t.Fatal(err)
	}
	if activeID == firstID || activeVersion != 2 || versionCount != 2 || activeDefinition != string(updatedDefinition) {
		t.Fatalf("active=%q version=%d count=%d definition=%s", activeID, activeVersion, versionCount, activeDefinition)
	}
	var preserved string
	if err := installation.DB().QueryRow("SELECT definition_json FROM strategy_versions WHERE id=?", firstID).Scan(&preserved); err != nil {
		t.Fatal(err)
	}
	if preserved != firstDefinition {
		t.Fatalf("historical definition changed: %s", preserved)
	}
}

func TestEquivalentStrategyDefinitionReusesPublishedVersion(t *testing.T) {
	repository, installation := repositoryForTest(t)
	items := seedRepositoryGraph(t, repository, installation)
	if _, err := repository.Put(context.Background(), PutResource{
		Kind: KindStrategy, Name: items[KindStrategy].Name, Spec: items[KindStrategy].Spec, ExpectedVersion: 1,
	}, testActor); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := installation.DB().QueryRow("SELECT count(*) FROM strategy_versions WHERE strategy_id=?", items[KindStrategy].ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("versions=%d, want 1", count)
	}
}

func TestAgentTokenRevocationCannotContradictEnvelope(t *testing.T) {
	repository, installation := repositoryForTest(t)
	items := seedRepositoryGraph(t, repository, installation)
	agent := items[KindAgentToken]
	if _, err := installation.DB().Exec("UPDATE agent_tokens SET verifier_sha256=zeroblob(32),token_prefix='mc_test',issued_at='2026-09-06T12:00:00Z' WHERE resource_id=?", agent.ID); err != nil {
		t.Fatal(err)
	}
	disabled := rawSpec(`{"allowedRouteRefs":[{"name":"route"}],"expiresAt":null,"enabled":false}`)
	if _, err := repository.Put(context.Background(), PutResource{Kind: KindAgentToken, Name: agent.Name, Spec: disabled, ExpectedVersion: 1}, testActor); err != nil {
		t.Fatal(err)
	}
	var firstRevokedAt string
	if err := installation.DB().QueryRow("SELECT revoked_at FROM agent_tokens WHERE resource_id=?", agent.ID).Scan(&firstRevokedAt); err != nil {
		t.Fatal(err)
	}
	repository.now = func() time.Time { return time.Date(2026, 9, 6, 13, 0, 0, 0, time.UTC) }
	if _, err := repository.Put(context.Background(), PutResource{Kind: KindAgentToken, Name: agent.Name, Spec: disabled, ExpectedVersion: 2}, testActor); err != nil {
		t.Fatal(err)
	}
	reenabled := rawSpec(`{"allowedRouteRefs":[{"name":"route"}],"expiresAt":null,"enabled":true}`)
	if _, err := repository.Put(context.Background(), PutResource{Kind: KindAgentToken, Name: agent.Name, Spec: reenabled, ExpectedVersion: 3}, testActor); !IsRepositoryCode(err, CodeInvalidResource) {
		t.Fatalf("reactivation error=%v", err)
	}
	stored, err := repository.Get(context.Background(), KindAgentToken, agent.Name)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ResourceVersion != 3 || string(stored.Spec) != string(disabled) {
		t.Fatalf("reactivation changed envelope: %#v", stored)
	}
	var revokedAt *string
	if err := installation.DB().QueryRow("SELECT revoked_at FROM agent_tokens WHERE resource_id=?", agent.ID).Scan(&revokedAt); err != nil {
		t.Fatal(err)
	}
	if revokedAt == nil {
		t.Fatal("disabled token projection was not revoked")
	}
	if *revokedAt != firstRevokedAt {
		t.Fatalf("revocation timestamp changed from %q to %q", firstRevokedAt, *revokedAt)
	}
	var verifier []byte
	var prefix, issued string
	if err := installation.DB().QueryRow("SELECT verifier_sha256,token_prefix,issued_at FROM agent_tokens WHERE resource_id=?", agent.ID).Scan(&verifier, &prefix, &issued); err != nil {
		t.Fatal(err)
	}
	if len(verifier) != 32 || prefix != "mc_test" || issued != "2026-09-06T12:00:00Z" {
		t.Fatalf("token identity fields changed: verifier=%d prefix=%q issued=%q", len(verifier), prefix, issued)
	}
}

func TestSuccessAuditFailureRollsBackMutation(t *testing.T) {
	repository, installation := repositoryForTest(t)
	if _, err := installation.DB().Exec(`CREATE TRIGGER reject_success_audit BEFORE INSERT ON audit_events
		WHEN NEW.result='success' BEGIN SELECT RAISE(ABORT,'audit_unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	_, err := repository.Put(context.Background(), PutResource{Kind: KindProvider, Name: "provider", Spec: rawSpec(`{}`)}, testActor)
	if err == nil {
		t.Fatal("mutation succeeded without its success audit")
	}
	if _, getErr := repository.Get(context.Background(), KindProvider, "provider"); !IsRepositoryCode(getErr, CodeNotFound) {
		t.Fatalf("mutation survived audit rollback: %v", getErr)
	}
	var failures int
	if scanErr := installation.DB().QueryRow("SELECT count(*) FROM audit_events WHERE result='failure'").Scan(&failures); scanErr != nil {
		t.Fatal(scanErr)
	}
	if failures != 1 {
		t.Fatalf("failure audits=%d, want 1", failures)
	}
}

func TestRepositoryRejectsProviderMismatch(t *testing.T) {
	repository, installation := repositoryForTest(t)
	seedRepositoryGraph(t, repository, installation)
	putTestResource(t, repository, KindProvider, "other-provider", `{}`)
	putTestResource(t, repository, KindProviderAccount, "other-account", `{"providerRef":{"name":"other-provider"}}`)
	if _, err := installation.DB().Exec(`INSERT INTO secrets VALUES('secret-2','other-secret',1,'XCHACHA20-POLY1305',zeroblob(24),x'02','mc_fp_other',1,'2026-01-01','2026-01-01')`); err != nil {
		t.Fatal(err)
	}
	putTestResource(t, repository, KindCredential, "other-credential", `{"providerAccountRef":{"name":"other-account"},"egressRef":{"name":"direct"},"secretRef":{"name":"other-secret"},"enabled":true}`)
	_, err := repository.Put(context.Background(), PutResource{Kind: KindDestination, Name: "mismatch", Spec: rawSpec(`{"modelRef":{"name":"model"},"credentialRef":{"name":"other-credential"},"weight":100,"enabled":true}`)}, testActor)
	if !IsRepositoryCode(err, CodeProviderMismatch) {
		t.Fatalf("provider mismatch error = %v", err)
	}
	if _, err := repository.Get(context.Background(), KindDestination, "mismatch"); !IsRepositoryCode(err, CodeNotFound) {
		t.Fatalf("mismatched envelope committed: %v", err)
	}
}

func TestRepositoryEnforcesLogicalAndPhysicalDeletionOrder(t *testing.T) {
	repository, installation := repositoryForTest(t)
	items := seedRepositoryGraph(t, repository, installation)
	ctx := context.Background()
	if err := repository.Delete(ctx, KindProvider, "provider", 1, true, testActor); !IsRepositoryCode(err, CodeResourceInUse) {
		t.Fatalf("provider delete error=%v", err)
	}
	if err := repository.Delete(ctx, KindDestination, "destination", 1, true, testActor); !IsRepositoryCode(err, CodeResourceInUse) {
		t.Fatalf("destination delete error=%v", err)
	}
	if err := repository.Delete(ctx, KindRoute, "route", 1, true, testActor); !IsRepositoryCode(err, CodeResourceInUse) {
		t.Fatalf("route delete error=%v", err)
	}
	for _, kind := range []ResourceKind{KindAgentToken, KindRoute, KindStrategy, KindDestination, KindModel, KindCredential} {
		item := items[kind]
		if err := repository.Delete(ctx, kind, item.Name, item.ResourceVersion, true, testActor); err != nil {
			t.Fatalf("ordered delete %s: %v", kind, err)
		}
	}
	var secrets int
	if err := installation.DB().QueryRow("SELECT count(*) FROM secrets WHERE name='primary-secret'").Scan(&secrets); err != nil {
		t.Fatal(err)
	}
	if secrets != 1 {
		t.Fatal("deleting Credential deleted its Secret")
	}
}

func TestAuditDetailsUseOnlyTypedAllowlistedFields(t *testing.T) {
	repository, installation := repositoryForTest(t)
	putTestResource(t, repository, KindProvider, "provider", `{}`)
	_, _ = repository.Put(context.Background(), PutResource{Kind: KindProvider, Name: "provider", Spec: rawSpec(`{}`), ExpectedVersion: 99}, testActor)
	rows, err := installation.DB().Query(`SELECT DISTINCT key FROM audit_events, json_each(audit_events.details_json) ORDER BY key`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			t.Fatal(err)
		}
		keys = append(keys, key)
	}
	if fmt.Sprint(keys) != "[code version]" {
		t.Fatalf("audit detail keys = %v", keys)
	}
}

func TestInvalidResourceAuditUsesContractualActionAndDetails(t *testing.T) {
	repository, installation := repositoryForTest(t)
	_, err := repository.Put(context.Background(), PutResource{Kind: KindProvider, Name: "provider", Spec: rawSpec(`[]`)}, testActor)
	if !IsRepositoryCode(err, CodeInvalidResource) {
		t.Fatalf("invalid resource error=%v", err)
	}
	var action, code string
	if err := installation.DB().QueryRow(`SELECT action,json_extract(details_json,'$.code') FROM audit_events WHERE result='failure'`).Scan(&action, &code); err != nil {
		t.Fatal(err)
	}
	if action != "resource.create" || code != CodeInvalidResource {
		t.Fatalf("audit action=%q code=%q", action, code)
	}
}

func TestDeleteRequiresExplicitAuthorization(t *testing.T) {
	repository, _ := repositoryForTest(t)
	item := putTestResource(t, repository, KindProvider, "provider", `{}`)
	err := repository.Delete(context.Background(), item.Kind, item.Name, item.ResourceVersion, false, testActor)
	if !IsRepositoryCode(err, CodeDeleteNotAllowed) {
		t.Fatalf("delete error=%v", err)
	}
	if _, err := repository.Get(context.Background(), item.Kind, item.Name); err != nil {
		t.Fatalf("unauthorized delete changed state: %v", err)
	}
}

func TestRepositoryErrorsRemainInspectableWhenFailureAuditFails(t *testing.T) {
	repository, installation := repositoryForTest(t)
	putTestResource(t, repository, KindProvider, "provider", `{}`)
	if _, err := installation.DB().Exec("DROP TABLE audit_events"); err != nil {
		t.Fatal(err)
	}
	_, err := repository.Put(context.Background(), PutResource{Kind: KindProviderAccount, Name: "broken", Spec: rawSpec(`{"providerRef":{"name":"missing"}}`)}, testActor)
	var combined *MutationAuditError
	if !errors.As(err, &combined) || !IsRepositoryCode(err, CodeReferenceMissing) {
		t.Fatalf("combined error=%T %v", err, err)
	}
}
