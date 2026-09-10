// Package adminsettings parses and validates administrative settings without I/O.
package adminsettings

const (
	APIVersion    = "modelcairn.io/v1alpha1"
	DocumentKind  = "AdminSettings"
	MaxInputBytes = 64 << 10
	MaxDepth      = 16
)

type Transport string

const (
	LoopbackHTTP Transport = "loopback-http"
	DirectTLS    Transport = "direct-tls"
	ProxyTLS     Transport = "proxy-tls"
)

type Document struct {
	APIVersion      string `json:"apiVersion"`
	Kind            string `json:"kind"`
	ResourceVersion *int64 `json:"resourceVersion,omitempty"`
	Spec            Patch  `json:"spec"`
}
type Patch struct {
	PublicOrigin            *string    `json:"publicOrigin,omitempty"`
	Listen                  *string    `json:"listen,omitempty"`
	Transport               *Transport `json:"transport,omitempty"`
	TrustedProxyCIDRs       *[]string  `json:"trustedProxyCidrs,omitempty"`
	TLSCertificatePath      *string    `json:"tlsCertificatePath,omitempty"`
	TLSPrivateKeyPath       *string    `json:"tlsPrivateKeyPath,omitempty"`
	IdleSeconds             *int       `json:"idleSeconds,omitempty"`
	AbsoluteSeconds         *int       `json:"absoluteSeconds,omitempty"`
	GlobalAttemptsPerMinute *int       `json:"globalAttemptsPerMinute,omitempty"`
	GlobalBurst             *int       `json:"globalBurst,omitempty"`
	ClientAttemptsPerMinute *int       `json:"clientAttemptsPerMinute,omitempty"`
	ClientBurst             *int       `json:"clientBurst,omitempty"`
	MaxClientEntries        *int       `json:"maxClientEntries,omitempty"`
	ClientIdleSeconds       *int       `json:"clientIdleSeconds,omitempty"`
	ArgonMemoryKiB          *int       `json:"argonMemoryKiB,omitempty"`
	ArgonIterations         *int       `json:"argonIterations,omitempty"`
}
type Resolved struct {
	PublicOrigin            string    `json:"publicOrigin"`
	Listen                  string    `json:"listen"`
	Transport               Transport `json:"transport"`
	TrustedProxyCIDRs       []string  `json:"trustedProxyCidrs"`
	TLSCertificatePath      string    `json:"tlsCertificatePath"`
	TLSPrivateKeyPath       string    `json:"tlsPrivateKeyPath"`
	IdleSeconds             int       `json:"idleSeconds"`
	AbsoluteSeconds         int       `json:"absoluteSeconds"`
	GlobalAttemptsPerMinute int       `json:"globalAttemptsPerMinute"`
	GlobalBurst             int       `json:"globalBurst"`
	ClientAttemptsPerMinute int       `json:"clientAttemptsPerMinute"`
	ClientBurst             int       `json:"clientBurst"`
	MaxClientEntries        int       `json:"maxClientEntries"`
	ClientIdleSeconds       int       `json:"clientIdleSeconds"`
	ArgonMemoryKiB          int       `json:"argonMemoryKiB"`
	ArgonIterations         int       `json:"argonIterations"`
}

func Defaults() Resolved {
	return Resolved{Listen: "127.0.0.1:8080", Transport: LoopbackHTTP, TrustedProxyCIDRs: []string{}, IdleSeconds: 1800, AbsoluteSeconds: 43200, GlobalAttemptsPerMinute: 30, GlobalBurst: 5, ClientAttemptsPerMinute: 5, ClientBurst: 3, MaxClientEntries: 1024, ClientIdleSeconds: 900, ArgonMemoryKiB: 19456, ArgonIterations: 2}
}

type Diagnostic struct {
	Code string `json:"code"`
	Path string `json:"path"`
}
type Error struct {
	Diagnostics []Diagnostic `json:"diagnostics"`
}

func (e *Error) Error() string {
	if len(e.Diagnostics) == 0 {
		return "invalid_admin_settings"
	}
	return e.Diagnostics[0].Code + " at " + e.Diagnostics[0].Path
}
func failure(code, path string) error {
	return &Error{Diagnostics: []Diagnostic{{Code: code, Path: path}}}
}

const (
	CodeInputTooLarge     = "input_too_large"
	CodeEmptyDocument     = "empty_document"
	CodeMultipleDocuments = "multiple_documents"
	CodeAliasNotAllowed   = "alias_not_allowed"
	CodeTagNotAllowed     = "tag_not_allowed"
	CodeDuplicateKey      = "duplicate_key"
	CodeNonStringKey      = "non_string_key"
	CodeDepthExceeded     = "depth_exceeded"
	CodeInvalidStructure  = "invalid_structure"
	CodeInvalidValue      = "invalid_value"
)
