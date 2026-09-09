package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sxamx/modelcairn/internal/storage"
)

func TestSecretCLISetMetadataUpdateRotateDelete(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	args := func(command ...string) []string { return append([]string{"secret"}, command...) }
	code, out, errOut := runCLI(t, args("set", "--data-dir", dataDir, "api-key"), "first-secret\n", false)
	if code != 0 || strings.Contains(out, "first-secret") || errOut != "" {
		t.Fatalf("set code=%d out=%q err=%q", code, out, errOut)
	}
	var created storage.SecretMetadata
	if err := json.Unmarshal([]byte(out), &created); err != nil {
		t.Fatal(err)
	}
	if created.ResourceVersion != 1 || created.Fingerprint == "" {
		t.Fatalf("created=%+v", created)
	}
	code, out, errOut = runCLI(t, args("metadata", "--data-dir", dataDir, "api-key"), "", false)
	if code != 0 || strings.Contains(out, "first-secret") || errOut != "" {
		t.Fatalf("metadata code=%d out=%q err=%q", code, out, errOut)
	}
	code, out, errOut = runCLI(t, args("set", "--data-dir", dataDir, "--version", "1", "api-key"), "second-secret\n", false)
	if code != 0 || strings.Contains(out, "second-secret") {
		t.Fatalf("update code=%d out=%q err=%q", code, out, errOut)
	}
	code, out, errOut = runCLI(t, args("rotate", "--data-dir", dataDir), "", false)
	if code != 0 || !strings.Contains(out, "version 2") {
		t.Fatalf("rotate code=%d out=%q err=%q", code, out, errOut)
	}
	code, out, errOut = runCLI(t, args("delete", "--data-dir", dataDir, "--version", "2", "api-key"), "", false)
	if code != 0 || out != "secret deleted\n" {
		t.Fatalf("delete code=%d out=%q err=%q", code, out, errOut)
	}
}

func TestSecretCLIRejectsShortAndStaleValuesWithoutDisclosure(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	code, _, errOut := runCLI(t, []string{"secret", "set", "--data-dir", dataDir, "api-key"}, "short\n", false)
	if code != 2 || strings.Contains(errOut, "short") {
		t.Fatalf("short code=%d err=%q", code, errOut)
	}
	if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
		t.Fatalf("invalid input created installation: %v", err)
	}
	code, _, _ = runCLI(t, []string{"secret", "set", "--data-dir", dataDir, "api-key"}, "valid-secret\n", false)
	if code != 0 {
		t.Fatal("create failed")
	}
	code, _, errOut = runCLI(t, []string{"secret", "set", "--data-dir", dataDir, "--version", "2", "api-key"}, "never-stored-secret\n", false)
	if code != 3 || strings.Contains(errOut, "never-stored-secret") {
		t.Fatalf("stale code=%d err=%q", code, errOut)
	}
	installation, err := storage.OpenInstallation(context.Background(), dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer installation.Close()
	if err := installation.Secrets().Use(context.Background(), "api-key", func(value []byte) error {
		if string(value) != "valid-secret" {
			t.Fatal("stale write changed secret")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestSecretCLIInteractiveRequiresRealTerminal(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	code, _, errOut := runCLI(t, []string{"secret", "set", "--data-dir", dataDir, "api-key"}, "would-echo-secret", true)
	if code != 2 || !strings.Contains(errOut, "interactive_terminal_required") || strings.Contains(errOut, "would-echo-secret") {
		t.Fatalf("code=%d err=%q", code, errOut)
	}
}

func TestSecretCLIInputBoundaryRejectsTrailingData(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	maximum := strings.Repeat("x", 16384)
	code, _, errOut := runCLI(t, []string{"secret", "set", "--data-dir", dataDir, "api-key"}, maximum+"\r\n", false)
	if code != 0 || errOut != "" {
		t.Fatalf("maximum with CRLF code=%d err=%q", code, errOut)
	}

	extraDir := filepath.Join(t.TempDir(), "extra")
	code, _, errOut = runCLI(t, []string{"secret", "set", "--data-dir", extraDir, "api-key"}, maximum+"\nextra", false)
	if code != 2 || !strings.Contains(errOut, "invalid_secret") {
		t.Fatalf("trailing data code=%d err=%q", code, errOut)
	}
	if _, err := os.Stat(extraDir); !os.IsNotExist(err) {
		t.Fatalf("trailing data created installation: %v", err)
	}
}

func TestSecretCLIRedactsMetadataNames(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	secret := "same-secret-name"
	code, out, errOut := runCLI(t, []string{"secret", "set", "--data-dir", dataDir, secret}, secret+"\n", false)
	if code != 0 || strings.Contains(out, secret) || errOut != "" {
		t.Fatalf("set code=%d out=%q err=%q", code, out, errOut)
	}
	code, out, errOut = runCLI(t, []string{"secret", "metadata", "--data-dir", dataDir}, "", false)
	if code != 0 || strings.Contains(out, secret) || errOut != "" {
		t.Fatalf("metadata code=%d out=%q err=%q", code, out, errOut)
	}
}
