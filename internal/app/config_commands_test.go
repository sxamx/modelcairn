package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sxamx/modelcairn/internal/config"
	"github.com/sxamx/modelcairn/internal/storage"
)

const cliConfig = `apiVersion: modelcairn.io/v1alpha1
kind: Configuration
resources:
- kind: Provider
  metadata: {name: provider}
  spec: {}
`

func writeCLIConfig(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
func runCLI(t *testing.T, args []string, input string, interactive bool) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), args, strings.NewReader(input), &stdout, &stderr, interactive)
	return code, stdout.String(), stderr.String()
}

func TestConfigValidateDoesNotCreateInstallation(t *testing.T) {
	root := t.TempDir()
	configPath := writeCLIConfig(t, root, cliConfig)
	dataDir := filepath.Join(root, "must-not-exist")
	code, out, errOut := runCLI(t, []string{"config", "validate", configPath}, "", false)
	if code != 0 || out != "configuration valid\n" || errOut != "" {
		t.Fatalf("code=%d out=%q err=%q", code, out, errOut)
	}
	if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
		t.Fatalf("validate created state: %v", err)
	}
}

func TestConfigPlanApplyExportAndSingleUse(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	configPath := writeCLIConfig(t, root, cliConfig)
	planPath := filepath.Join(root, "plan.json")
	code, _, errOut := runCLI(t, []string{"config", "plan", "--data-dir", dataDir, "--out", planPath, configPath}, "", false)
	if code != 0 {
		t.Fatalf("plan code=%d err=%q", code, errOut)
	}
	info, err := os.Stat(planPath)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("plan permissions=%o", info.Mode().Perm())
	}
	code, out, errOut := runCLI(t, []string{"config", "apply", "--data-dir", dataDir, "--plan", planPath, configPath}, "", false)
	if code != 0 || out != "configuration applied\n" {
		t.Fatalf("apply code=%d out=%q err=%q", code, out, errOut)
	}
	code, _, errOut = runCLI(t, []string{"config", "apply", "--data-dir", dataDir, "--plan", planPath, configPath}, "", false)
	if code != 4 || !strings.Contains(errOut, "plan_already_used") {
		t.Fatalf("reuse code=%d err=%q", code, errOut)
	}
	code, out, errOut = runCLI(t, []string{"config", "export", "--data-dir", dataDir}, "", false)
	if code != 0 {
		t.Fatalf("export code=%d err=%q", code, errOut)
	}
	doc, err := config.Parse([]byte(out))
	if err != nil || len(doc.Resources) != 1 {
		t.Fatalf("export parse=%v resources=%v", err, len(doc.Resources))
	}
}

func TestConfigApplyAutomationRequiresPlanBeforeOpeningState(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "state")
	path := writeCLIConfig(t, root, cliConfig)
	code, _, errOut := runCLI(t, []string{"config", "apply", "--data-dir", dataDir, path}, "", false)
	if code != 2 || !strings.Contains(errOut, "plan file is required") {
		t.Fatalf("code=%d err=%q", code, errOut)
	}
	if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
		t.Fatalf("missing-plan apply created state: %v", err)
	}
}

func TestConfigInteractiveApplyConfirmsUnderOneOwner(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	path := writeCLIConfig(t, root, cliConfig)
	code, out, errOut := runCLI(t, []string{"config", "apply", "--data-dir", dataDir, path}, "yes\n", true)
	if code != 0 || !strings.Contains(out, "create Provider/provider") || !strings.Contains(out, "configuration applied") {
		t.Fatalf("code=%d out=%q err=%q", code, out, errOut)
	}
	installation, err := storage.OpenInstallation(context.Background(), dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer installation.Close()
	items, err := storage.NewRepository(installation.DB()).List(context.Background())
	if err != nil || len(items) != 1 {
		t.Fatalf("items=%d err=%v", len(items), err)
	}
}

func TestConfigInteractiveCancellationDoesNotApply(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	path := writeCLIConfig(t, root, cliConfig)
	code, out, errOut := runCLI(t, []string{"config", "apply", "--data-dir", dataDir, path}, "no\n", true)
	if code != 0 || !strings.Contains(out, "cancelled") || errOut != "" {
		t.Fatalf("code=%d out=%q err=%q", code, out, errOut)
	}
	installation, err := storage.OpenInstallation(context.Background(), dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer installation.Close()
	items, err := storage.NewRepository(installation.DB()).List(context.Background())
	if err != nil || len(items) != 0 {
		t.Fatalf("items=%d err=%v", len(items), err)
	}
}

func TestConfigPlanFileIsExclusiveAndTamperingHasStableExit(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	path := writeCLIConfig(t, root, cliConfig)
	planPath := filepath.Join(root, "plan.json")
	if err := os.WriteFile(planPath, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, _ := runCLI(t, []string{"config", "plan", "--data-dir", dataDir, "--out", planPath, path}, "", false)
	if code != 1 {
		t.Fatalf("overwrite code=%d", code)
	}
	raw, _ := os.ReadFile(planPath)
	if string(raw) != "keep" {
		t.Fatal("existing plan overwritten")
	}
	bad := filepath.Join(root, "bad-plan.json")
	invalidDataDir := filepath.Join(root, "invalid-plan-state")
	if err := os.WriteFile(bad, []byte(`{"token":"tampered","unknown":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := runCLI(t, []string{"config", "apply", "--data-dir", invalidDataDir, "--plan", bad, path}, "", false)
	if code != 4 || strings.TrimSpace(errOut) != "invalid_plan" {
		t.Fatalf("code=%d err=%q", code, errOut)
	}
	if _, err := os.Stat(invalidDataDir); !os.IsNotExist(err) {
		t.Fatalf("invalid plan created state: %v", err)
	}
}

func TestConfigVersionConflictHasStableExit(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	initial := writeCLIConfig(t, root, cliConfig)
	code, _, errOut := runCLI(t, []string{"config", "apply", "--data-dir", dataDir, initial}, "yes\n", true)
	if code != 0 {
		t.Fatalf("initial apply code=%d err=%q", code, errOut)
	}
	stale := strings.Replace(cliConfig, "metadata: {name: provider}", "metadata: {name: provider, resourceVersion: 99}", 1)
	stalePath := filepath.Join(root, "stale.yaml")
	if err := os.WriteFile(stalePath, []byte(stale), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, errOut = runCLI(t, []string{"config", "apply", "--data-dir", dataDir, stalePath}, "yes\n", true)
	if code != 3 || !strings.Contains(errOut, "version_conflict") {
		t.Fatalf("stale apply code=%d err=%q", code, errOut)
	}
}
