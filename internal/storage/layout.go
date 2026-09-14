package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

var generationNamePattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

func resolveStateDirectory(root string) (string, error) {
	current := filepath.Join(root, "current")
	info, err := os.Lstat(current)
	if os.IsNotExist(err) {
		return root, nil
	}
	if err != nil {
		return "", fmt.Errorf("inspect current generation: %w", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return "", fmt.Errorf("current generation pointer must be a symbolic link")
	}
	target, err := os.Readlink(current)
	if err != nil {
		return "", fmt.Errorf("read current generation pointer: %w", err)
	}
	clean := filepath.Clean(target)
	if filepath.IsAbs(clean) || filepath.Dir(clean) != "generations" || !generationNamePattern.MatchString(filepath.Base(clean)) {
		return "", fmt.Errorf("current generation pointer is outside the managed layout")
	}
	resolved := filepath.Join(root, clean)
	stateInfo, err := os.Lstat(resolved)
	if err != nil {
		return "", fmt.Errorf("inspect active generation: %w", err)
	}
	if stateInfo.Mode()&os.ModeSymlink != 0 || !stateInfo.IsDir() {
		return "", fmt.Errorf("active generation must be a real directory")
	}
	if err := ensurePrivateDirectory(resolved); err != nil {
		return "", fmt.Errorf("validate active generation: %w", err)
	}
	return resolved, nil
}
