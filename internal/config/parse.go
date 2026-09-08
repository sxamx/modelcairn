package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

func Parse(data []byte) (*Document, error) {
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
	if err := inspectNode(root.Content[0], "$", 0); err != nil {
		return nil, err
	}
	value, err := nodeValue(root.Content[0])
	if err != nil {
		return nil, failure(CodeInvalidStructure, "$")
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, failure(CodeInvalidStructure, "$")
	}
	var raw rawDocument
	if err := decodeStrict(encoded, &raw); err != nil {
		return nil, failure(CodeInvalidStructure, "$")
	}
	if raw.Resources == nil {
		return nil, failure(CodeInvalidStructure, "$.resources")
	}
	doc := &Document{APIVersion: raw.APIVersion, Kind: raw.Kind, Resources: make([]Resource, 0, len(*raw.Resources))}
	for i, item := range *raw.Resources {
		resource, err := decodeResource(item, fmt.Sprintf("$.resources[%d]", i))
		if err != nil {
			return nil, err
		}
		doc.Resources = append(doc.Resources, resource)
	}
	if err := Validate(doc, nil); err != nil {
		return nil, err
	}
	return doc, nil
}

func inspectNode(node *yaml.Node, path string, depth int) error {
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
			if _, ok := seen[key.Value]; ok {
				return failure(CodeDuplicateKey, childPath(path, key.Value))
			}
			seen[key.Value] = struct{}{}
			if err := inspectNode(node.Content[i+1], childPath(path, key.Value), depth); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for i, child := range node.Content {
			if err := inspectNode(child, fmt.Sprintf("%s[%d]", path, i), depth); err != nil {
				return err
			}
		}
	case yaml.ScalarNode:
		if node.Tag == "!!null" && !strings.HasSuffix(path, ".spec.expiresAt") {
			return failure(CodeInvalidValue, path)
		}
	default:
		return failure(CodeInvalidStructure, path)
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
		case "!!null":
			return nil, nil
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
func decodeStrict(data []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return fmt.Errorf("trailing data")
	}
	return nil
}
func childPath(parent, key string) string {
	for _, r := range key {
		if !(r == '_' || r == '-' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return parent + "[key]"
		}
	}
	return parent + "." + key
}
