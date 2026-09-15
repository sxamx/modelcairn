//go:build !windows

package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestActivateGenerationReportsLinkedWhenDirectorySyncFails(t *testing.T) {
	root := t.TempDir()
	name := "0123456789abcdef0123456789abcdef"
	target := filepath.Join(root, "generations", name)
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	syncFailure := errors.New("injected directory sync failure")
	linked, err := activateGenerationWithSync(root, name, func(string) error { return syncFailure })
	if !linked || !errors.Is(err, syncFailure) {
		t.Fatalf("expected linked pointer and sync failure, got linked=%v err=%v", linked, err)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(root, "current"))
	if err != nil {
		t.Fatal(err)
	}
	if resolved != target {
		t.Fatalf("current resolved to %q, want %q", resolved, target)
	}
}
