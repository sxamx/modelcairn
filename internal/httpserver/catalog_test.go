package httpserver

import (
	"bytes"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/sxamx/modelcairn/internal/config"
	"github.com/sxamx/modelcairn/internal/redact"
)

func TestResourceBodyPreservesStrictDuplicateDetection(t *testing.T) {
	request := httptest.NewRequest("POST", "/", bytes.NewBufferString(`{
		"kind":"Provider","metadata":{"name":"acme","name":"other"},"spec":{}}`))
	_, err := decodeResourceBody(request)
	var parseErr *config.Error
	if err == nil || !errors.As(err, &parseErr) || len(parseErr.Diagnostics) == 0 || parseErr.Diagnostics[0].Code != config.CodeDuplicateKey {
		t.Fatalf("duplicate body error=%v", err)
	}
}

func TestResourceDTOStringRedactionPreservesStructure(t *testing.T) {
	redactor := redact.New()
	release, err := redactor.Register([]byte("super-secret-value"))
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	input := map[string]any{"nested": []any{"prefix super-secret-value suffix", float64(7), true}}
	got := redactResourceStrings(input, redactor.String).(map[string]any)
	items := got["nested"].([]any)
	if items[0] != "prefix [REDACTED] suffix" || items[1] != float64(7) || items[2] != true {
		t.Fatalf("redacted DTO=%#v", got)
	}
}
