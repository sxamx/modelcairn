//go:build !windows

package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

func activateGeneration(root, name string) error {
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		return err
	}
	temporary := filepath.Join(root, ".current-"+hex.EncodeToString(random)+".tmp")
	if err := os.Symlink(filepath.Join("generations", name), temporary); err != nil {
		return fmt.Errorf("create generation pointer: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(temporary)
		}
	}()
	if err := os.Rename(temporary, filepath.Join(root, "current")); err != nil {
		return fmt.Errorf("activate generation pointer: %w", err)
	}
	cleanup = false
	if err := syncDirectory(root); err != nil {
		return fmt.Errorf("sync generation pointer: %w", err)
	}
	return nil
}

func deactivateGeneration(root string) error {
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		return err
	}
	retired := filepath.Join(root, ".current-retired-"+hex.EncodeToString(random)+".tmp")
	if err := os.Rename(filepath.Join(root, "current"), retired); err != nil {
		return fmt.Errorf("deactivate generation pointer: %w", err)
	}
	if err := syncDirectory(root); err != nil {
		return fmt.Errorf("sync deactivated generation pointer: %w", err)
	}
	if err := os.Remove(retired); err != nil {
		return fmt.Errorf("remove retired generation pointer: %w", err)
	}
	return syncDirectory(root)
}
