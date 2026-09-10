package adminsettings

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// DecodeCanonicalJSON accepts only the exact resolved representation produced by
// CanonicalJSON. It is intended for persisted trusted settings and fails closed on
// missing/unknown fields, duplicate-key drift, noncanonical order, or bad values.
func DecodeCanonicalJSON(data []byte) (Resolved, error) {
	if len(data) > MaxInputBytes {
		return Resolved{}, failure(CodeInputTooLarge, "$")
	}
	var result Resolved
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return Resolved{}, failure(CodeInvalidStructure, "$")
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return Resolved{}, failure(CodeInvalidStructure, "$")
	}
	if err := Validate(result); err != nil {
		return Resolved{}, err
	}
	canonical, err := CanonicalJSON(result)
	if err != nil || !bytes.Equal(data, canonical) {
		return Resolved{}, failure(CodeInvalidStructure, "$")
	}
	return result, nil
}
