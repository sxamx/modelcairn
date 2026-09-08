package config

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func decodeResource(data []byte, path string) (Resource, error) {
	var raw rawResource
	if err := decodeStrict(data, &raw); err != nil {
		return Resource{}, failure(CodeInvalidStructure, path)
	}
	var metadata Metadata
	if len(raw.Metadata) == 0 || decodeStrict(raw.Metadata, &metadata) != nil {
		return Resource{}, failure(CodeInvalidStructure, path+".metadata")
	}
	if raw.State == "" {
		raw.State = Present
	}
	result := Resource{Kind: raw.Kind, State: raw.State, Metadata: metadata, Presence: make(map[string]FieldPresence)}
	if raw.State == Absent {
		if len(raw.Spec) > 0 || metadata.DisplayName != nil || metadata.Description != nil {
			return Resource{}, failure(CodeInvalidStructure, path)
		}
		if !knownKind(raw.Kind) {
			return Resource{}, failure(CodeInvalidValue, path+".kind")
		}
		return result, nil
	}
	if raw.State != Present {
		return Resource{}, failure(CodeInvalidValue, path+".state")
	}
	if len(raw.Spec) == 0 {
		return Resource{}, failure(CodeInvalidStructure, path+".spec")
	}
	if bytes.Equal(bytes.TrimSpace(raw.Spec), []byte("null")) {
		return Resource{}, failure(CodeInvalidStructure, path+".spec")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw.Spec, &fields); err != nil || fields == nil {
		return Resource{}, failure(CodeInvalidStructure, path+".spec")
	}
	for name, value := range fields {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			result.Presence[name] = FieldNull
		} else {
			result.Presence[name] = FieldValue
		}
	}
	var err error
	switch raw.Kind {
	case ProviderKind:
		var v ProviderSpec
		err = decodeStrict(raw.Spec, &v)
		result.Spec = v
	case ProviderAccountKind:
		var v ProviderAccountSpec
		err = decodeStrict(raw.Spec, &v)
		result.Spec = v
	case ProviderConnectionKind:
		var v struct {
			ProviderRef         Ref    `json:"providerRef"`
			BaseURL             string `json:"baseUrl"`
			Adapter             string `json:"adapter"`
			AllowPrivateNetwork *bool  `json:"allowPrivateNetwork,omitempty"`
			Enabled             *bool  `json:"enabled,omitempty"`
		}
		err = decodeStrict(raw.Spec, &v)
		if err == nil {
			result.Spec = ProviderConnectionSpec{ProviderRef: v.ProviderRef, BaseURL: v.BaseURL, Adapter: v.Adapter, AllowPrivateNetwork: valueOr(v.AllowPrivateNetwork, false), Enabled: valueOr(v.Enabled, true)}
		}
	case CredentialKind:
		var v struct {
			ProviderAccountRef Ref   `json:"providerAccountRef"`
			EgressRef          Ref   `json:"egressRef"`
			SecretRef          Ref   `json:"secretRef"`
			Enabled            *bool `json:"enabled,omitempty"`
		}
		err = decodeStrict(raw.Spec, &v)
		if err == nil {
			result.Spec = CredentialSpec{ProviderAccountRef: v.ProviderAccountRef, EgressRef: v.EgressRef, SecretRef: v.SecretRef, Enabled: valueOr(v.Enabled, true)}
		}
	case EgressKind:
		var v struct {
			Type    string `json:"type"`
			Enabled *bool  `json:"enabled,omitempty"`
		}
		err = decodeStrict(raw.Spec, &v)
		if err == nil {
			result.Spec = EgressSpec{Type: v.Type, Enabled: valueOr(v.Enabled, true)}
		}
	case ModelKind:
		var v struct {
			ConnectionRef   Ref       `json:"connectionRef"`
			ProviderModelID string    `json:"providerModelId"`
			Capabilities    *[]string `json:"capabilities"`
			Enabled         *bool     `json:"enabled,omitempty"`
		}
		err = decodeStrict(raw.Spec, &v)
		if err == nil {
			if v.Capabilities == nil || result.Presence["capabilities"] == FieldNull {
				return Resource{}, failure(CodeInvalidStructure, path+".spec.capabilities")
			}
			result.Spec = ModelSpec{ConnectionRef: v.ConnectionRef, ProviderModelID: v.ProviderModelID, Capabilities: *v.Capabilities, Enabled: valueOr(v.Enabled, true)}
		}
	case DestinationKind:
		var v struct {
			ModelRef      Ref   `json:"modelRef"`
			CredentialRef Ref   `json:"credentialRef"`
			Enabled       *bool `json:"enabled,omitempty"`
			Weight        *int  `json:"weight,omitempty"`
		}
		err = decodeStrict(raw.Spec, &v)
		if err == nil {
			result.Spec = DestinationSpec{ModelRef: v.ModelRef, CredentialRef: v.CredentialRef, Enabled: valueOr(v.Enabled, true), Weight: valueOr(v.Weight, 100)}
		}
	case StrategyKind:
		var v struct {
			Destinations     *[]Ref `json:"destinations"`
			MaxAttempts      *int   `json:"maxAttempts,omitempty"`
			AttemptTimeoutMS *int   `json:"attemptTimeoutMs,omitempty"`
			TotalTimeoutMS   *int   `json:"totalTimeoutMs,omitempty"`
		}
		err = decodeStrict(raw.Spec, &v)
		if err == nil {
			if v.Destinations == nil || result.Presence["destinations"] == FieldNull {
				return Resource{}, failure(CodeInvalidStructure, path+".spec.destinations")
			}
			result.Spec = StrategySpec{Destinations: *v.Destinations, MaxAttempts: valueOr(v.MaxAttempts, 3), AttemptTimeoutMS: valueOr(v.AttemptTimeoutMS, 60000), TotalTimeoutMS: valueOr(v.TotalTimeoutMS, 120000)}
		}
	case RouteKind:
		var v struct {
			ModelAlias  string `json:"modelAlias"`
			StrategyRef Ref    `json:"strategyRef"`
			Enabled     *bool  `json:"enabled,omitempty"`
		}
		err = decodeStrict(raw.Spec, &v)
		if err == nil {
			result.Spec = RouteSpec{ModelAlias: v.ModelAlias, StrategyRef: v.StrategyRef, Enabled: valueOr(v.Enabled, true)}
		}
	case AgentTokenKind:
		var v struct {
			AllowedRouteRefs *[]Ref  `json:"allowedRouteRefs"`
			ExpiresAt        *string `json:"expiresAt"`
			Enabled          *bool   `json:"enabled,omitempty"`
		}
		err = decodeStrict(raw.Spec, &v)
		if err == nil {
			if v.AllowedRouteRefs == nil || result.Presence["allowedRouteRefs"] == FieldNull {
				return Resource{}, failure(CodeInvalidStructure, path+".spec.allowedRouteRefs")
			}
			result.Spec = AgentTokenSpec{AllowedRouteRefs: *v.AllowedRouteRefs, ExpiresAt: v.ExpiresAt, Enabled: valueOr(v.Enabled, true)}
		}
	default:
		return Resource{}, failure(CodeInvalidValue, path+".kind")
	}
	if err != nil {
		return Resource{}, failure(CodeInvalidStructure, path+".spec")
	}
	return result, nil
}

func valueOr[T any](value *T, fallback T) T {
	if value == nil {
		return fallback
	}
	return *value
}
func knownKind(kind Kind) bool {
	switch kind {
	case ProviderKind, ProviderAccountKind, ProviderConnectionKind, CredentialKind, EgressKind, ModelKind, DestinationKind, StrategyKind, RouteKind, AgentTokenKind:
		return true
	}
	return false
}

func (r Resource) MarshalJSON() ([]byte, error) {
	m := map[string]any{"kind": r.Kind, "state": r.State, "metadata": r.Metadata}
	if r.State == Present {
		encoded, err := json.Marshal(r.Spec)
		if err != nil {
			return nil, err
		}
		var spec map[string]any
		if err := json.Unmarshal(encoded, &spec); err != nil {
			return nil, err
		}
		for _, name := range optionalFields(r.Kind) {
			if r.Presence == nil {
				break
			}
			switch r.Presence[name] {
			case FieldOmitted:
				delete(spec, name)
			case FieldNull:
				spec[name] = nil
			}
		}
		m["spec"] = spec
	}
	return json.Marshal(m)
}

func optionalFields(kind Kind) []string {
	switch kind {
	case ProviderConnectionKind:
		return []string{"allowPrivateNetwork", "enabled"}
	case CredentialKind, EgressKind, ModelKind, RouteKind:
		return []string{"enabled"}
	case DestinationKind:
		return []string{"enabled", "weight"}
	case StrategyKind:
		return []string{"maxAttempts", "attemptTimeoutMs", "totalTimeoutMs"}
	case AgentTokenKind:
		return []string{"expiresAt", "enabled"}
	default:
		return nil
	}
}
func (r *Resource) UnmarshalJSON([]byte) error { return fmt.Errorf("use config.Parse") }
