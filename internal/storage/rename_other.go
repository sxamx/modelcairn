//go:build !linux && !windows

package storage

import "os"

func renameNoReplace(oldPath, newPath string) error {
	if err := os.Link(oldPath, newPath); err != nil {
		return err
	}
	return os.Remove(oldPath)
}
