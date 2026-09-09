package config

import (
	"bytes"
	"encoding/json"
	"sort"
)

// Change contains only the operation and identity; values remain in Resolved.
type Change struct {
	Kind   Kind   `json:"kind"`
	Name   string `json:"name"`
	Action string `json:"action"`
}

type Prepared struct {
	Changes  []Change
	Resolved *Document
}

// Prepare is a read-only calculation. Authentication, revisions and atomic
// persistence are the responsibility of the installation transaction layer.
func Prepare(desired *Document, catalog Catalog, allowDelete bool) (*Prepared, error) {
	if desired == nil || catalog == nil {
		return nil, failure(CodeInvalidStructure, "$")
	}
	current := make(map[string]Resource)
	for _, r := range catalog.Resources() {
		current[string(r.Kind)+"\x00"+r.Metadata.Name] = r
	}
	out := &Prepared{Changes: make([]Change, 0, len(desired.Resources)), Resolved: &Document{APIVersion: desired.APIVersion, Kind: desired.Kind, Resources: make([]Resource, 0, len(desired.Resources))}}
	for i, resource := range desired.Resources {
		old, exists := current[string(resource.Kind)+"\x00"+resource.Metadata.Name]
		var previous *Resource
		if exists {
			previous = &old
		}
		resolved, err := Resolve(resource, previous)
		if err != nil {
			return nil, err
		}
		action := "create"
		if resource.State == Absent {
			if !allowDelete {
				return nil, failure("delete_not_allowed", resourcePath(i))
			}
			action = "delete"
			if !exists {
				action = "noop"
			}
		} else if exists {
			action = "update"
			// Resolve both representations to compare fully materialized values.
			old.Presence = nil
			before, e := json.Marshal(old)
			if e != nil {
				return nil, failure(CodeInvalidStructure, resourcePath(i))
			}
			after, e := json.Marshal(resolved)
			if e != nil {
				return nil, failure(CodeInvalidStructure, resourcePath(i))
			}
			if bytes.Equal(before, after) {
				action = "noop"
			}
		}
		out.Resolved.Resources = append(out.Resolved.Resources, resolved)
		out.Changes = append(out.Changes, Change{Kind: resource.Kind, Name: resource.Metadata.Name, Action: action})
	}
	if err := Validate(out.Resolved, catalog); err != nil {
		return nil, err
	}
	sort.Slice(out.Changes, func(i, j int) bool {
		if out.Changes[i].Kind == out.Changes[j].Kind {
			return out.Changes[i].Name < out.Changes[j].Name
		}
		return out.Changes[i].Kind < out.Changes[j].Kind
	})
	return out, nil
}
