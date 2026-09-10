package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sxamx/modelcairn/internal/adminauth"
	"github.com/sxamx/modelcairn/internal/storage"
)

func writeSettingsFixture(t *testing.T, path string) {
	t.Helper()
	data := []byte("apiVersion: modelcairn.io/v1alpha1\nkind: AdminSettings\nspec:\n  publicOrigin: http://127.0.0.1:8080\n")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestAdminCLIBootstrapAndReset(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	settings := filepath.Join(t.TempDir(), "settings.yaml")
	writeSettingsFixture(t, settings)
	code, out, errOut := runCLI(t, []string{"admin", "bootstrap", "--data-dir", dataDir, "--username", "owner", "--settings", settings}, "initial password\n", false)
	if code != 0 || !strings.Contains(out, "created") || errOut != "" {
		t.Fatalf("bootstrap code=%d out=%q err=%q", code, out, errOut)
	}
	code, _, errOut = runCLI(t, []string{"admin", "bootstrap", "--data-dir", dataDir, "--username", "other", "--settings", settings}, "another password\n", false)
	if code != 3 || !strings.Contains(errOut, storage.CodeAlreadyExists) {
		t.Fatalf("second code=%d err=%q", code, errOut)
	}
	code, out, errOut = runCLI(t, []string{"admin", "reset-password", "--data-dir", dataDir}, "replacement password\r\n", false)
	if code != 0 || !strings.Contains(out, "sessions revoked") || errOut != "" {
		t.Fatalf("reset code=%d out=%q err=%q", code, out, errOut)
	}
	i, err := storage.OpenInstallation(context.Background(), dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer i.Close()
	var phc string
	if err := i.DB().QueryRow("SELECT password_phc FROM admin_users").Scan(&phc); err != nil {
		t.Fatal(err)
	}
	if ok, err := adminauth.Verify([]byte("replacement password"), phc); err != nil || !ok {
		t.Fatal("reset password unavailable")
	}
}

func TestAdminCLIValidatesBeforeCreatingInstallation(t *testing.T) {
	base := t.TempDir()
	dataDir := filepath.Join(base, "data")
	settings := filepath.Join(base, "settings.yaml")
	writeSettingsFixture(t, settings)
	code, _, errOut := runCLI(t, []string{"admin", "bootstrap", "--data-dir", dataDir, "--username", "owner", "--settings", settings}, "short\n", false)
	if code != 2 || strings.Contains(errOut, "short") {
		t.Fatalf("code=%d err=%q", code, errOut)
	}
	if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
		t.Fatalf("invalid password created installation: %v", err)
	}
	badDir := filepath.Join(base, "bad-data")
	badSettings := filepath.Join(base, "bad.yaml")
	if err := os.WriteFile(badSettings, []byte("password: should-not-appear\n"), 0600); err != nil {
		t.Fatal(err)
	}
	code, _, errOut = runCLI(t, []string{"admin", "bootstrap", "--data-dir", badDir, "--username", "owner", "--settings", badSettings}, "valid password\n", false)
	if code != 2 || strings.Contains(errOut, "should-not-appear") {
		t.Fatalf("code=%d err=%q", code, errOut)
	}
	if _, err := os.Stat(badDir); !os.IsNotExist(err) {
		t.Fatalf("invalid settings created installation: %v", err)
	}
}

func TestAdminCLIInteractiveRequiresTerminal(t *testing.T) {
	settings := filepath.Join(t.TempDir(), "settings.yaml")
	writeSettingsFixture(t, settings)
	code, _, errOut := runCLI(t, []string{"admin", "bootstrap", "--data-dir", filepath.Join(t.TempDir(), "data"), "--username", "owner", "--settings", settings}, "visible password", true)
	if code != 2 || !strings.Contains(errOut, "interactive_terminal_required") || strings.Contains(errOut, "visible password") {
		t.Fatalf("code=%d err=%q", code, errOut)
	}
}
