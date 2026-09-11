package httpserver

import (
	"net/http/httptest"
	"testing"
)

func TestIfMatchRequiresCanonicalPositiveVersion(t *testing.T) {
	for _, value := range []string{`"+1"`, `"-1"`, `"01"`, `"0"`, `W/"1"`, `*`, `"1", "2"`, `"9223372036854775808"`} {
		r := httptest.NewRequest("PUT", "/", nil)
		r.Header.Set("If-Match", value)
		if _, _, err := optionalIfMatch(r); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
	r := httptest.NewRequest("PUT", "/", nil)
	r.Header.Set("If-Match", `"9223372036854775807"`)
	if version, present, err := optionalIfMatch(r); err != nil || !present || version != 9223372036854775807 {
		t.Fatal("rejected canonical version")
	}
	r.Header.Add("If-Match", `"2"`)
	if _, _, err := optionalIfMatch(r); err == nil {
		t.Fatal("accepted duplicate precondition")
	}
}
