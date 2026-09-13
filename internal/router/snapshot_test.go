package router

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/sxamx/modelcairn/internal/chatcompletions"
	"github.com/sxamx/modelcairn/internal/storage"
)

func seedSnapshotGraph(t *testing.T) (*Loader, *storage.Repository, *storage.Installation, map[storage.ResourceKind]storage.Resource) {
	t.Helper()
	ctx := context.Background()
	installation, err := storage.OpenInstallation(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = installation.Close() })
	if _, err := installation.DB().ExecContext(ctx, `INSERT INTO secrets(id,name,key_version,algorithm,nonce,ciphertext,fingerprint,resource_version,created_at,updated_at)
		VALUES('secret-1','primary-secret',1,'XCHACHA20-POLY1305',zeroblob(24),x'01','mc_fp_fixture',1,'2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	repository := storage.NewRepository(installation.DB())
	actor := storage.Actor{Type: "system", ID: "snapshot-test"}
	put := func(kind storage.ResourceKind, name, spec string) storage.Resource {
		t.Helper()
		item, putErr := repository.Put(ctx, storage.PutResource{Kind: kind, Name: name, Spec: json.RawMessage(spec)}, actor)
		if putErr != nil {
			t.Fatalf("put %s/%s: %v", kind, name, putErr)
		}
		return item
	}
	items := map[storage.ResourceKind]storage.Resource{}
	items[storage.KindProvider] = put(storage.KindProvider, "provider", `{}`)
	items[storage.KindProviderAccount] = put(storage.KindProviderAccount, "account", `{"providerRef":{"name":"provider"}}`)
	items[storage.KindProviderConnection] = put(storage.KindProviderConnection, "connection", `{"providerRef":{"name":"provider"},"baseUrl":"https://api.invalid/v1","adapter":"openai-chat-v1","allowPrivateNetwork":false,"enabled":true}`)
	items[storage.KindEgress] = put(storage.KindEgress, "direct", `{"type":"direct","enabled":true}`)
	items[storage.KindCredential] = put(storage.KindCredential, "credential", `{"providerAccountRef":{"name":"account"},"egressRef":{"name":"direct"},"secretRef":{"name":"primary-secret"},"enabled":true}`)
	items[storage.KindModel] = put(storage.KindModel, "model", `{"connectionRef":{"name":"connection"},"providerModelId":"physical-model","capabilities":["text","stream"],"enabled":true}`)
	items[storage.KindDestination] = put(storage.KindDestination, "destination", `{"modelRef":{"name":"model"},"credentialRef":{"name":"credential"},"weight":100,"enabled":true}`)
	items[storage.KindStrategy] = put(storage.KindStrategy, "strategy", `{"destinations":[{"name":"destination"}],"maxAttempts":3,"attemptTimeoutMs":1000,"totalTimeoutMs":2500}`)
	items[storage.KindRoute] = put(storage.KindRoute, "route", `{"modelAlias":"assistant","strategyRef":{"name":"strategy"},"enabled":true}`)
	loader := NewLoader(installation.DB())
	loader.now = func() time.Time { return time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC) }
	return loader, repository, installation, items
}

func TestLoadSnapshotPreservesAffinityBudgetsAndEligibility(t *testing.T) {
	loader, _, installation, items := seedSnapshotGraph(t)
	snapshot, err := loader.Load(context.Background(), "assistant", []chatcompletions.Capability{chatcompletions.CapabilityText, chatcompletions.CapabilityStream})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.StrategyVersion != 1 || snapshot.MaxAttempts != 3 || snapshot.AttemptTimeout != time.Second || snapshot.TotalTimeout != 2500*time.Millisecond {
		t.Fatalf("snapshot=%+v", snapshot)
	}
	if len(snapshot.Destinations) != 1 {
		t.Fatalf("destinations=%d", len(snapshot.Destinations))
	}
	destination := snapshot.Destinations[0]
	if !destination.Eligible() || destination.ProviderModelID != "physical-model" || destination.EgressType != "direct" || destination.SecretID != "secret-1" || destination.SecretName != "primary-secret" {
		t.Fatalf("destination=%+v", destination)
	}
	if _, err := installation.DB().Exec(`INSERT INTO cooldowns(id,scope_kind,scope_resource_id,reason,starts_at,ends_at)
		VALUES('cooldown-1','credential',?,'rate_limited','2026-09-12T11:00:00Z','2026-09-12T13:00:00Z')`, items[storage.KindCredential].ID); err != nil {
		t.Fatal(err)
	}
	cooling, err := loader.Load(context.Background(), "assistant", []chatcompletions.Capability{chatcompletions.CapabilityTools})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"capability_missing:tools", "cooldown_active"}
	if got := cooling.Destinations[0].ExclusionReasons; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("reasons=%v", got)
	}
}

func TestLoadedSnapshotRemainsImmutableAfterStrategyPublication(t *testing.T) {
	loader, repository, _, items := seedSnapshotGraph(t)
	first, err := loader.Load(context.Background(), "assistant", nil)
	if err != nil {
		t.Fatal(err)
	}
	updated := json.RawMessage(`{"destinations":[{"name":"destination"}],"maxAttempts":1,"attemptTimeoutMs":500,"totalTimeoutMs":900}`)
	if _, err := repository.Put(context.Background(), storage.PutResource{Kind: storage.KindStrategy, Name: "strategy", Spec: updated, ExpectedVersion: items[storage.KindStrategy].ResourceVersion}, storage.Actor{Type: "system"}); err != nil {
		t.Fatal(err)
	}
	second, err := loader.Load(context.Background(), "assistant", nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.StrategyVersion != 1 || first.MaxAttempts != 3 || second.StrategyVersion != 2 || second.MaxAttempts != 1 || first.StrategyVersionID == second.StrategyVersionID {
		t.Fatalf("first=%+v second=%+v", first, second)
	}
}

func TestLoadSnapshotDistinguishesMissingAndDisabledRoute(t *testing.T) {
	loader, _, installation, items := seedSnapshotGraph(t)
	_, err := loader.Load(context.Background(), "missing", nil)
	var loadErr *LoadError
	if !errors.As(err, &loadErr) || loadErr.Code != CodeRouteNotFound {
		t.Fatalf("missing err=%v", err)
	}
	if _, err := installation.DB().Exec("UPDATE routes SET enabled=0 WHERE resource_id=?", items[storage.KindRoute].ID); err != nil {
		t.Fatal(err)
	}
	_, err = loader.Load(context.Background(), "assistant", nil)
	if !errors.As(err, &loadErr) || loadErr.Code != CodeRouteDisabled {
		t.Fatalf("disabled err=%v", err)
	}
}
