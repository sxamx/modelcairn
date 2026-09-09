package config

import (
	"reflect"
	"testing"
)

func TestResolvePreservesOmissionsAndReplacesArrays(t *testing.T) {
	old := mustParse(t, basePrefix+`resources:
- kind: Strategy
  metadata: {name: route-plan, displayName: Custom, description: Keep}
  spec: {destinations: [{name: old}], maxAttempts: 7, attemptTimeoutMs: 800, totalTimeoutMs: 900}`).Resources[0]
	desired := mustParse(t, basePrefix+`resources:
- kind: Strategy
  metadata: {name: route-plan}
  spec: {destinations: [{name: new}]}`).Resources[0]
	got, err := Resolve(desired, &old)
	if err != nil {
		t.Fatal(err)
	}
	spec := got.Spec.(StrategySpec)
	if spec.MaxAttempts != 7 || spec.AttemptTimeoutMS != 800 || spec.TotalTimeoutMS != 900 || !reflect.DeepEqual(spec.Destinations, []Ref{{Name: "new"}}) {
		t.Fatalf("unexpected resolved spec: %+v", spec)
	}
	if got.Metadata.DisplayName == nil || *got.Metadata.DisplayName != "Custom" || got.Metadata.Description == nil || *got.Metadata.Description != "Keep" {
		t.Fatal("metadata was not retained")
	}
	if desired.Spec.(StrategySpec).MaxAttempts != 3 || old.Spec.(StrategySpec).Destinations[0].Name != "old" {
		t.Fatal("input was mutated")
	}
}

func TestResolveExplicitFalseAndNullWin(t *testing.T) {
	old := mustParse(t, basePrefix+`resources:
- kind: AgentToken
  metadata: {name: agent}
  spec: {allowedRouteRefs: [{name: old}], enabled: true, expiresAt: "2030-01-01T00:00:00Z"}`).Resources[0]
	desired := mustParse(t, basePrefix+`resources:
- kind: AgentToken
  metadata: {name: agent}
  spec: {allowedRouteRefs: [{name: new}], enabled: false, expiresAt: null}`).Resources[0]
	got, err := Resolve(desired, &old)
	if err != nil {
		t.Fatal(err)
	}
	spec := got.Spec.(AgentTokenSpec)
	if spec.Enabled || spec.ExpiresAt != nil {
		t.Fatal("explicit values were overwritten")
	}
}

func TestResolveCreationIgnoresIdentityButUpdateChecksIt(t *testing.T) {
	desired := mustParse(t, basePrefix+`resources:
- kind: Egress
  metadata: {name: direct, uid: "00000000-0000-4000-8000-000000000001", resourceVersion: 9}
  spec: {type: direct}`).Resources[0]
	created, err := Resolve(desired, nil)
	if err != nil {
		t.Fatal(err)
	}
	if created.Metadata.UID != nil || created.Metadata.ResourceVersion != nil || !created.Spec.(EgressSpec).Enabled {
		t.Fatal("creation did not apply identity/default semantics")
	}
	version := int64(8)
	old := desired
	old.Metadata.ResourceVersion = &version
	_, err = Resolve(desired, &old)
	assertDiagnostic(t, err, "version_conflict", "$.metadata.resourceVersion")
}
