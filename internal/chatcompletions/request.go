// Package chatcompletions implements ModelCairn's strict Phase 1 data contract.
package chatcompletions

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"unicode/utf8"
)

const (
	MaxRequestBytes      = 1 << 20
	MaxRequestAliasBytes = 128
	MaxMessages          = 4096
	MaxTools             = 128
	maxJSONDepth         = 64
)

var aliasPattern = regexp.MustCompile(`^[A-Za-z0-9._:/-]{1,` + fmt.Sprint(MaxRequestAliasBytes) + `}$`)
var functionPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type Capability string

const (
	CapabilityText          Capability = "text"
	CapabilityStream        Capability = "stream"
	CapabilityTools         Capability = "tools"
	CapabilityParallelTools Capability = "parallel-tools"
	CapabilityDeveloperRole Capability = "developer-role"
)

type Error struct{ Code, Param string }

func (e *Error) Error() string         { return e.Code }
func invalid(code, param string) error { return &Error{Code: code, Param: param} }

type Request struct {
	Model               string          `json:"model"`
	Messages            []Message       `json:"messages"`
	Stream              bool            `json:"stream,omitempty"`
	StreamOptions       *StreamOptions  `json:"stream_options,omitempty"`
	Tools               []Tool          `json:"tools,omitempty"`
	ToolChoice          json.RawMessage `json:"tool_choice,omitempty"`
	ParallelToolCalls   *bool           `json:"parallel_tool_calls,omitempty"`
	Temperature         *float64        `json:"temperature,omitempty"`
	TopP                *float64        `json:"top_p,omitempty"`
	MaxTokens           *int            `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int            `json:"max_completion_tokens,omitempty"`
}
type Message struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content,omitempty"`
	Name       string          `json:"name,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall      `json:"tool_calls,omitempty"`
}
type TextPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
type Tool struct {
	Type     string       `json:"type"`
	Function FunctionTool `json:"function"`
}
type FunctionTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
	Strict      *bool           `json:"strict,omitempty"`
}
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}
type Parsed struct {
	Request      Request
	Capabilities []Capability
}

func Parse(data []byte) (Parsed, error) {
	if len(data) == 0 {
		return Parsed{}, invalid("invalid_request", "")
	}
	if len(data) > MaxRequestBytes {
		return Parsed{}, invalid("request_too_large", "")
	}
	if err := ValidateJSONDocument(data); err != nil {
		return Parsed{}, err
	}
	var request Request
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil {
		return Parsed{}, invalid("invalid_request", "")
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return Parsed{}, invalid("invalid_json", "")
	}
	capabilities, err := validateRequest(request)
	if err != nil {
		return Parsed{}, err
	}
	return Parsed{Request: request, Capabilities: capabilities}, nil
}

// ValidateJSONDocument rejects ambiguous or abusive JSON before typed decoding.
// It is also used for untrusted upstream Chat Completions responses.
func ValidateJSONDocument(data []byte) error {
	if !utf8.Valid(data) {
		return invalid("invalid_json", "")
	}
	return validateJSONShape(data)
}

func validateRequest(r Request) ([]Capability, error) {
	if !aliasPattern.MatchString(r.Model) {
		return nil, invalid("invalid_value", "model")
	}
	if len(r.Messages) == 0 || len(r.Messages) > MaxMessages {
		return nil, invalid("invalid_value", "messages")
	}
	required := map[Capability]bool{CapabilityText: true}
	for i, message := range r.Messages {
		if err := validateMessage(message, i, required); err != nil {
			return nil, err
		}
	}
	if r.Stream {
		required[CapabilityStream] = true
	} else if r.StreamOptions != nil {
		return nil, invalid("invalid_value", "stream_options")
	}
	if len(r.Tools) > MaxTools {
		return nil, invalid("invalid_value", "tools")
	}
	seen := make(map[string]struct{}, len(r.Tools))
	for i, tool := range r.Tools {
		if tool.Type != "function" || !functionPattern.MatchString(tool.Function.Name) {
			return nil, invalid("invalid_value", fmt.Sprintf("tools[%d]", i))
		}
		if _, exists := seen[tool.Function.Name]; exists {
			return nil, invalid("duplicate_tool", fmt.Sprintf("tools[%d].function.name", i))
		}
		seen[tool.Function.Name] = struct{}{}
		if len(tool.Function.Description) > 4096 || !validJSONObject(tool.Function.Parameters) {
			return nil, invalid("invalid_value", fmt.Sprintf("tools[%d].function.parameters", i))
		}
	}
	if len(r.Tools) > 0 {
		required[CapabilityTools] = true
	}
	if len(r.ToolChoice) > 0 {
		if len(r.Tools) == 0 || validateToolChoice(r.ToolChoice, seen) != nil {
			return nil, invalid("invalid_value", "tool_choice")
		}
		required[CapabilityTools] = true
	}
	if r.ParallelToolCalls != nil {
		if len(r.Tools) == 0 {
			return nil, invalid("invalid_value", "parallel_tool_calls")
		}
		if *r.ParallelToolCalls {
			required[CapabilityParallelTools] = true
		}
	}
	if r.Temperature != nil && (*r.Temperature < 0 || *r.Temperature > 2) {
		return nil, invalid("invalid_value", "temperature")
	}
	if r.TopP != nil && (*r.TopP < 0 || *r.TopP > 1) {
		return nil, invalid("invalid_value", "top_p")
	}
	if r.MaxTokens != nil && r.MaxCompletionTokens != nil {
		return nil, invalid("conflicting_parameters", "max_tokens")
	}
	if r.MaxTokens != nil && *r.MaxTokens < 1 {
		return nil, invalid("invalid_value", "max_tokens")
	}
	if r.MaxCompletionTokens != nil && *r.MaxCompletionTokens < 1 {
		return nil, invalid("invalid_value", "max_completion_tokens")
	}
	result := make([]Capability, 0, len(required))
	for capability := range required {
		result = append(result, capability)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result, nil
}

func validateMessage(m Message, index int, required map[Capability]bool) error {
	param := fmt.Sprintf("messages[%d]", index)
	if len(m.Name) > 64 {
		return invalid("invalid_value", param+".name")
	}
	switch m.Role {
	case "developer":
		required[CapabilityDeveloperRole] = true
		fallthrough
	case "system", "user":
		if !validTextContent(m.Content, false) || m.ToolCallID != "" || len(m.ToolCalls) > 0 {
			return invalid("invalid_value", param)
		}
	case "assistant":
		if !validTextContent(m.Content, true) || m.ToolCallID != "" {
			return invalid("invalid_value", param)
		}
		if (len(m.Content) == 0 || bytes.Equal(m.Content, []byte("null"))) && len(m.ToolCalls) == 0 {
			return invalid("invalid_value", param+".content")
		}
		for i, call := range m.ToolCalls {
			if call.ID == "" || len(call.ID) > 256 || call.Type != "function" || !functionPattern.MatchString(call.Function.Name) || len(call.Function.Arguments) > MaxRequestBytes {
				return invalid("invalid_value", fmt.Sprintf("%s.tool_calls[%d]", param, i))
			}
		}
		if len(m.ToolCalls) > 0 {
			required[CapabilityTools] = true
		}
	case "tool":
		if m.ToolCallID == "" || len(m.ToolCallID) > 256 || !validTextContent(m.Content, false) || len(m.ToolCalls) > 0 {
			return invalid("invalid_value", param)
		}
		required[CapabilityTools] = true
	default:
		return invalid("invalid_value", param+".role")
	}
	return nil
}

func validTextContent(raw json.RawMessage, nullable bool) bool {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nullable
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return utf8.ValidString(text)
	}
	var parts []TextPart
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&parts) != nil || len(parts) == 0 {
		return false
	}
	for _, part := range parts {
		if part.Type != "text" || !utf8.ValidString(part.Text) {
			return false
		}
	}
	return true
}

func validJSONObject(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return true
	}
	var value map[string]any
	return json.Unmarshal(raw, &value) == nil && value != nil
}

func validateToolChoice(raw json.RawMessage, tools map[string]struct{}) error {
	var mode string
	if json.Unmarshal(raw, &mode) == nil {
		if mode == "none" || mode == "auto" || mode == "required" {
			return nil
		}
		return errors.New("invalid mode")
	}
	var named struct {
		Type     string `json:"type"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&named) != nil || named.Type != "function" {
		return errors.New("invalid choice")
	}
	if _, exists := tools[named.Function.Name]; !exists {
		return errors.New("unknown tool")
	}
	return nil
}

func validateJSONShape(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := walkJSON(decoder, 0); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return invalid("invalid_json", "")
	}
	return nil
}
func walkJSON(decoder *json.Decoder, depth int) error {
	if depth > maxJSONDepth {
		return invalid("json_depth_exceeded", "")
	}
	token, err := decoder.Token()
	if err != nil {
		return invalid("invalid_json", "")
	}
	delimiter, composite := token.(json.Delim)
	if !composite {
		return nil
	}
	want := json.Delim(0)
	switch delimiter {
	case '{':
		want = '}'
		seen := map[string]struct{}{}
		for decoder.More() {
			key, err := decoder.Token()
			name, ok := key.(string)
			if err != nil || !ok {
				return invalid("invalid_json", "")
			}
			if _, duplicate := seen[name]; duplicate {
				return invalid("duplicate_field", name)
			}
			seen[name] = struct{}{}
			if err := walkJSON(decoder, depth+1); err != nil {
				return err
			}
		}
	case '[':
		want = ']'
		for decoder.More() {
			if err := walkJSON(decoder, depth+1); err != nil {
				return err
			}
		}
	default:
		return invalid("invalid_json", "")
	}
	closing, err := decoder.Token()
	if err != nil || closing != want {
		return invalid("invalid_json", "")
	}
	return nil
}
