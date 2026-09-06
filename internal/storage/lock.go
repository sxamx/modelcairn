package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrInstallationInUse is returned without opening SQLite or key material.
var ErrInstallationInUse = errors.New("installation_in_use")

// Lock holds the kernel lock and its descriptor for an entire stateful operation.
type Lock struct{ file *os.File }

// AcquireLock creates a private data directory and takes its shared process lock.
func AcquireLock(dataDir string) (*Lock, error) {
	if err := ensurePrivateDirectory(dataDir); err != nil {
		return nil, err
	}
	file, err := openLockFile(filepath.Join(dataDir, "modelcairn.lock"))
	if err != nil {
		return nil, fmt.Errorf("open installation lock: %w", err)
	}
	if err := tryLock(file); err != nil {
		_ = file.Close()
		if isLockContended(err) {
			return nil, ErrInstallationInUse
		}
		return nil, fmt.Errorf("lock installation: %w", err)
	}
	return &Lock{file: file}, nil
}

func ensurePrivateDirectory(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return fmt.Errorf("create data directory: %w", err)
		}
		info, err = os.Lstat(path)
	}
	if err != nil {
		return fmt.Errorf("inspect data directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("data directory must be a real directory")
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return fmt.Errorf("secure data directory: %w", err)
	}
	return nil
}

func (l *Lock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	unlockErr := unlock(l.file)
	closeErr := l.file.Close()
	l.file = nil
	if unlockErr != nil {
		return fmt.Errorf("unlock installation: %w", unlockErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close installation lock: %w", closeErr)
	}
	return nil
}
