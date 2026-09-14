//go:build windows

package storage

import "fmt"

func activateGeneration(_, _ string) error {
	return fmt.Errorf("generational activation is supported on Linux installations")
}
