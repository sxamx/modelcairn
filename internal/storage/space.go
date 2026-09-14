package storage

import "fmt"

// RequireFreeSpace fails before a large write when the filesystem cannot
// provide the requested bytes to the current service identity.
func RequireFreeSpace(path string, required uint64) error {
	available, err := availableDiskBytes(path)
	if err != nil {
		return fmt.Errorf("inspect available disk space: %w", err)
	}
	if available < required {
		return fmt.Errorf("insufficient disk space: need %d bytes, have %d", required, available)
	}
	return nil
}
