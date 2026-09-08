package config

import (
	"bytes"
	"encoding/json"
	"sort"

	"github.com/sxamx/modelcairn/internal/redact"
)

// CanonicalJSON returns a stable, secret-free representation. Resources are
// ordered by kind and declarative name so input order cannot change its digest.
func CanonicalJSON(doc *Document, redactor *redact.Redactor) ([]byte, error) {
	clone := *doc
	clone.Resources = append([]Resource(nil), doc.Resources...)
	sort.Slice(clone.Resources, func(i, j int) bool {
		if clone.Resources[i].Kind == clone.Resources[j].Kind {
			return clone.Resources[i].Metadata.Name < clone.Resources[j].Metadata.Name
		}
		return clone.Resources[i].Kind < clone.Resources[j].Kind
	})
	encoded, err := json.Marshal(clone)
	if err != nil {
		return nil, err
	}
	if redactor != nil {
		decoder := json.NewDecoder(bytes.NewReader(encoded))
		decoder.UseNumber()
		var value any
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		encoded, err = json.Marshal(redactStrings(value, redactor))
		if err != nil {
			return nil, err
		}
	}
	var out bytes.Buffer
	if err := json.Indent(&out, encoded, "", "  "); err != nil {
		return nil, err
	}
	out.WriteByte('\n')
	return out.Bytes(), nil
}

func redactStrings(value any, redactor *redact.Redactor) any {
	switch current := value.(type) {
	case string:
		return redactor.String(current)
	case []any:
		for i := range current {
			current[i] = redactStrings(current[i], redactor)
		}
	case map[string]any:
		for key, item := range current {
			current[key] = redactStrings(item, redactor)
		}
	}
	return value
}
