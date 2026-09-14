//go:build linux

package storage

import (
	"fmt"
	"math"
	"syscall"
)

func availableDiskBytes(path string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	blocks, size := uint64(stat.Bavail), uint64(stat.Bsize)
	if size != 0 && blocks > math.MaxUint64/size {
		return 0, fmt.Errorf("available disk size overflow")
	}
	return blocks * size, nil
}
