package storage

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func seedRotation(t *testing.T, dir string) (*Installation, []SecretMetadata) {
	t.Helper()
	i, err := OpenInstallation(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	var before []SecretMetadata
	for _, name := range []string{"alpha", "beta"} {
		m, err := i.Secrets().Put(context.Background(), PutSecret{Name: name, Value: []byte("rotation-canary-" + name)}, Actor{Type: "cli"})
		if err != nil {
			i.Close()
			t.Fatal(err)
		}
		before = append(before, m)
	}
	return i, before
}

func checkRotationValues(t *testing.T, i *Installation, before []SecretMetadata, rotated bool) {
	t.Helper()
	for _, old := range before {
		got, err := i.Secrets().GetMetadata(context.Background(), old.Name)
		if err != nil {
			t.Fatal(err)
		}
		if got.ResourceVersion != old.ResourceVersion || !got.CreatedAt.Equal(old.CreatedAt) || !got.UpdatedAt.Equal(old.UpdatedAt) {
			t.Fatal("rotation changed logical identity")
		}
		if (got.Fingerprint != old.Fingerprint) != rotated {
			t.Fatal("fingerprint does not match committed generation")
		}
		if got.KeyVersion != i.Secrets().ActiveKeyVersion() {
			t.Fatal("mixed key generations")
		}
		if err := i.Secrets().Use(context.Background(), old.Name, func(p []byte) error {
			if !bytes.Equal(p, []byte("rotation-canary-"+old.Name)) {
				return fmt.Errorf("plaintext mismatch")
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMissingActiveKeyDisablesStoreDuringPreflight(t *testing.T) {
	for _, operation := range []string{"rotate", "collect"} {
		t.Run(operation, func(t *testing.T) {
			i, _ := seedRotation(t, t.TempDir())
			defer i.Close()
			s := i.Secrets()
			if err := os.Remove(filepath.Join(s.keyring.dir, "v1.key")); err != nil {
				t.Fatal(err)
			}
			var err error
			if operation == "rotate" {
				_, err = s.RotateMasterKey(context.Background(), Actor{Type: "cli"})
			} else {
				err = s.CollectUnusedKeys(context.Background())
			}
			if err == nil {
				t.Fatal("missing active key accepted")
			}
			if _, err := s.Put(context.Background(), PutSecret{Name: "new", Value: []byte("another-canary")}, Actor{Type: "cli"}); err != errKeyMaterialUnavailable {
				t.Fatal("store allowed writes after detecting missing key")
			}
			if err := s.Use(context.Background(), "alpha", func([]byte) error { t.Fatal("callback ran after missing key"); return nil }); err != errKeyMaterialUnavailable {
				t.Fatal("store allowed use after detecting missing key")
			}
			if err := s.Delete(context.Background(), "alpha", 1, Actor{Type: "cli"}); err != errKeyMaterialUnavailable {
				t.Fatal("store allowed deletion after detecting missing key")
			}
		})
	}
}

func TestRotationCommitsAllSecretsAndCollectsOldKey(t *testing.T) {
	dir := t.TempDir()
	i, before := seedRotation(t, dir)
	version, err := i.Secrets().RotateMasterKey(context.Background(), Actor{Type: "cli"})
	if err != nil || version != 2 {
		t.Fatalf("rotation version=%d error=%v", version, err)
	}
	checkRotationValues(t, i, before, true)
	if _, err := os.Stat(filepath.Join(dir, keyringName, "v1.key")); !os.IsNotExist(err) {
		t.Fatal("old key retained after successful collection")
	}
	if err := i.Close(); err != nil {
		t.Fatal(err)
	}
	i, err = OpenInstallation(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer i.Close()
	checkRotationValues(t, i, before, true)
}

func TestRotationAuditFailureRollsBackCiphertextAndActiveVersion(t *testing.T) {
	i, before := seedRotation(t, t.TempDir())
	defer i.Close()
	_, err := i.DB().Exec(`CREATE TRIGGER fail_rotation_audit BEFORE INSERT ON audit_events WHEN NEW.action='secret.rotate' BEGIN SELECT RAISE(ABORT,'audit_unavailable'); END`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := i.Secrets().RotateMasterKey(context.Background(), Actor{Type: "cli"}); err == nil {
		t.Fatal("accepted failed audit")
	}
	checkRotationValues(t, i, before, false)
	if i.Secrets().ActiveKeyVersion() != 1 {
		t.Fatal("active version changed on rollback")
	}
}

func TestRotationProcessHelper(t *testing.T) {
	if os.Getenv("MODELCAIRN_ROTATION_HELPER") != "1" {
		return
	}
	i, err := OpenInstallation(context.Background(), os.Getenv("MODELCAIRN_ROTATION_DIR"))
	if err != nil {
		t.Fatal(err)
	}
	i.Secrets().keyring.boundary = func(name string) {
		if name == os.Getenv("MODELCAIRN_ROTATION_POINT") {
			fmt.Println("boundary-reached")
			var signal [1]byte
			_, _ = os.Stdin.Read(signal[:])
			os.Exit(72)
		}
	}
	_, err = i.Secrets().RotateMasterKey(context.Background(), Actor{Type: "cli"})
	if err != nil {
		t.Fatal(err)
	}
	t.Fatal("requested boundary not reached")
}

func TestRotationSurvivesProcessKillAtEveryBoundary(t *testing.T) {
	points := []string{"temporary-created", "file-synced", "key-renamed", "directory-synced", "row-reencrypted", "before-commit", "database-committed", "rows-verified", "old-key-removed", "gc-directory-synced", "temporary-removed", "cleanup-synced"}
	for index, point := range points {
		t.Run(point, func(t *testing.T) {
			dir := t.TempDir()
			i, before := seedRotation(t, dir)
			if err := os.WriteFile(filepath.Join(dir, keyringName, ".new-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.tmp"), make([]byte, 32), 0600); err != nil {
				t.Fatal(err)
			}
			if err := i.Close(); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRotationProcessHelper$")
			cmd.Env = append(os.Environ(), "MODELCAIRN_ROTATION_HELPER=1", "MODELCAIRN_ROTATION_DIR="+dir, "MODELCAIRN_ROTATION_POINT="+point)
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			stdin, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			defer stdin.Close()
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			scanner := bufio.NewScanner(stdout)
			found := scanner.Scan() && scanner.Text() == "boundary-reached"
			if err := cmd.Process.Kill(); err != nil && found {
				t.Fatal(err)
			}
			_ = cmd.Wait()
			if !found {
				t.Fatal("child failed to reach requested boundary")
			}
			i, err = OpenInstallation(context.Background(), dir)
			if err != nil {
				t.Fatal(err)
			}
			defer i.Close()
			checkRotationValues(t, i, before, index >= 6)
			if err := i.Secrets().CollectUnusedKeys(context.Background()); err != nil {
				t.Fatal(err)
			}
			versions, err := i.Secrets().keyring.versions()
			if err != nil || len(versions) != 1 || versions[0] != i.Secrets().ActiveKeyVersion() {
				t.Fatal("collection removed a required key or retained an orphan")
			}
			entries, err := os.ReadDir(filepath.Join(dir, keyringName))
			if err != nil || len(entries) != 1 {
				t.Fatal("cleanup left an abandoned temporary")
			}
			var audits int
			if err := i.DB().QueryRow(`SELECT count(*) FROM audit_events WHERE action='secret.rotate' AND result='success'`).Scan(&audits); err != nil {
				t.Fatal(err)
			}
			want := 0
			if index >= 6 {
				want = 1
			}
			if audits != want {
				t.Fatal("audit differs from committed generation")
			}
		})
	}
}

func TestRotationEmptyInstallation(t *testing.T) {
	dir := t.TempDir()
	i, err := OpenInstallation(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := i.Secrets().RotateMasterKey(context.Background(), Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	if err := i.Close(); err != nil {
		t.Fatal(err)
	}
	i, err = OpenInstallation(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer i.Close()
	if i.Secrets().ActiveKeyVersion() != 2 {
		t.Fatal("empty active key not preserved")
	}
}

func TestRotationFailClosedAfterCommit(t *testing.T) {
	for _, mode := range []string{"missing-key", "wrong-key", "bad-row", "missing-check", "commit-error"} {
		t.Run(mode, func(t *testing.T) {
			i, _ := seedRotation(t, t.TempDir())
			defer i.Close()
			s := i.Secrets()
			s.keyring.boundary = func(point string) {
				if point != "database-committed" {
					return
				}
				var err error
				switch mode {
				case "missing-key":
					err = os.Remove(filepath.Join(s.keyring.dir, "v2.key"))
				case "wrong-key":
					err = os.WriteFile(filepath.Join(s.keyring.dir, "v2.key"), bytes.Repeat([]byte{9}, 32), 0600)
				case "bad-row":
					_, err = s.db.Exec(`UPDATE secrets SET ciphertext=x'00'`)
				case "missing-check":
					_, err = s.db.Exec(`UPDATE installation_state SET key_check=NULL`)
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			ctx := context.Background()
			if mode == "commit-error" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				defer cancel()
				s.keyring.boundary = func(point string) {
					if point == "before-commit" {
						cancel()
					}
				}
			}
			if _, err := s.RotateMasterKey(ctx, Actor{Type: "cli"}); err == nil {
				t.Fatal("fault accepted")
			}
			if _, err := s.Put(context.Background(), PutSecret{Name: "new", Value: []byte("another-canary")}, Actor{Type: "cli"}); err != errKeyMaterialUnavailable {
				t.Fatal("write did not fail closed")
			}
			if err := s.Use(context.Background(), "alpha", func([]byte) error { t.Fatal("callback ran in unavailable store"); return nil }); err != errKeyMaterialUnavailable {
				t.Fatal("use did not fail closed")
			}
			if err := s.Delete(context.Background(), "alpha", 1, Actor{Type: "cli"}); err != errKeyMaterialUnavailable {
				t.Fatal("delete did not fail closed")
			}
		})
	}
}

func TestSecretCallbackCanMutateStore(t *testing.T) {
	i, _ := seedRotation(t, t.TempDir())
	done := make(chan error, 1)
	go func() {
		done <- i.Secrets().Use(context.Background(), "alpha", func(plain []byte) error {
			if err := i.Secrets().Delete(context.Background(), "alpha", 1, Actor{Type: "cli"}); err != nil {
				return err
			}
			if i.Secrets().Redactor().String(string(plain)) != "[REDACTED]" {
				return fmt.Errorf("callback lost redaction")
			}
			return nil
		})
	}()
	select {
	case err := <-done:
		defer i.Close()
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("reentrant callback deadlocked")
	}
}

func TestMissingKeyCheckCannotBeRecreated(t *testing.T) {
	dir := t.TempDir()
	i, err := OpenInstallation(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := i.DB().Exec(`UPDATE installation_state SET key_check=NULL`); err != nil {
		t.Fatal(err)
	}
	if err := i.Close(); err != nil {
		t.Fatal(err)
	}
	if reopened, err := OpenInstallation(context.Background(), dir); err == nil {
		reopened.Close()
		t.Fatal("missing authentication evidence recreated")
	}
}

func TestCollectionRetainsNonactiveReferencedKey(t *testing.T) {
	i, before := seedRotation(t, t.TempDir())
	defer i.Close()
	s := i.Secrets()
	key, err := s.keyring.create(2)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(key)
	var id string
	if err := s.db.QueryRow(`SELECT id FROM secrets WHERE name='alpha'`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	plain := []byte("rotation-canary-alpha")
	nonce, ciphertext, err := sealSecret(key, plain, secretContext{s.installationID, id, 1, 2})
	if err != nil {
		t.Fatal(err)
	}
	fp, err := secretFingerprint(key, plain, s.installationID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE secrets SET key_version=2,nonce=?,ciphertext=?,fingerprint=? WHERE id=?`, nonce, ciphertext, fp, id); err != nil {
		t.Fatal(err)
	}
	if err := s.CollectUnusedKeys(context.Background()); err != nil {
		t.Fatal(err)
	}
	versions, err := s.keyring.versions()
	if err != nil || len(versions) != 2 {
		t.Fatal("GC removed referenced key")
	}
	if _, err := s.RotateMasterKey(context.Background(), Actor{Type: "cli"}); err != nil {
		t.Fatal(err)
	}
	checkRotationValues(t, i, before, true)
}
