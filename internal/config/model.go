// Package config parses, validates, and canonically exports ModelCairn's
// declarative configuration without performing persistence or network I/O.
package config

import "encoding/json"

const (
	APIVersion    = "modelcairn.io/v1alpha1"
	DocumentKind  = "Configuration"
	MaxInputBytes = 8 << 20
	MaxDepth      = 64
)

type Document struct {
	APIVersion string     `json:"apiVersion"`
	Kind       string     `json:"kind"`
	Resources  []Resource `json:"resources"`
}

type Resource struct {
	Kind     Kind
	State    State
	Metadata Metadata
	Spec     any
	Presence map[string]FieldPresence
}

type FieldPresence uint8

const (
	FieldOmitted FieldPresence = iota
	FieldValue
	FieldNull
)

func (r Resource) FieldPresence(name string) FieldPresence { return r.Presence[name] }

type Kind string

const (
	ProviderKind           Kind = "Provider"
	ProviderAccountKind    Kind = "ProviderAccount"
	ProviderConnectionKind Kind = "ProviderConnection"
	CredentialKind         Kind = "Credential"
	EgressKind             Kind = "Egress"
	ModelKind              Kind = "Model"
	DestinationKind        Kind = "Destination"
	StrategyKind           Kind = "Strategy"
	RouteKind              Kind = "Route"
	AgentTokenKind         Kind = "AgentToken"
)

type State string

const (
	Present State = "present"
	Absent  State = "absent"
)

type Metadata struct {
	Name            string  `json:"name"`
	DisplayName     *string `json:"displayName,omitempty"`
	Description     *string `json:"description,omitempty"`
	UID             *string `json:"uid,omitempty"`
	ResourceVersion *int64  `json:"resourceVersion,omitempty"`
}
type Ref struct {
	Name string `json:"name"`
}
type ProviderSpec struct{}
type ProviderAccountSpec struct {
	ProviderRef Ref `json:"providerRef"`
}
type ProviderConnectionSpec struct {
	ProviderRef         Ref    `json:"providerRef"`
	BaseURL             string `json:"baseUrl"`
	Adapter             string `json:"adapter"`
	AllowPrivateNetwork bool   `json:"allowPrivateNetwork"`
	Enabled             bool   `json:"enabled"`
}
type CredentialSpec struct {
	ProviderAccountRef Ref  `json:"providerAccountRef"`
	EgressRef          Ref  `json:"egressRef"`
	SecretRef          Ref  `json:"secretRef"`
	Enabled            bool `json:"enabled"`
}
type EgressSpec struct {
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
}
type ModelSpec struct {
	ConnectionRef   Ref      `json:"connectionRef"`
	ProviderModelID string   `json:"providerModelId"`
	Capabilities    []string `json:"capabilities"`
	Enabled         bool     `json:"enabled"`
}
type DestinationSpec struct {
	ModelRef      Ref  `json:"modelRef"`
	CredentialRef Ref  `json:"credentialRef"`
	Enabled       bool `json:"enabled"`
	Weight        int  `json:"weight"`
}
type StrategySpec struct {
	Destinations     []Ref `json:"destinations"`
	MaxAttempts      int   `json:"maxAttempts"`
	AttemptTimeoutMS int   `json:"attemptTimeoutMs"`
	TotalTimeoutMS   int   `json:"totalTimeoutMs"`
}
type RouteSpec struct {
	ModelAlias  string `json:"modelAlias"`
	StrategyRef Ref    `json:"strategyRef"`
	Enabled     bool   `json:"enabled"`
}
type AgentTokenSpec struct {
	AllowedRouteRefs []Ref   `json:"allowedRouteRefs"`
	ExpiresAt        *string `json:"expiresAt"`
	Enabled          bool    `json:"enabled"`
}

type rawDocument struct {
	APIVersion string             `json:"apiVersion"`
	Kind       string             `json:"kind"`
	Resources  *[]json.RawMessage `json:"resources"`
}
type rawResource struct {
	Kind     Kind            `json:"kind"`
	State    State           `json:"state,omitempty"`
	Metadata json.RawMessage `json:"metadata"`
	Spec     json.RawMessage `json:"spec,omitempty"`
}
