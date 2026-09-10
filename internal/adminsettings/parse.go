package adminsettings

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"strconv"
	"strings"
)

type ParseMode uint8

const (
	Initial ParseMode = iota
	Update
)

func Parse(data []byte, mode ParseMode) (*Document, error) {
	if len(data) > MaxInputBytes {
		return nil, failure(CodeInputTooLarge, "$")
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var root yaml.Node
	if err := decoder.Decode(&root); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, failure(CodeEmptyDocument, "$")
		}
		return nil, failure(CodeInvalidStructure, "$")
	}
	if len(root.Content) != 1 || root.Content[0].Kind == 0 {
		return nil, failure(CodeEmptyDocument, "$")
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, failure(CodeInvalidStructure, "$")
		}
		return nil, failure(CodeMultipleDocuments, "$")
	}
	if err := inspect(root.Content[0], "$", 0); err != nil {
		return nil, err
	}
	value, err := nodeValue(root.Content[0])
	if err != nil {
		return nil, failure(CodeInvalidStructure, "$")
	}
	top, ok := value.(map[string]any)
	if !ok {
		return nil, failure(CodeInvalidStructure, "$")
	}
	if _, ok := top["spec"]; !ok {
		return nil, failure(CodeInvalidStructure, "$.spec")
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, failure(CodeInvalidStructure, "$")
	}
	var result Document
	strict := json.NewDecoder(bytes.NewReader(encoded))
	strict.DisallowUnknownFields()
	if strict.Decode(&result) != nil {
		return nil, failure(CodeInvalidStructure, "$")
	}
	if err := validatePatchStructure(result.Spec); err != nil {
		return nil, err
	}
	if result.APIVersion != APIVersion {
		return nil, failure(CodeInvalidValue, "$.apiVersion")
	}
	if result.Kind != DocumentKind {
		return nil, failure(CodeInvalidValue, "$.kind")
	}
	if mode == Initial && result.ResourceVersion != nil {
		return nil, failure(CodeInvalidStructure, "$.resourceVersion")
	}
	if mode == Update && (result.ResourceVersion == nil || *result.ResourceVersion < 1) {
		return nil, failure(CodeInvalidStructure, "$.resourceVersion")
	}
	if mode != Initial && mode != Update {
		return nil, failure(CodeInvalidStructure, "$")
	}
	return &result, nil
}

func validatePatchStructure(p Patch) error {
	if p.PublicOrigin != nil && !validString(*p.PublicOrigin, 2048) {
		return failure(CodeInvalidValue, "$.spec.publicOrigin")
	}
	if p.Listen != nil && !validString(*p.Listen, 128) {
		return failure(CodeInvalidValue, "$.spec.listen")
	}
	if p.TLSCertificatePath != nil && !validOptionalString(*p.TLSCertificatePath, 4096) {
		return failure(CodeInvalidValue, "$.spec.tlsCertificatePath")
	}
	if p.TLSPrivateKeyPath != nil && !validOptionalString(*p.TLSPrivateKeyPath, 4096) {
		return failure(CodeInvalidValue, "$.spec.tlsPrivateKeyPath")
	}
	if p.TrustedProxyCIDRs != nil {
		if len(*p.TrustedProxyCIDRs) > 32 {
			return failure(CodeInvalidValue, "$.spec.trustedProxyCidrs")
		}
		for _, item := range *p.TrustedProxyCIDRs {
			if !validString(item, 64) {
				return failure(CodeInvalidValue, "$.spec.trustedProxyCidrs")
			}
		}
	}
	return nil
}
func inspect(node *yaml.Node, path string, depth int) error {
	if node.Kind == yaml.AliasNode {
		return failure(CodeAliasNotAllowed, path)
	}
	allowed := map[string]bool{"!!map": true, "!!seq": true, "!!str": true, "!!int": true, "!!float": true, "!!bool": true, "!!null": true}
	if !allowed[node.Tag] {
		return failure(CodeTagNotAllowed, path)
	}
	if node.Kind == yaml.MappingNode || node.Kind == yaml.SequenceNode {
		depth++
		if depth > MaxDepth {
			return failure(CodeDepthExceeded, path)
		}
	}
	switch node.Kind {
	case yaml.MappingNode:
		seen := map[string]struct{}{}
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
				return failure(CodeNonStringKey, path)
			}
			child := childPath(path, key.Value)
			if fields := allowedFields(path); fields != nil && !fields[key.Value] {
				return failure(CodeInvalidStructure, child)
			}
			if _, ok := seen[key.Value]; ok {
				return failure(CodeDuplicateKey, child)
			}
			seen[key.Value] = struct{}{}
			if err := inspect(node.Content[i+1], child, depth); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for i, child := range node.Content {
			if err := inspect(child, fmt.Sprintf("%s[%d]", path, i), depth); err != nil {
				return err
			}
		}
	case yaml.ScalarNode:
		if node.Tag == "!!null" {
			return failure(CodeInvalidValue, path)
		}
	default:
		return failure(CodeInvalidStructure, path)
	}
	return nil
}

func allowedFields(path string) map[string]bool {
	if path == "$" {
		return map[string]bool{"apiVersion": true, "kind": true, "resourceVersion": true, "spec": true}
	}
	if path == "$.spec" {
		return map[string]bool{"publicOrigin": true, "listen": true, "transport": true, "trustedProxyCidrs": true, "tlsCertificatePath": true, "tlsPrivateKeyPath": true, "idleSeconds": true, "absoluteSeconds": true, "globalAttemptsPerMinute": true, "globalBurst": true, "clientAttemptsPerMinute": true, "clientBurst": true, "maxClientEntries": true, "clientIdleSeconds": true, "argonMemoryKiB": true, "argonIterations": true}
	}
	return nil
}
func nodeValue(node *yaml.Node) (any, error) {
	switch node.Kind {
	case yaml.MappingNode:
		r := make(map[string]any, len(node.Content)/2)
		for i := 0; i < len(node.Content); i += 2 {
			v, e := nodeValue(node.Content[i+1])
			if e != nil {
				return nil, e
			}
			r[node.Content[i].Value] = v
		}
		return r, nil
	case yaml.SequenceNode:
		r := make([]any, len(node.Content))
		for i, c := range node.Content {
			v, e := nodeValue(c)
			if e != nil {
				return nil, e
			}
			r[i] = v
		}
		return r, nil
	case yaml.ScalarNode:
		switch node.Tag {
		case "!!str":
			return node.Value, nil
		case "!!bool":
			return strings.EqualFold(node.Value, "true"), nil
		case "!!int":
			return strconv.ParseInt(node.Value, 0, 64)
		case "!!float":
			return strconv.ParseFloat(node.Value, 64)
		}
	}
	return nil, fmt.Errorf("unsupported node")
}
func childPath(parent, key string) string {
	for _, r := range key {
		if !(r == '_' || r == '-' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return parent + "[key]"
		}
	}
	return parent + "." + key
}
