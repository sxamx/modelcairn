package chatcompletions

import (
	"errors"
	"strings"
	"testing"
)

func TestParseMinimalRequest(t *testing.T) {
	parsed, err := Parse([]byte(`{"model":"primary/chat","messages":[{"role":"user","content":"hello"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Request.Model != "primary/chat" || len(parsed.Capabilities) != 1 || parsed.Capabilities[0] != CapabilityText {
		t.Fatalf("parsed=%+v", parsed)
	}
}

func TestParseDerivesCapabilities(t *testing.T) {
	input := `{"model":"agent","messages":[{"role":"developer","content":[{"type":"text","text":"help"}]},{"role":"user","content":"run"}],"stream":true,"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}}],"tool_choice":{"type":"function","function":{"name":"lookup"}},"parallel_tool_calls":true,"max_completion_tokens":50}`
	parsed, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	want := []Capability{CapabilityDeveloperRole, CapabilityParallelTools, CapabilityStream, CapabilityText, CapabilityTools}
	if len(parsed.Capabilities) != len(want) {
		t.Fatalf("capabilities=%v", parsed.Capabilities)
	}
	for i := range want {
		if parsed.Capabilities[i] != want[i] {
			t.Fatalf("capabilities=%v", parsed.Capabilities)
		}
	}
}

func TestParseRejectsInvalidRequests(t *testing.T) {
	tests := []struct{ name, input, code, param string }{
		{"unknown", `{"model":"x","messages":[],"surprise":true}`, "invalid_request", ""},
		{"duplicate", `{"model":"x","model":"y","messages":[]}`, "duplicate_field", "model"},
		{"missing-messages", `{"model":"x"}`, "invalid_value", "messages"},
		{"bad-alias", `{"model":"has space","messages":[{"role":"user","content":"x"}]}`, "invalid_value", "model"},
		{"multimodal", `{"model":"x","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"x"}}]}]}`, "invalid_value", "messages[0]"},
		{"empty-assistant", `{"model":"x","messages":[{"role":"assistant"}]}`, "invalid_value", "messages[0].content"},
		{"stream-options", `{"model":"x","messages":[{"role":"user","content":"x"}],"stream_options":{"include_usage":true}}`, "invalid_value", "stream_options"},
		{"token-conflict", `{"model":"x","messages":[{"role":"user","content":"x"}],"max_tokens":1,"max_completion_tokens":1}`, "conflicting_parameters", "max_tokens"},
		{"unknown-tool", `{"model":"x","messages":[{"role":"user","content":"x"}],"tools":[{"type":"function","function":{"name":"one"}}],"tool_choice":{"type":"function","function":{"name":"two"}}}`, "invalid_value", "tool_choice"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse([]byte(test.input))
			var contract *Error
			if !errors.As(err, &contract) || contract.Code != test.code || contract.Param != test.param {
				t.Fatalf("error=%#v", err)
			}
		})
	}
}

func TestParseRejectsNestedDuplicateAndDepth(t *testing.T) {
	_, err := Parse([]byte(`{"model":"x","messages":[{"role":"user","content":"x"}],"tools":[{"type":"function","function":{"name":"one","parameters":{"type":"object","type":"array"}}}]}`))
	var contract *Error
	if !errors.As(err, &contract) || contract.Code != "duplicate_field" || contract.Param != "type" {
		t.Fatalf("duplicate error=%v", err)
	}
	deep := strings.Repeat(`{"x":`, maxJSONDepth+2) + `0` + strings.Repeat(`}`, maxJSONDepth+2)
	_, err = Parse([]byte(deep))
	if !errors.As(err, &contract) || contract.Code != "json_depth_exceeded" {
		t.Fatalf("depth error=%v", err)
	}
}

func TestParseRequestSizeBound(t *testing.T) {
	_, err := Parse(make([]byte, MaxRequestBytes+1))
	var contract *Error
	if !errors.As(err, &contract) || contract.Code != "request_too_large" {
		t.Fatalf("error=%v", err)
	}
}
