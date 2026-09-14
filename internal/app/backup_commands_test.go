package app

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackupCLIRequiresOutputAndNeverAcceptsPassphraseFlag(t *testing.T) {
	for _, args := range [][]string{{"backup", "create"}, {"backup", "create", "--out", "x", "--passphrase", "exposed"}} {
		var stdout, stderr bytes.Buffer
		if code := run(context.Background(), args, strings.NewReader("strong-passphrase\n"), &stdout, &stderr, false); code != 2 {
			t.Fatalf("run(%v) code = %d, want 2; stderr=%s", args, code, stderr.String())
		}
	}
}

func TestBackupCLICreateAndVerifyFromStdin(t *testing.T) {
	dataDir := t.TempDir()
	backupPath := filepath.Join(t.TempDir(), "one.mcb.age")
	passphrase := "strong-backup-passphrase\n"
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"backup", "create", "--data-dir", dataDir, "--out", backupPath},
		strings.NewReader(passphrase), &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("create code=%d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = run(context.Background(), []string{"backup", "verify", backupPath}, strings.NewReader(passphrase), &stdout, &stderr, false)
	if code != 0 || !strings.Contains(stdout.String(), "backup verified") {
		t.Fatalf("verify code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}
