package redact

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func TestRedactorHandlesOverlapReferencesAndRemoval(t *testing.T) {
	r := New()
	removeShort, err := r.Register([]byte("secret-1"))
	if err != nil {
		t.Fatal(err)
	}
	removeLong, err := r.Register([]byte("secret-1-suffix"))
	if err != nil {
		t.Fatal(err)
	}
	removeDuplicate, err := r.Register([]byte("secret-1"))
	if err != nil {
		t.Fatal(err)
	}
	if got := r.String("a secret-1-suffix and secret-1"); got != "a [REDACTED] and [REDACTED]" {
		t.Fatalf("redacted = %q", got)
	}
	removeShort()
	if strings.Contains(r.String("secret-1"), "secret-1") {
		t.Fatal("duplicate registration was removed too early")
	}
	removeDuplicate()
	if got := r.String("secret-1"); got != "secret-1" {
		t.Fatalf("removed value still redacted: %q", got)
	}
	removeLong()
	removeLong()
}

func TestSlogHandlerRedactsMessagesAndStructuredValues(t *testing.T) {
	r := New()
	_, err := r.Register([]byte("canary-secret"))
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	logger := slog.New(NewHandler(slog.NewJSONHandler(&output, nil), r)).With("bound", "canary-secret")
	logger.Error("message canary-secret", "string", "canary-secret", "bytes", []byte("canary-secret"), "error", errors.New("canary-secret"), "map", map[string]string{"nested": "canary-secret"}, "group", slog.GroupValue(slog.String("nested", "canary-secret")))
	if strings.Contains(output.String(), "canary-secret") || strings.Count(output.String(), Replacement) != 7 {
		t.Fatalf("unsafe log output: %s", output.String())
	}
}

func TestRegisterRejectsValuesOutsideSecretBounds(t *testing.T) {
	r := New()
	for _, value := range [][]byte{nil, bytes.Repeat([]byte{'x'}, 7), bytes.Repeat([]byte{'x'}, 16385)} {
		if _, err := r.Register(value); !errors.Is(err, ErrInvalidValue) {
			t.Fatalf("registration error = %v", err)
		}
	}
}

func TestBoundAttributesAreRedactedAtEmissionTime(t *testing.T) {
	r := New()
	var output bytes.Buffer
	logger := slog.New(NewHandler(slog.NewJSONHandler(&output, nil), r)).With("before", "future-secret").WithGroup("nested").With("after", "future-secret")
	if _, err := r.Register([]byte("future-secret")); err != nil {
		t.Fatal(err)
	}
	logger.Info("event")
	if strings.Contains(output.String(), "future-secret") || strings.Count(output.String(), Replacement) != 2 {
		t.Fatalf("unsafe late-bound output: %s", output.String())
	}
}

func TestSlogHandlerRedactsAttributeAndGroupNames(t *testing.T) {
	r := New()
	if _, err := r.Register([]byte("future-secret")); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	logger := slog.New(NewHandler(slog.NewJSONHandler(&output, nil), r)).WithGroup("future-secret")
	logger.Info("event", "future-secret", "safe")
	if strings.Contains(output.String(), "future-secret") || strings.Count(output.String(), Replacement) != 2 {
		t.Fatalf("unsafe log names: %s", output.String())
	}
}
