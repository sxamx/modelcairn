package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/sxamx/modelcairn/internal/redact"
)

func TestExampleRoundTripsCanonically(t *testing.T) {
	input, err := os.ReadFile("../../docs/contratos/config/example-v1alpha1.yaml")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Resources) != 10 {
		t.Fatalf("resources=%d", len(doc.Resources))
	}
	encoded, err := CanonicalJSON(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Parse(encoded)
	if err != nil {
		t.Fatal(err)
	}
	second, _ := CanonicalJSON(again, nil)
	if !bytes.Equal(encoded, second) {
		t.Fatal("canonical round trip drifted")
	}
}

func TestDefaultsAreCanonical(t *testing.T) {
	doc := mustParse(t, `apiVersion: modelcairn.io/v1alpha1
kind: Configuration
resources:
- kind: ProviderConnection
  metadata: {name: api}
  spec: {providerRef: {name: provider}, baseUrl: https://example.invalid/v1, adapter: openai-chat-v1}
- kind: Destination
  metadata: {name: destination}
  spec: {modelRef: {name: model}, credentialRef: {name: credential}}
- kind: Strategy
  metadata: {name: strategy}
  spec: {destinations: [{name: destination}]}
`)
	connection := doc.Resources[0].Spec.(ProviderConnectionSpec)
	if !connection.Enabled || connection.AllowPrivateNetwork {
		t.Fatal("connection defaults")
	}
	if doc.Resources[0].FieldPresence("enabled") != FieldOmitted || doc.Resources[0].FieldPresence("allowPrivateNetwork") != FieldOmitted {
		t.Fatal("omitted connection fields lost their presence state")
	}
	destination := doc.Resources[1].Spec.(DestinationSpec)
	if !destination.Enabled || destination.Weight != 100 {
		t.Fatal("destination defaults")
	}
	strategy := doc.Resources[2].Spec.(StrategySpec)
	if strategy.MaxAttempts != 3 || strategy.AttemptTimeoutMS != 60000 || strategy.TotalTimeoutMS != 120000 {
		t.Fatal("strategy defaults")
	}
	encoded, err := CanonicalJSON(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte(`"maxAttempts"`)) || bytes.Contains(encoded, []byte(`"weight"`)) {
		t.Fatal("canonical export turned omitted defaults into explicit updates")
	}
}

func TestRequiredCollectionsRejectMissingAndNull(t *testing.T) {
	tests := []struct{ input, code string }{
		{basePrefix + `resources:
- kind: Model
  metadata: {name: model}
  spec: {connectionRef: {name: api}, providerModelId: model}`, CodeInvalidStructure},
		{basePrefix + `resources:
- kind: Model
  metadata: {name: model}
  spec: {connectionRef: {name: api}, providerModelId: model, capabilities: null}`, CodeInvalidValue},
		{basePrefix + `resources:
- kind: Provider
  metadata: {name: provider}
  spec: null`, CodeInvalidValue},
	}
	for _, test := range tests {
		_, err := Parse([]byte(test.input))
		assertDiagnostic(t, err, test.code, "")
	}
}

func TestExplicitNullSurvivesForNullableOptionalField(t *testing.T) {
	doc := mustParse(t, basePrefix+`resources:
- kind: AgentToken
  metadata: {name: agent}
  spec: {allowedRouteRefs: [{name: route}], expiresAt: null}`)
	if doc.Resources[0].FieldPresence("expiresAt") != FieldNull {
		t.Fatal("explicit null was lost")
	}
	encoded, err := CanonicalJSON(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoded, []byte(`"expiresAt": null`)) {
		t.Fatal("explicit null was omitted")
	}
}

func TestParserRejectsHostileYAMLBeforeValuesEscape(t *testing.T) {
	tests := []struct{ name, input, code, path string }{
		{"duplicate", basePrefix + "resources: []\nresources: []\n", CodeDuplicateKey, "$.resources"},
		{"non-string", basePrefix + "resources: []\n1: value\n", CodeNonStringKey, "$"},
		{"alias", basePrefix + "resources: &items []\nextra: *items\n", CodeAliasNotAllowed, "$.extra"},
		{"tag", basePrefix + "resources: !custom []\n", CodeTagNotAllowed, "$.resources"},
		{"documents", basePrefix + "resources: []\n---\n{}\n", CodeMultipleDocuments, "$"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.input))
			assertDiagnostic(t, err, tc.code, tc.path)
			if strings.Contains(err.Error(), "value") {
				t.Fatal("diagnostic echoed rejected scalar")
			}
		})
	}
}

func TestLimits(t *testing.T) {
	_, err := Parse(make([]byte, MaxInputBytes+1))
	assertDiagnostic(t, err, CodeInputTooLarge, "$")
	var b strings.Builder
	b.WriteString(basePrefix + "resources:\n")
	for i := 0; i < 65; i++ {
		b.WriteString(strings.Repeat("  ", i+1) + "-\n")
	}
	_, err = Parse([]byte(b.String()))
	assertDiagnostic(t, err, CodeDepthExceeded, "")
}

func TestResourceCountBoundary(t *testing.T) {
	build := func(count int) []byte {
		var b strings.Builder
		b.WriteString(basePrefix + "resources:\n")
		for i := 0; i < count; i++ {
			fmt.Fprintf(&b, "- {kind: Provider, metadata: {name: p-%05d}, spec: {}}\n", i)
		}
		return []byte(b.String())
	}
	if _, err := Parse(build(10000)); err != nil {
		t.Fatalf("10,000 resources: %v", err)
	}
	_, err := Parse(build(10001))
	assertDiagnostic(t, err, CodeInvalidValue, "$.resources")
}

func TestUnicodeLengthCountsCharacters(t *testing.T) {
	valid := strings.Repeat("á", 120)
	if _, err := Parse([]byte(basePrefix + "resources:\n- kind: Provider\n  metadata: {name: provider, displayName: '" + valid + "'}\n  spec: {}\n")); err != nil {
		t.Fatal(err)
	}
	invalid := valid + "á"
	_, err := Parse([]byte(basePrefix + "resources:\n- kind: Provider\n  metadata: {name: provider, displayName: '" + invalid + "'}\n  spec: {}\n"))
	assertDiagnostic(t, err, CodeInvalidValue, "$.resources[0].metadata.displayName")
}

func TestDuplicateJSONKeyIsRejected(t *testing.T) {
	_, err := Parse([]byte(`{"apiVersion":"modelcairn.io/v1alpha1","kind":"Configuration","kind":"Configuration","resources":[]}`))
	assertDiagnostic(t, err, CodeDuplicateKey, "$.kind")
}

func TestInvalidURLsAndTombstones(t *testing.T) {
	for _, rawURL := range []string{"https://user@example.invalid/v1", "https://example.invalid/v1?q=x", "https://example.invalid/v1#x", "file:///tmp/api"} {
		t.Run(rawURL, func(t *testing.T) {
			input := basePrefix + `resources:
- kind: ProviderConnection
  metadata: {name: api}
  spec: {providerRef: {name: provider}, baseUrl: "` + rawURL + `", adapter: openai-chat-v1}
`
			_, err := Parse([]byte(input))
			assertDiagnostic(t, err, CodeInvalidValue, "$.resources[0].spec.baseUrl")
		})
	}
	_, err := Parse([]byte(basePrefix + `resources:
- kind: Provider
  state: absent
  metadata: {name: old, description: forbidden-canary}
`))
	assertDiagnostic(t, err, CodeInvalidStructure, "$.resources[0]")
	if strings.Contains(err.Error(), "forbidden-canary") {
		t.Fatal("secret-like canary escaped")
	}
}

func TestPrivateLiteralURLsRequireExplicitPolicy(t *testing.T) {
	for _, rawURL := range []string{"http://localhost/v1", "http://127.0.0.1/v1", "http://10.0.0.1/v1", "http://192.168.1.2/v1", "http://169.254.1.1/v1", "http://[::1]/v1", "http://[fd00::1]/v1"} {
		t.Run(rawURL, func(t *testing.T) {
			input := basePrefix + `resources:
- kind: ProviderConnection
  metadata: {name: api}
  spec: {providerRef: {name: provider}, baseUrl: "` + rawURL + `", adapter: openai-chat-v1}`
			_, err := Parse([]byte(input))
			assertDiagnostic(t, err, CodeInvalidValue, "$.resources[0].spec.baseUrl")
		})
	}
	mustParse(t, basePrefix+`resources:
- kind: ProviderConnection
  metadata: {name: api}
  spec: {providerRef: {name: provider}, baseUrl: "http://127.0.0.1/v1", adapter: openai-chat-v1, allowPrivateNetwork: true}`)
}

func TestProviderAffinityAndReferences(t *testing.T) {
	input := basePrefix + `resources:
- {kind: Provider, metadata: {name: one}, spec: {}}
- {kind: Provider, metadata: {name: two}, spec: {}}
- kind: ProviderAccount
  metadata: {name: account}
  spec: {providerRef: {name: one}}
- kind: ProviderConnection
  metadata: {name: connection}
  spec: {providerRef: {name: two}, baseUrl: https://example.invalid, adapter: openai-chat-v1}
- kind: Egress
  metadata: {name: direct}
  spec: {type: direct}
- kind: Credential
  metadata: {name: key}
  spec: {providerAccountRef: {name: account}, egressRef: {name: direct}, secretRef: {name: stored-secret}}
- kind: Model
  metadata: {name: model}
  spec: {connectionRef: {name: connection}, providerModelId: x, capabilities: [text]}
- kind: Destination
  metadata: {name: destination}
  spec: {modelRef: {name: model}, credentialRef: {name: key}}`
	_, err := Parse([]byte(input))
	assertDiagnostic(t, err, CodeProviderMismatch, "$.resources[7].spec.credentialRef")
	partial := mustParse(t, basePrefix+`resources:
- kind: Route
  metadata: {name: route}
  spec: {modelAlias: chat, strategyRef: {name: missing}}
`)
	err = Validate(partial, emptyCatalog{})
	assertDiagnostic(t, err, CodeReferenceNotFound, "$.resources[0].spec.strategyRef")
}

func TestStableDiagnosticsDoNotEchoCanaries(t *testing.T) {
	canary := "TOP-SECRET-CANARY-9921"
	_, err := Parse([]byte(basePrefix + "resources:\n- kind: Provider\n  metadata: {name: " + canary + "}\n  spec: {}\n"))
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), canary) {
		t.Fatal("diagnostic leaked canary")
	}
	assertDiagnostic(t, err, CodeInvalidValue, "$.resources[0].metadata.name")
}

func TestCanonicalExportUsesSharedRedactor(t *testing.T) {
	canary := `TOP-SECRET-"CANARY"-9921`
	doc := mustParse(t, basePrefix+`resources:
- kind: Provider
  metadata:
    name: provider
    description: 'TOP-SECRET-"CANARY"-9921'
  spec: {}`)
	r := redact.New()
	release, err := r.Register([]byte(canary))
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	encoded, err := CanonicalJSON(doc, r)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte(canary)) || !bytes.Contains(encoded, []byte(redact.Replacement)) {
		t.Fatal("export was not redacted")
	}
	if _, err := Parse(encoded); err != nil {
		t.Fatalf("redacted export is invalid: %v", err)
	}
}

func TestProgrammaticResourceWithoutPresenceKeepsExplicitValues(t *testing.T) {
	doc := &Document{APIVersion: APIVersion, Kind: DocumentKind, Resources: []Resource{{
		Kind: DestinationKind, State: Present, Metadata: Metadata{Name: "destination"},
		Spec: DestinationSpec{ModelRef: Ref{Name: "model"}, CredentialRef: Ref{Name: "key"}, Enabled: false, Weight: 321},
	}}}
	encoded, err := CanonicalJSON(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoded, []byte(`"enabled": false`)) || !bytes.Contains(encoded, []byte(`"weight": 321`)) {
		t.Fatal("programmatic explicit fields were omitted")
	}
}

func TestReferenceToDeclaredTombstoneFailsWithoutCatalog(t *testing.T) {
	_, err := Parse([]byte(basePrefix + `resources:
- kind: Strategy
  state: absent
  metadata: {name: retired}
- kind: Route
  metadata: {name: route}
  spec: {modelAlias: chat, strategyRef: {name: retired}}`))
	assertDiagnostic(t, err, CodeReferenceNotFound, "$.resources[1].spec.strategyRef")
}

func TestTombstoneCannotLeaveExistingReferenceBehind(t *testing.T) {
	doc := mustParse(t, basePrefix+`resources:
- kind: Strategy
  state: absent
  metadata: {name: active}`)
	catalog := staticCatalog{items: []Resource{{
		Kind: RouteKind, State: Present, Metadata: Metadata{Name: "existing-route"},
		Spec: RouteSpec{ModelAlias: "chat", StrategyRef: Ref{Name: "active"}, Enabled: true},
	}, {
		Kind: StrategyKind, State: Present, Metadata: Metadata{Name: "active"},
		Spec: StrategySpec{Destinations: []Ref{{Name: "destination"}}, MaxAttempts: 1, AttemptTimeoutMS: 1000, TotalTimeoutMS: 2000},
	}}}
	err := Validate(doc, catalog)
	assertDiagnostic(t, err, CodeReferenceNotFound, "$.existing[Route/existing-route].spec.strategyRef")
}

func TestRelatedTombstonesMayDeleteExistingGraphTogether(t *testing.T) {
	doc := mustParse(t, basePrefix+`resources:
- kind: Route
  state: absent
  metadata: {name: existing-route}
- kind: Strategy
  state: absent
  metadata: {name: active}`)
	catalog := staticCatalog{items: []Resource{{
		Kind: RouteKind, State: Present, Metadata: Metadata{Name: "existing-route"},
		Spec: RouteSpec{ModelAlias: "chat", StrategyRef: Ref{Name: "active"}, Enabled: true},
	}, {
		Kind: StrategyKind, State: Present, Metadata: Metadata{Name: "active"},
		Spec: StrategySpec{Destinations: []Ref{{Name: "destination"}}, MaxAttempts: 1, AttemptTimeoutMS: 1000, TotalTimeoutMS: 2000},
	}}}
	if err := Validate(doc, catalog); err != nil {
		t.Fatal(err)
	}
}

func TestNullIsRejectedExceptForAgentTokenExpiry(t *testing.T) {
	inputs := []string{
		basePrefix + "resources:\n- kind: Provider\n  state: null\n  metadata: {name: provider}\n  spec: {}\n",
		basePrefix + "resources:\n- kind: Provider\n  metadata: {name: provider, displayName: null}\n  spec: {}\n",
		basePrefix + "resources:\n- kind: ProviderConnection\n  metadata: {name: api}\n  spec: {providerRef: {name: provider}, baseUrl: https://example.invalid, adapter: openai-chat-v1, enabled: null}\n",
		basePrefix + "resources:\n- kind: Destination\n  metadata: {name: destination}\n  spec: {modelRef: {name: model}, credentialRef: {name: key}, weight: null}\n",
		basePrefix + "resources:\n- kind: Strategy\n  metadata: {name: strategy}\n  spec: {destinations: [{name: destination}], maxAttempts: null}\n",
	}
	for i, input := range inputs {
		t.Run(fmt.Sprintf("case-%d", i), func(t *testing.T) {
			_, err := Parse([]byte(input))
			assertDiagnostic(t, err, CodeInvalidValue, "")
		})
	}
}

func TestCredentialSecretMustExistInCatalog(t *testing.T) {
	doc := mustParse(t, basePrefix+`resources:
- kind: Credential
  metadata: {name: key}
  spec: {providerAccountRef: {name: account}, egressRef: {name: direct}, secretRef: {name: missing-secret}}`)
	catalog := staticCatalog{items: []Resource{
		{Kind: ProviderAccountKind, State: Present, Metadata: Metadata{Name: "account"}, Spec: ProviderAccountSpec{ProviderRef: Ref{Name: "provider"}}},
		{Kind: EgressKind, State: Present, Metadata: Metadata{Name: "direct"}, Spec: EgressSpec{Type: "direct", Enabled: true}},
		{Kind: ProviderKind, State: Present, Metadata: Metadata{Name: "provider"}, Spec: ProviderSpec{}},
	}}
	err := Validate(doc, catalog)
	assertDiagnostic(t, err, CodeReferenceNotFound, "$.resources[0].spec.secretRef")
}

type emptyCatalog struct{}

func (emptyCatalog) Resources() []Resource    { return nil }
func (emptyCatalog) SecretExists(string) bool { return false }

type staticCatalog struct {
	items   []Resource
	secrets map[string]bool
}

func (c staticCatalog) Resources() []Resource         { return c.items }
func (c staticCatalog) SecretExists(name string) bool { return c.secrets[name] }

const basePrefix = "apiVersion: modelcairn.io/v1alpha1\nkind: Configuration\n"

func mustParse(t *testing.T, input string) *Document {
	t.Helper()
	d, e := Parse([]byte(input))
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func assertDiagnostic(t *testing.T, err error, code, path string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s", code)
	}
	var target *Error
	if !errors.As(err, &target) || len(target.Diagnostics) == 0 {
		t.Fatalf("unexpected error %T %v", err, err)
	}
	got := target.Diagnostics[0]
	if got.Code != code {
		t.Fatalf("code=%s want=%s path=%s", got.Code, code, got.Path)
	}
	if path != "" && got.Path != path {
		t.Fatalf("path=%s want=%s", got.Path, path)
	}
}
