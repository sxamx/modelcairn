package config

import "testing"

func TestPrepareClassifiesOperationsAndPreservesDisabled(t *testing.T) {
	old := mustParse(t, basePrefix+`resources:
- {kind: Egress, metadata: {name: keep}, spec: {type: direct, enabled: false}}
- {kind: Provider, metadata: {name: remove}, spec: {}}
- {kind: Provider, metadata: {name: change}, spec: {}}`)
	desired := mustParse(t, basePrefix+`resources:
- {kind: Provider, metadata: {name: new}, spec: {}}
- {kind: Egress, metadata: {name: keep}, spec: {type: direct}}
- {kind: Provider, metadata: {name: change, description: updated}, spec: {}}
- {kind: Provider, state: absent, metadata: {name: remove}}`)
	plan, err := Prepare(desired, staticCatalog{items: old.Resources}, true)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]string{"new": "create", "keep": "noop", "change": "update", "remove": "delete"}
	for _, c := range plan.Changes {
		if c.Action != expected[c.Name] {
			t.Fatalf("unexpected change: %+v", c)
		}
	}
	if plan.Resolved.Resources[1].Spec.(EgressSpec).Enabled {
		t.Fatal("omitted enabled reset to true")
	}
	_, err = Prepare(desired, staticCatalog{items: old.Resources}, false)
	assertDiagnostic(t, err, "delete_not_allowed", "$.resources[3]")
}
