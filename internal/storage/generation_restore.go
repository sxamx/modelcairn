package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"time"
)

// RestoredSecret carries the stable fields authenticated by MCB1. Ciphertext,
// fingerprints, installation identity, and key version are always regenerated.
type RestoredSecret struct {
	ID              string
	Name            string
	ResourceVersion int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// GenerationRestore owns the root lock until it is activated or aborted.
type GenerationRestore struct {
	root      string
	name      string
	dir       string
	lock      *Lock
	db        *sql.DB
	tx        *sql.Tx
	keys      *keyring
	key       []byte
	identity  string
	expected  int
	seen      map[string]struct{}
	sealed    bool
	activated bool
}

// BeginGenerationRestore creates an unreferenced private generation.
func BeginGenerationRestore(dataDir string) (*GenerationRestore, error) {
	lock, err := AcquireLock(dataDir)
	if err != nil {
		return nil, err
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		_ = lock.Close()
		return nil, fmt.Errorf("generate restore identity: %w", err)
	}
	name := hex.EncodeToString(random)
	generations := filepath.Join(dataDir, "generations")
	if err := ensurePrivateDirectory(generations); err != nil {
		_ = lock.Close()
		return nil, fmt.Errorf("prepare generations directory: %w", err)
	}
	dir := filepath.Join(generations, name)
	if err := os.Mkdir(dir, 0o700); err != nil {
		_ = lock.Close()
		return nil, fmt.Errorf("create restore generation: %w", err)
	}
	return &GenerationRestore{root: dataDir, name: name, dir: dir, lock: lock, seen: make(map[string]struct{})}, nil
}

// WriteDatabase installs the authenticated snapshot into the unreferenced
// generation and prepares a transaction for streamed secret re-encryption.
func (r *GenerationRestore) WriteDatabase(ctx context.Context, reader io.Reader, size int64) error {
	if r.db != nil || r.sealed || size < 1 {
		return fmt.Errorf("invalid restore database state")
	}
	required := uint64(size)
	if required > (math.MaxUint64-(16<<20))/2 {
		return fmt.Errorf("restored database size overflows disk budget")
	}
	if err := RequireFreeSpace(r.dir, required*2+(16<<20)); err != nil {
		return err
	}
	path := filepath.Join(r.dir, "modelcairn.db")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create restored database: %w", err)
	}
	written, copyErr := io.CopyN(file, reader, size)
	if copyErr == nil && written != size {
		copyErr = io.ErrUnexpectedEOF
	}
	if copyErr == nil {
		copyErr = file.Sync()
	}
	closeErr := file.Close()
	if copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		return fmt.Errorf("write restored database: %w", copyErr)
	}
	r.db, err = OpenSQLite(ctx, path)
	if err != nil {
		return err
	}
	if err := Migrate(ctx, r.db); err != nil {
		return fmt.Errorf("migrate restored database: %w", err)
	}
	if err := CheckIntegrity(ctx, r.db); err != nil {
		return err
	}
	if err := CheckSchemaCompatibility(ctx, r.db); err != nil {
		return err
	}
	if err := r.db.QueryRowContext(ctx, "SELECT count(*) FROM secrets").Scan(&r.expected); err != nil {
		return fmt.Errorf("count restored secrets: %w", err)
	}
	r.keys, err = openKeyring(r.dir)
	if err != nil {
		return err
	}
	r.key, err = r.keys.create(1)
	if err != nil {
		return err
	}
	r.identity, err = newUUID()
	if err != nil {
		return err
	}
	r.tx, err = r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin restored state transaction: %w", err)
	}
	return nil
}

// RestoreSecret replaces one old ciphertext with ciphertext bound to the new
// installation identity while retaining its database identifier and lifecycle.
func (r *GenerationRestore) RestoreSecret(ctx context.Context, item RestoredSecret, value []byte) error {
	if r.tx == nil || r.sealed || !secretNamePattern.MatchString(item.Name) || item.ID == "" || item.ResourceVersion < 1 || item.CreatedAt.IsZero() || item.UpdatedAt.Before(item.CreatedAt) || len(value) < 1 || len(value) > 16<<10 {
		return fmt.Errorf("invalid restored secret")
	}
	if _, exists := r.seen[item.ID]; exists {
		return fmt.Errorf("duplicate restored secret")
	}
	var name, created, updated string
	var version int64
	if err := r.tx.QueryRowContext(ctx, "SELECT name,resource_version,created_at,updated_at FROM secrets WHERE id=?", item.ID).Scan(&name, &version, &created, &updated); err != nil {
		return fmt.Errorf("match restored secret: %w", err)
	}
	if name != item.Name || version != item.ResourceVersion || created != item.CreatedAt.UTC().Format(time.RFC3339Nano) || updated != item.UpdatedAt.UTC().Format(time.RFC3339Nano) {
		return fmt.Errorf("restored secret metadata does not match database")
	}
	nonce, ciphertext, err := sealSecret(r.key, value, secretContext{r.identity, item.ID, item.ResourceVersion, 1})
	if err != nil {
		return err
	}
	fingerprint, err := secretFingerprint(r.key, value, r.identity)
	if err != nil {
		return err
	}
	result, err := r.tx.ExecContext(ctx, `UPDATE secrets SET key_version=1,algorithm='XCHACHA20-POLY1305',nonce=?,ciphertext=?,fingerprint=? WHERE id=?`, nonce, ciphertext, fingerprint, item.ID)
	if err != nil {
		return fmt.Errorf("re-encrypt restored secret: %w", err)
	}
	if changed, err := result.RowsAffected(); err != nil || changed != 1 {
		return fmt.Errorf("restored secret update was not singular")
	}
	r.seen[item.ID] = struct{}{}
	return nil
}

// Seal commits and validates the staged generation without making it active.
func (r *GenerationRestore) Seal(ctx context.Context) error {
	if r.tx == nil || r.sealed || len(r.seen) != r.expected {
		return fmt.Errorf("restored secret set is incomplete")
	}
	check, err := masterKeyCheck(r.key, r.identity)
	if err == nil {
		now := time.Now().UTC().Format(time.RFC3339Nano)
		_, err = r.tx.ExecContext(ctx, `UPDATE installation_state SET installation_id=?,active_key_version=1,key_check=?,updated_at=? WHERE singleton=1`, r.identity, check, now)
	}
	if err == nil {
		_, err = r.tx.ExecContext(ctx, "DELETE FROM admin_sessions")
	}
	if err == nil {
		_, err = r.tx.ExecContext(ctx, "DELETE FROM consumed_plan_tokens")
	}
	if err != nil {
		_ = r.tx.Rollback()
		r.tx = nil
		return fmt.Errorf("finalize restored state: %w", err)
	}
	if err := r.tx.Commit(); err != nil {
		r.tx = nil
		return fmt.Errorf("commit restored state: %w", err)
	}
	r.tx = nil
	if err := CheckIntegrity(ctx, r.db); err != nil {
		return err
	}
	if err := CheckSchemaCompatibility(ctx, r.db); err != nil {
		return err
	}
	store, err := openSecretStore(ctx, r.db, r.keys)
	if err != nil {
		return fmt.Errorf("verify restored secret store: %w", err)
	}
	store.close()
	if err := r.db.Close(); err != nil {
		return fmt.Errorf("close restored database: %w", err)
	}
	r.db = nil
	clear(r.key)
	r.key = nil
	r.sealed = true
	return nil
}

// Activate atomically selects a fully sealed generation.
func (r *GenerationRestore) Activate() error {
	if !r.sealed || r.activated {
		return fmt.Errorf("restore generation is not ready for activation")
	}
	previous, err := activeGenerationName(r.root)
	if err != nil {
		return err
	}
	if err := writePreviousGeneration(r.dir, previous); err != nil {
		return err
	}
	if err := activateGeneration(r.root, r.name); err != nil {
		return err
	}
	r.activated = true
	return r.releaseLock()
}

func (r *GenerationRestore) releaseLock() error {
	if r.lock == nil {
		return nil
	}
	err := r.lock.Close()
	r.lock = nil
	return err
}

// Abort removes only an unreferenced generation and never changes current.
func (r *GenerationRestore) Abort() error {
	if r.activated {
		return nil
	}
	if r.tx != nil {
		_ = r.tx.Rollback()
		r.tx = nil
	}
	if r.db != nil {
		_ = r.db.Close()
		r.db = nil
	}
	clear(r.key)
	r.key = nil
	removeErr := os.RemoveAll(r.dir)
	lockErr := r.releaseLock()
	if removeErr != nil {
		return removeErr
	}
	return lockErr
}

func (r *GenerationRestore) Generation() string { return r.name }

func activeGenerationName(root string) (string, error) {
	state, err := resolveStateDirectory(root)
	if err != nil {
		return "", err
	}
	if filepath.Clean(state) == filepath.Clean(root) {
		return "legacy", nil
	}
	name := filepath.Base(state)
	if !generationNamePattern.MatchString(name) {
		return "", fmt.Errorf("active generation name is invalid")
	}
	return name, nil
}

func writePreviousGeneration(generationDir, previous string) error {
	if previous != "legacy" && !generationNamePattern.MatchString(previous) {
		return fmt.Errorf("previous generation name is invalid")
	}
	path := filepath.Join(generationDir, ".previous")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create rollback marker: %w", err)
	}
	_, writeErr := io.WriteString(file, previous+"\n")
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		_ = os.Remove(path)
		return fmt.Errorf("write rollback marker: %w", writeErr)
	}
	if err := syncDirectory(generationDir); err != nil {
		return fmt.Errorf("sync rollback marker: %w", err)
	}
	return nil
}

// RollbackGeneration atomically selects the predecessor recorded by the active
// generation. Neither the active nor predecessor data is deleted.
func RollbackGeneration(dataDir string) (string, string, error) {
	lock, err := AcquireLock(dataDir)
	if err != nil {
		return "", "", err
	}
	defer lock.Close()
	state, err := resolveStateDirectory(dataDir)
	if err != nil {
		return "", "", err
	}
	if filepath.Clean(state) == filepath.Clean(dataDir) {
		return "", "", fmt.Errorf("legacy layout has no recorded predecessor")
	}
	active := filepath.Base(state)
	file, err := os.Open(filepath.Join(state, ".previous"))
	if err != nil {
		return "", "", fmt.Errorf("open rollback marker: %w", err)
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, 65))
	closeErr := file.Close()
	if readErr == nil {
		readErr = closeErr
	}
	if readErr != nil || len(raw) > 64 {
		return "", "", fmt.Errorf("read rollback marker")
	}
	previous := string(raw)
	if len(previous) == 0 || previous[len(previous)-1] != '\n' {
		return "", "", fmt.Errorf("invalid rollback marker")
	}
	previous = previous[:len(previous)-1]
	if previous == "legacy" {
		if err := deactivateGeneration(dataDir); err != nil {
			return "", "", err
		}
	} else {
		if !generationNamePattern.MatchString(previous) {
			return "", "", fmt.Errorf("invalid rollback target")
		}
		info, err := os.Lstat(filepath.Join(dataDir, "generations", previous))
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "", "", fmt.Errorf("rollback target is unavailable")
		}
		if err := activateGeneration(dataDir, previous); err != nil {
			return "", "", err
		}
	}
	return active, previous, nil
}
