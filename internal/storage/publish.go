package storage

import "fmt"

// PublishNoReplace atomically moves a prepared file into place and fails if the
// destination appeared concurrently.
func PublishNoReplace(temporary, destination string) error {
	if temporary == "" || destination == "" {
		return fmt.Errorf("publish paths are required")
	}
	return renameNoReplace(temporary, destination)
}
