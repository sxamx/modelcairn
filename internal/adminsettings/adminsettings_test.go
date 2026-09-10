package adminsettings

import (
	"bytes"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const initialPrefix = "apiVersion: modelcairn.io/v1alpha1\nkind: AdminSettings\nspec:\n  publicOrigin: http://127.0.0.1:8080\n"

func TestInitialDefaultsAndCanonicalOutput(t *testing.T) {
	doc, err := Parse([]byte(initialPrefix), Initial)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ResolveInitial(doc)
	if err != nil {
		t.Fatal(err)
	}
	want := Defaults()
	want.PublicOrigin = "http://127.0.0.1:8080"
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolved mismatch: %#v", got)
	}
	a, _ := CanonicalJSON(got)
	b, _ := CanonicalJSON(got)
	if !bytes.Equal(a, b) {
		t.Fatal("canonical output drifted")
	}
}

func TestUpdatePreservesOmissions(t *testing.T) {
	doc, err := Parse([]byte("apiVersion: modelcairn.io/v1alpha1\nkind: AdminSettings\nresourceVersion: 7\nspec:\n  idleSeconds: 600\n"), Update)
	if err != nil {
		t.Fatal(err)
	}
	current := Defaults()
	current.PublicOrigin = "http://127.0.0.1:8080"
	got, err := ResolveUpdate(doc, current)
	if err != nil {
		t.Fatal(err)
	}
	if got.IdleSeconds != 600 || got.AbsoluteSeconds != current.AbsoluteSeconds {
		t.Fatal("update omission semantics")
	}
	got.TrustedProxyCIDRs = append(got.TrustedProxyCIDRs, "127.0.0.1/32")
	if len(current.TrustedProxyCIDRs) != 0 {
		t.Fatal("resolved update aliases current slice")
	}
}

func TestResolveNormalizesOriginDefaultPortAndCase(t *testing.T) {
	doc, err := Parse([]byte("apiVersion: modelcairn.io/v1alpha1\nkind: AdminSettings\nspec:\n  publicOrigin: HTTP://LOCALHOST:80/\n"), Initial)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := ResolveInitial(doc)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.PublicOrigin != "http://localhost" {
		t.Fatalf("origin=%q", resolved.PublicOrigin)
	}
}

func TestRawOriginLimitAppliesBeforeNormalization(t *testing.T) {
	raw := "http://" + strings.Repeat("a", 2041) + ":80/"
	doc := &Document{APIVersion: APIVersion, Kind: DocumentKind, Spec: Patch{PublicOrigin: &raw}}
	_, err := ResolveInitial(doc)
	assertDiagnostic(t, err, CodeInvalidValue, "$.spec.publicOrigin")
}

func TestResolvedAlwaysOwnsNonNilProxySlice(t *testing.T) {
	version := int64(1)
	current := Defaults()
	current.PublicOrigin = "http://127.0.0.1:8080"
	current.TrustedProxyCIDRs = nil
	got, err := ResolveUpdate(&Document{ResourceVersion: &version}, current)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := CanonicalJSON(got)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoded, []byte(`"trustedProxyCidrs":[]`)) {
		t.Fatalf("canonical=%s", encoded)
	}
}

func TestStrictParsing(t *testing.T) {
	tests := []struct {
		name, input, code, path string
		mode                    ParseMode
	}{
		{"empty", "", CodeEmptyDocument, "$", Initial},
		{"unknown", initialPrefix + "  extra: secret-canary\n", CodeInvalidStructure, "$.spec.extra", Initial},
		{"duplicate", initialPrefix + "  idleSeconds: 600\n  idleSeconds: 601\n", CodeDuplicateKey, "$.spec.idleSeconds", Initial},
		{"null", initialPrefix + "  listen: null\n", CodeInvalidValue, "$.spec.listen", Initial},
		{"alias", initialPrefix + "  trustedProxyCidrs: &x []\n  tlsCertificatePath: *x\n", CodeAliasNotAllowed, "$.spec.tlsCertificatePath", Initial},
		{"multi", initialPrefix + "---\n{}\n", CodeMultipleDocuments, "$", Initial},
		{"initial-version", "apiVersion: modelcairn.io/v1alpha1\nkind: AdminSettings\nresourceVersion: 1\nspec: {publicOrigin: http://127.0.0.1:8080}\n", CodeInvalidStructure, "$.resourceVersion", Initial},
		{"update-version", "apiVersion: modelcairn.io/v1alpha1\nkind: AdminSettings\nspec: {}\n", CodeInvalidStructure, "$.resourceVersion", Update},
		{"missing-spec", "apiVersion: modelcairn.io/v1alpha1\nkind: AdminSettings\n", CodeInvalidStructure, "$.spec", Initial},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.input), tc.mode)
			assertDiagnostic(t, err, tc.code, tc.path)
			if strings.Contains(err.Error(), "secret-canary") {
				t.Fatal("value leaked")
			}
		})
	}
	_, err := Parse(make([]byte, MaxInputBytes+1), Initial)
	assertDiagnostic(t, err, CodeInputTooLarge, "$")
}

func TestSemanticBounds(t *testing.T) {
	base := Defaults()
	base.PublicOrigin = "http://127.0.0.1:8080"
	tests := []struct {
		name, path string
		mutate     func(*Resolved)
	}{
		{"idle-low", "$.spec.idleSeconds", func(v *Resolved) { v.IdleSeconds = 299 }}, {"idle-over-absolute", "$.spec.idleSeconds", func(v *Resolved) { v.IdleSeconds = v.AbsoluteSeconds + 1 }},
		{"global-rate", "$.spec.globalAttemptsPerMinute", func(v *Resolved) { v.GlobalAttemptsPerMinute = 0 }}, {"client-rate", "$.spec.clientAttemptsPerMinute", func(v *Resolved) { v.ClientAttemptsPerMinute = v.GlobalAttemptsPerMinute + 1 }},
		{"global-burst", "$.spec.globalBurst", func(v *Resolved) { v.GlobalBurst = 21 }}, {"client-burst", "$.spec.clientBurst", func(v *Resolved) { v.ClientBurst = v.GlobalBurst + 1 }},
		{"entries", "$.spec.maxClientEntries", func(v *Resolved) { v.MaxClientEntries = 63 }}, {"client-idle", "$.spec.clientIdleSeconds", func(v *Resolved) { v.ClientIdleSeconds = 3601 }},
		{"argon-memory", "$.spec.argonMemoryKiB", func(v *Resolved) { v.ArgonMemoryKiB = 65537 }}, {"argon-iterations", "$.spec.argonIterations", func(v *Resolved) { v.ArgonIterations = 1 }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := base
			tc.mutate(&v)
			assertDiagnostic(t, Validate(v), CodeInvalidValue, tc.path)
		})
	}
}

func TestTransportCombinations(t *testing.T) {
	base := Defaults()
	base.PublicOrigin = "http://127.0.0.1:8080"
	if err := Validate(base); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"HTTP://127.0.0.1:8080", "http://127.0.0.1:80", "http://127.0.0.1:8080/path", "http://user@127.0.0.1:8080", "http://*.example.test"} {
		t.Run(bad, func(t *testing.T) {
			v := base
			v.PublicOrigin = bad
			assertDiagnostic(t, Validate(v), CodeInvalidValue, "$.spec.publicOrigin")
		})
	}
	localhost := base
	localhost.PublicOrigin = "http://localhost:8080"
	if err := Validate(localhost); err != nil {
		t.Fatalf("localhost loopback: %v", err)
	}
	for _, bad := range []string{"localhost:8080", "127.0.0.1:08080", "0.0.0.0:0", "[0:0::1]:8080"} {
		v := base
		v.Listen = bad
		assertDiagnostic(t, Validate(v), CodeInvalidValue, "$.spec.listen")
	}
	proxy := base
	proxy.PublicOrigin = "https://gateway.example"
	proxy.Transport = ProxyTLS
	proxy.TrustedProxyCIDRs = []string{"100.64.0.0/10"}
	if err := Validate(proxy); err != nil {
		t.Fatal(err)
	}
	for _, cidrs := range [][]string{{"100.64.1.0/10"}, {"bad"}, {"100.64.0.0/10", "100.64.0.0/10"}} {
		v := proxy
		v.TrustedProxyCIDRs = cidrs
		assertDiagnostic(t, Validate(v), CodeInvalidValue, "$.spec.trustedProxyCidrs")
	}
	direct := base
	direct.PublicOrigin = "https://gateway.example"
	direct.Transport = DirectTLS
	direct.TLSCertificatePath = filepath.Join(t.TempDir(), "cert.pem")
	direct.TLSPrivateKeyPath = filepath.Join(t.TempDir(), "key.pem")
	if err := Validate(direct); err != nil {
		t.Fatal(err)
	}
	direct.TLSPrivateKeyPath = "relative"
	assertDiagnostic(t, Validate(direct), CodeInvalidValue, "$.spec.tlsPrivateKeyPath")
}

func TestStringBounds(t *testing.T) {
	base := Defaults()
	base.PublicOrigin = "http://127.0.0.1:8080"
	v := base
	v.Listen = strings.Repeat("x", 129)
	assertDiagnostic(t, Validate(v), CodeInvalidValue, "$.spec.listen")
	v = base
	v.PublicOrigin = strings.Repeat("x", 2049)
	assertDiagnostic(t, Validate(v), CodeInvalidValue, "$.spec.publicOrigin")
	v = base
	v.TrustedProxyCIDRs = []string{strings.Repeat("1", 65)}
	assertDiagnostic(t, Validate(v), CodeInvalidValue, "$.spec.trustedProxyCidrs")
	v = base
	v.TLSCertificatePath = strings.Repeat("x", 4097)
	assertDiagnostic(t, Validate(v), CodeInvalidValue, "$.spec.tlsCertificatePath")
}

func TestFileAccessibilityIsNotPureValidation(t *testing.T) {
	v := Defaults()
	v.PublicOrigin = "https://gateway.example"
	v.Transport = DirectTLS
	v.TLSCertificatePath = filepath.Join(t.TempDir(), "missing-cert")
	v.TLSPrivateKeyPath = filepath.Join(t.TempDir(), "missing-key")
	if err := Validate(v); err != nil {
		t.Fatal(err)
	}
}

func assertDiagnostic(t *testing.T, err error, code, path string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}
	var target *Error
	if !errors.As(err, &target) || len(target.Diagnostics) == 0 {
		t.Fatalf("unexpected error: %v", err)
	}
	if target.Diagnostics[0].Code != code || target.Diagnostics[0].Path != path {
		t.Fatalf("diagnostic=%+v", target.Diagnostics[0])
	}
}
