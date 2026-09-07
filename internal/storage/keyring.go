package storage

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	masterKeySize = 32
	keyringName   = "keys"
)

var errKeyMaterialUnavailable = errors.New("key_material_unavailable")

type keyring struct {
	dir string
	// boundary is an internal test seam, never configured by environment or CLI.
	boundary func(string)
}

func (k *keyring) checkpoint(name string) {
	if k.boundary != nil {
		k.boundary(name)
	}
}

func openKeyring(dataDir string) (*keyring, error) {
	dir := filepath.Join(dataDir, keyringName)
	if err := ensurePrivateDirectory(dir); err != nil {
		return nil, fmt.Errorf("prepare keyring: %w", err)
	}
	if err := syncDirectory(dataDir); err != nil {
		return nil, fmt.Errorf("sync keyring parent: %w", err)
	}
	return &keyring{dir: dir}, nil
}

func (k *keyring) create(version int64) ([]byte, error) {
	if version < 1 {
		return nil, errKeyMaterialUnavailable
	}
	key := make([]byte, masterKeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate master key: %w", err)
	}
	published := false
	defer func() {
		if !published {
			clear(key)
		}
	}()

	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return nil, fmt.Errorf("generate key filename: %w", err)
	}
	temporary := filepath.Join(k.dir, ".new-"+hex.EncodeToString(random)+".tmp")
	file, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create private key temporary: %w", err)
	}
	cleanup := true
	k.checkpoint("temporary-created")
	defer func() {
		if cleanup {
			_ = os.Remove(temporary)
		}
	}()
	err = file.Chmod(0o600)
	if err == nil {
		err = securePrivateFile(file, temporary)
	}
	if err == nil {
		_, err = file.Write(key)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, fmt.Errorf("write private key: %w", err)
	}
	k.checkpoint("file-synced")
	final := filepath.Join(k.dir, keyFilename(version))
	if err := renameNoReplace(temporary, final); err != nil {
		return nil, fmt.Errorf("publish master key version %d: %w", version, err)
	}
	cleanup = false
	k.checkpoint("key-renamed")
	if err := syncDirectory(k.dir); err != nil {
		return nil, fmt.Errorf("sync keyring: %w", err)
	}
	k.checkpoint("directory-synced")
	published = true
	return key, nil
}

func (k *keyring) load(version int64) ([]byte, error) {
	if version < 1 {
		return nil, errKeyMaterialUnavailable
	}
	file, err := openKeyFile(filepath.Join(k.dir, keyFilename(version)))
	if err != nil {
		return nil, fmt.Errorf("%w: master key version %d", errKeyMaterialUnavailable, version)
	}
	defer file.Close()
	if err := validatePrivateFile(file); err != nil {
		return nil, fmt.Errorf("%w: master key version %d", errKeyMaterialUnavailable, version)
	}
	key := make([]byte, masterKeySize+1)
	n, err := io.ReadFull(file, key)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		clear(key)
		return nil, fmt.Errorf("%w: master key version %d", errKeyMaterialUnavailable, version)
	}
	if n != masterKeySize {
		clear(key)
		return nil, fmt.Errorf("%w: master key version %d", errKeyMaterialUnavailable, version)
	}
	return key[:masterKeySize], nil
}

func (k *keyring) versions() ([]int64, error) {
	entries, err := os.ReadDir(k.dir)
	if err != nil {
		return nil, fmt.Errorf("read keyring: %w", err)
	}
	var versions []int64
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "v") || !strings.HasSuffix(name, ".key") {
			continue
		}
		version, err := strconv.ParseInt(strings.TrimSuffix(strings.TrimPrefix(name, "v"), ".key"), 10, 64)
		if err != nil || version < 1 || keyFilename(version) != name {
			return nil, fmt.Errorf("invalid keyring entry")
		}
		versions = append(versions, version)
	}
	sort.Slice(versions, func(a, b int) bool { return versions[a] < versions[b] })
	return versions, nil
}

func keyFilename(version int64) string { return fmt.Sprintf("v%d.key", version) }

func masterKeyCheck(key []byte, installationID string) ([]byte, error) {
	if len(key) != masterKeySize || installationID == "" {
		return nil, errKeyMaterialUnavailable
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("modelcairn/master-key-check/v1\x00"))
	_, _ = mac.Write([]byte(installationID))
	return mac.Sum(nil), nil
}
