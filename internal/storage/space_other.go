//go:build !linux && !windows

package storage

import "math"

func availableDiskBytes(string) (uint64, error) { return math.MaxUint64, nil }
