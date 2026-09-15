//go:build !windows

package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

func activateGeneration(root, name string) (bool, error) {
	return activateGenerationWithSync(root, name, syncDirectory)
}

func activateGenerationWithSync(root, name string, syncRoot func(string) error) (bool, error) {
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		return false, err
	}
	temporary := filepath.Join(root, ".current-"+hex.EncodeToString(random)+".tmp")
	if err := os.Symlink(filepath.Join("generations", name), temporary); err != nil {
		return false, fmt.Errorf("create generation pointer: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(temporary)
		}
	}()
	if err := os.Rename(temporary, filepath.Join(root, "current")); err != nil {
		return false, fmt.Errorf("activate generation pointer: %w", err)
	}
	cleanup = false
	if err := syncRoot(root); err != nil {
		return true, fmt.Errorf("sync generation pointer: %w", err)
	}
	return true, nil
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
