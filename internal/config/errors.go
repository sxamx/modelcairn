package config

import "fmt"

type Diagnostic struct {
	Code string `json:"code"`
	Path string `json:"path"`
}
type Error struct {
	Diagnostics []Diagnostic `json:"diagnostics"`
}

func (e *Error) Error() string {
	if len(e.Diagnostics) == 0 {
		return "invalid_configuration"
	}
	d := e.Diagnostics[0]
	return fmt.Sprintf("%s at %s", d.Code, d.Path)
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
	CodeDuplicateResource = "duplicate_resource"
	CodeReferenceNotFound = "reference_not_found"
	CodeProviderMismatch  = "provider_mismatch"
)
