package storage

import (
	"math"
	"testing"
)

func TestRequireFreeSpaceAcceptsSmallWriteAndRejectsImpossibleWrite(t *testing.T) {
	directory := t.TempDir()
	if err := RequireFreeSpace(directory, 1); err != nil {
		t.Fatalf("small write rejected: %v", err)
	}
	if err := RequireFreeSpace(directory, math.MaxUint64); err == nil {
		t.Fatal("impossible disk budget was accepted")
	}
}
