package config

import "encoding/json"

// Resolve materializes a declarative resource against its current persisted
// value. The caller must validate the resulting document and effective graph
// before committing it. Neither argument is mutated.
func Resolve(desired Resource, current *Resource) (Resource, error) {
	if current != nil {
		if desired.Kind != current.Kind || desired.Metadata.Name != current.Metadata.Name {
			return Resource{}, failure("version_conflict", "$.metadata")
		}
		if desired.Metadata.UID != nil && (current.Metadata.UID == nil || *desired.Metadata.UID != *current.Metadata.UID) {
			return Resource{}, failure("version_conflict", "$.metadata.uid")
		}
		if desired.Metadata.ResourceVersion != nil && (current.Metadata.ResourceVersion == nil || *desired.Metadata.ResourceVersion != *current.Metadata.ResourceVersion) {
			return Resource{}, failure("version_conflict", "$.metadata.resourceVersion")
		}
	}
	result := desired
	if current == nil {
		result.Metadata.UID = nil
		result.Metadata.ResourceVersion = nil
	} else {
		result.Metadata.UID = current.Metadata.UID
		result.Metadata.ResourceVersion = current.Metadata.ResourceVersion
	}
	if desired.State == Absent {
		return result, nil
	}
	if current != nil {
		if result.Metadata.DisplayName == nil {
			result.Metadata.DisplayName = current.Metadata.DisplayName
		}
		if result.Metadata.Description == nil {
			result.Metadata.Description = current.Metadata.Description
		}
	}
	encoded, err := json.Marshal(desired.Spec)
	if err != nil {
		return Resource{}, failure(CodeInvalidStructure, "$.spec")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil || fields == nil {
		return Resource{}, failure(CodeInvalidStructure, "$.spec")
	}
	if current != nil && desired.Presence != nil {
		previous, err := json.Marshal(current.Spec)
		if err != nil {
			return Resource{}, failure(CodeInvalidStructure, "$.spec")
		}
		var oldFields map[string]json.RawMessage
		if err := json.Unmarshal(previous, &oldFields); err != nil {
			return Resource{}, failure(CodeInvalidStructure, "$.spec")
		}
		for _, name := range optionalFields(desired.Kind) {
			if desired.FieldPresence(name) == FieldOmitted {
				if value, ok := oldFields[name]; ok {
					fields[name] = value
				}
			}
		}
	}
	// decodeResource reconstructs the concrete spec type and explicit presence.
	data, err := json.Marshal(map[string]any{"kind": result.Kind, "state": result.State, "metadata": result.Metadata, "spec": fields})
	if err != nil {
		return Resource{}, failure(CodeInvalidStructure, "$")
	}
	return decodeResource(data, "$")
}
