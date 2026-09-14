// Package backupmcb1 implements ModelCairn Backup profile version 1.
package backupmcb1

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"filippo.io/age"
	_ "modernc.org/sqlite"

	"github.com/sxamx/modelcairn/internal/storage"
)

const (
	Profile             = "modelcairn-backup"
	ProfileVersion      = 1
	maxManifestBytes    = 1 << 20
	maxDatabaseBytes    = 64 << 30
	maxSecretsBytes     = 64 << 20
	maxChecksumsBytes   = 1 << 20
	maxPlaintextBytes   = 65 << 30
	maxEncryptedBytes   = 66 << 30
	maxAgeHeaderBytes   = 64 << 10
	maxSecretValueBytes = 16 << 10
	maxPassphraseBytes  = 1024
	minPassphraseBytes  = 8
	defaultScryptLogN   = 16
	minimumScryptLogN   = 15
	maximumScryptLogN   = 18
)

var entryOrder = []string{"manifest.json", "database.sqlite", "secrets.jsonl", "checksums.json"}
var secretNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

type Entry struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type Manifest struct {
	Profile            string    `json:"profile"`
	ProfileVersion     int       `json:"profileVersion"`
	ApplicationVersion string    `json:"applicationVersion"`
	SchemaVersion      int       `json:"schemaVersion"`
	CreatedAt          time.Time `json:"createdAt"`
	Entries            []Entry   `json:"entries"`
}

type secretRecord struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	ResourceVersion int64     `json:"resourceVersion"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
	Value           []byte    `json:"value"`
}

type Checksums struct {
	Algorithm string            `json:"algorithm"`
	Entries   map[string]string `json:"entries"`
}

type CreateOptions struct {
	Installation       *storage.Installation
	Destination        string
	Passphrase         []byte
	ApplicationVersion string
	Now                time.Time
}

type Verification struct {
	Manifest Manifest
	Secrets  int
}

func validatePassphrase(passphrase []byte) error {
	if len(passphrase) < minPassphraseBytes || len(passphrase) > maxPassphraseBytes {
		return fmt.Errorf("backup passphrase must contain between %d and %d bytes", minPassphraseBytes, maxPassphraseBytes)
	}
	return nil
}

// Create writes and verifies a private file before atomically publishing it.
func Create(ctx context.Context, options CreateOptions) (Verification, error) {
	if options.Installation == nil || options.Destination == "" {
		return Verification{}, fmt.Errorf("installation and destination are required")
	}
	if err := validatePassphrase(options.Passphrase); err != nil {
		return Verification{}, err
	}
	if options.Now.IsZero() {
		options.Now = time.Now().UTC()
	} else {
		options.Now = options.Now.UTC()
	}
	if options.ApplicationVersion == "" {
		options.ApplicationVersion = "dev"
	}
	destination, err := filepath.Abs(options.Destination)
	if err != nil {
		return Verification{}, fmt.Errorf("resolve backup destination: %w", err)
	}
	if _, err := os.Lstat(destination); err == nil {
		return Verification{}, fmt.Errorf("backup destination already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return Verification{}, fmt.Errorf("inspect backup destination: %w", err)
	}
	parent := filepath.Dir(destination)
	work, err := os.MkdirTemp(parent, ".modelcairn-backup-work-")
	if err != nil {
		return Verification{}, fmt.Errorf("create private backup workspace: %w", err)
	}
	if err := os.Chmod(work, 0o700); err != nil {
		_ = os.RemoveAll(work)
		return Verification{}, fmt.Errorf("protect backup workspace: %w", err)
	}
	defer os.RemoveAll(work)
	snapshot := filepath.Join(work, "database.sqlite")
	if err := options.Installation.SnapshotSQLite(ctx, snapshot); err != nil {
		return Verification{}, err
	}
	databaseInfo, err := os.Stat(snapshot)
	if err != nil || databaseInfo.Size() > maxDatabaseBytes {
		return Verification{}, fmt.Errorf("database snapshot exceeds MCB1 limit")
	}
	schemaVersion, err := readSchemaVersion(ctx, options.Installation.DB())
	if err != nil {
		return Verification{}, err
	}
	secretSize, secretHash, secretCount, err := measureSecrets(ctx, options.Installation.Secrets())
	if err != nil {
		return Verification{}, err
	}
	databaseHash, err := hashFile(snapshot, maxDatabaseBytes)
	if err != nil {
		return Verification{}, err
	}
	manifest, manifestBytes, checksumBytes, err := buildMetadata(options, schemaVersion, databaseInfo.Size(), secretSize, databaseHash, secretHash)
	if err != nil {
		return Verification{}, err
	}
	plaintextSize := int64(len(manifestBytes)) + databaseInfo.Size() + secretSize + int64(len(checksumBytes))
	if plaintextSize > maxPlaintextBytes {
		return Verification{}, fmt.Errorf("backup payload exceeds MCB1 limit")
	}
	temporary, err := os.CreateTemp(parent, "."+filepath.Base(destination)+".*.tmp")
	if err != nil {
		return Verification{}, fmt.Errorf("create private backup file: %w", err)
	}
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		_ = os.Remove(temporary.Name())
		return Verification{}, fmt.Errorf("protect private backup file: %w", err)
	}
	temporaryPath := temporary.Name()
	keepTemporary := false
	defer func() {
		_ = temporary.Close()
		if !keepTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	recipient, err := age.NewScryptRecipient(string(options.Passphrase))
	if err != nil {
		return Verification{}, fmt.Errorf("create backup recipient: %w", err)
	}
	recipient.SetWorkFactor(defaultScryptLogN)
	encrypted, err := age.Encrypt(temporary, recipient)
	if err != nil {
		return Verification{}, fmt.Errorf("initialize backup encryption: %w", err)
	}
	tarWriter := tar.NewWriter(encrypted)
	writeErr := writeEntry(tarWriter, entryOrder[0], manifestBytes, options.Now)
	if writeErr == nil {
		writeErr = writeFileEntry(tarWriter, entryOrder[1], snapshot, databaseInfo.Size(), options.Now)
	}
	if writeErr == nil {
		writeErr = writeSecretsEntry(ctx, tarWriter, options.Installation.Secrets(), secretSize, options.Now)
	}
	if writeErr == nil {
		writeErr = writeEntry(tarWriter, entryOrder[3], checksumBytes, options.Now)
	}
	if closeErr := tarWriter.Close(); writeErr == nil {
		writeErr = closeErr
	}
	if closeErr := encrypted.Close(); writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		return Verification{}, fmt.Errorf("write encrypted backup: %w", writeErr)
	}
	if err := temporary.Sync(); err != nil {
		return Verification{}, fmt.Errorf("sync encrypted backup: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return Verification{}, fmt.Errorf("close encrypted backup: %w", err)
	}
	verification, err := Verify(ctx, temporaryPath, options.Passphrase)
	if err != nil {
		return Verification{}, fmt.Errorf("verify newly created backup: %w", err)
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return Verification{}, fmt.Errorf("publish backup atomically: %w", err)
	}
	keepTemporary = true
	if err := syncDirectory(parent); err != nil {
		return Verification{}, err
	}
	verification.Secrets = secretCount
	verification.Manifest = manifest
	return verification, nil
}

func buildMetadata(options CreateOptions, schemaVersion int, databaseSize, secretsSize int64, databaseHash, secretsHash string) (Manifest, []byte, []byte, error) {
	manifest := Manifest{Profile: Profile, ProfileVersion: ProfileVersion, ApplicationVersion: options.ApplicationVersion,
		SchemaVersion: schemaVersion, CreatedAt: options.Now, Entries: []Entry{{Name: entryOrder[0]}, {Name: entryOrder[1], Size: databaseSize}, {Name: entryOrder[2], Size: secretsSize}, {Name: entryOrder[3]}}}
	var manifestBytes, checksumBytes []byte
	for attempts := 0; attempts < 8; attempts++ {
		var err error
		manifestBytes, err = json.Marshal(manifest)
		if err != nil {
			return Manifest{}, nil, nil, err
		}
		manifestHash := sha256.Sum256(manifestBytes)
		checksums := Checksums{Algorithm: "SHA-256", Entries: map[string]string{
			entryOrder[0]: hex.EncodeToString(manifestHash[:]), entryOrder[1]: databaseHash, entryOrder[2]: secretsHash,
		}}
		checksumBytes, err = json.Marshal(checksums)
		if err != nil {
			return Manifest{}, nil, nil, err
		}
		previousManifest, previousChecksums := manifest.Entries[0].Size, manifest.Entries[3].Size
		manifest.Entries[0].Size = int64(len(manifestBytes))
		manifest.Entries[3].Size = int64(len(checksumBytes))
		if previousManifest == manifest.Entries[0].Size && previousChecksums == manifest.Entries[3].Size {
			return manifest, manifestBytes, checksumBytes, nil
		}
	}
	return Manifest{}, nil, nil, fmt.Errorf("MCB1 metadata sizes did not stabilize")
}

func readSchemaVersion(ctx context.Context, db *sql.DB) (int, error) {
	var version int
	if err := db.QueryRowContext(ctx, "SELECT coalesce(max(version),0) FROM schema_migrations").Scan(&version); err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	if version < 1 {
		return 0, fmt.Errorf("invalid schema version")
	}
	return version, nil
}

func encodeSecret(item storage.ExportedSecret, value []byte) ([]byte, error) {
	if len(value) < 1 || len(value) > maxSecretValueBytes {
		return nil, fmt.Errorf("secret %q exceeds MCB1 value limit", item.Name)
	}
	line, err := json.Marshal(secretRecord{ID: item.ID, Name: item.Name, ResourceVersion: item.ResourceVersion,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt, Value: value})
	if err != nil {
		return nil, err
	}
	line = append(line, '\n')
	return line, nil
}

func measureSecrets(ctx context.Context, store *storage.SecretStore) (int64, string, int, error) {
	hash := sha256.New()
	var size int64
	count := 0
	err := store.StreamSecrets(ctx, func(item storage.ExportedSecret, value []byte) error {
		line, err := encodeSecret(item, value)
		if err != nil {
			return err
		}
		defer clear(line)
		size += int64(len(line))
		if size > maxSecretsBytes {
			return fmt.Errorf("secrets payload exceeds MCB1 limit")
		}
		_, _ = hash.Write(line)
		count++
		return nil
	})
	return size, hex.EncodeToString(hash.Sum(nil)), count, err
}

func hashFile(path string, limit int64) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	written, err := io.Copy(hash, io.LimitReader(file, limit+1))
	if err != nil || written > limit {
		return "", fmt.Errorf("hash bounded file: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func writeHeader(writer *tar.Writer, name string, size int64, now time.Time) error {
	return writer.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: size, ModTime: now, Typeflag: tar.TypeReg, Format: tar.FormatPAX})
}

func writeEntry(writer *tar.Writer, name string, data []byte, now time.Time) error {
	if err := writeHeader(writer, name, int64(len(data)), now); err != nil {
		return err
	}
	_, err := writer.Write(data)
	return err
}

func writeFileEntry(writer *tar.Writer, name, path string, size int64, now time.Time) error {
	if err := writeHeader(writer, name, size, now); err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	written, err := io.CopyN(writer, file, size)
	if err != nil || written != size {
		return fmt.Errorf("copy database snapshot: %w", err)
	}
	return nil
}

func writeSecretsEntry(ctx context.Context, writer *tar.Writer, store *storage.SecretStore, size int64, now time.Time) error {
	if err := writeHeader(writer, entryOrder[2], size, now); err != nil {
		return err
	}
	var written int64
	err := store.StreamSecrets(ctx, func(item storage.ExportedSecret, value []byte) error {
		line, err := encodeSecret(item, value)
		if err != nil {
			return err
		}
		defer clear(line)
		n, err := writer.Write(line)
		written += int64(n)
		return err
	})
	if err != nil {
		return err
	}
	if written != size {
		return fmt.Errorf("secrets changed while creating backup")
	}
	return nil
}

// Verify authenticates the whole age stream, validates structure and checksums,
// and runs SQLite integrity/schema checks in a private temporary directory.
func Verify(ctx context.Context, path string, passphrase []byte) (Verification, error) {
	work, err := os.MkdirTemp("", "modelcairn-verify-")
	if err != nil {
		return Verification{}, err
	}
	defer os.RemoveAll(work)
	if err := os.Chmod(work, 0o700); err != nil {
		return Verification{}, err
	}
	databasePath := filepath.Join(work, "database.sqlite")
	verification, err := readArchive(path, passphrase, archiveSink{
		database: func(reader io.Reader, size int64) error {
			file, err := os.OpenFile(databasePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
			if err != nil {
				return err
			}
			written, copyErr := io.CopyN(file, reader, size)
			if copyErr == nil && written != size {
				copyErr = io.ErrUnexpectedEOF
			}
			closeErr := file.Close()
			if copyErr == nil {
				copyErr = closeErr
			}
			return copyErr
		},
		secret: func(secretRecord) error { return nil },
	})
	if err != nil {
		return Verification{}, err
	}
	if err := verifySQLite(ctx, databasePath); err != nil {
		return Verification{}, err
	}
	return verification, nil
}

type archiveSink struct {
	database func(io.Reader, int64) error
	secret   func(secretRecord) error
}

func readArchive(path string, passphrase []byte, sink archiveSink) (Verification, error) {
	if err := validatePassphrase(passphrase); err != nil {
		return Verification{}, err
	}
	if sink.database == nil || sink.secret == nil {
		return Verification{}, fmt.Errorf("MCB1 archive sink is incomplete")
	}
	info, err := os.Stat(path)
	if err != nil {
		return Verification{}, fmt.Errorf("inspect backup: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maxEncryptedBytes {
		return Verification{}, fmt.Errorf("backup physical size is outside MCB1 policy")
	}
	if err := validateAgeHeader(path); err != nil {
		return Verification{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return Verification{}, err
	}
	defer file.Close()
	identity, err := age.NewScryptIdentity(string(passphrase))
	if err != nil {
		return Verification{}, err
	}
	identity.SetMaxWorkFactor(maximumScryptLogN)
	decrypted, err := age.Decrypt(file, identity)
	if err != nil {
		return Verification{}, fmt.Errorf("decrypt MCB1 backup: %w", err)
	}
	plain := &io.LimitedReader{R: decrypted, N: maxPlaintextBytes + 1}
	tarReader := tar.NewReader(plain)
	var manifest Manifest
	actualHashes := make(map[string]string)
	actualSizes := make(map[string]int64)
	secretCount := 0
	var expected Checksums
	for index, expectedName := range entryOrder {
		header, err := tarReader.Next()
		if err != nil {
			return Verification{}, fmt.Errorf("read MCB1 entry %d: %w", index+1, err)
		}
		if header.Name != expectedName || (header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA) || header.Linkname != "" || header.Size < 0 {
			return Verification{}, fmt.Errorf("invalid MCB1 entry %d", index+1)
		}
		limit := entryLimit(expectedName)
		if header.Size > limit {
			return Verification{}, fmt.Errorf("MCB1 entry %q exceeds limit", expectedName)
		}
		hash := sha256.New()
		bounded := &countingReader{reader: io.LimitReader(tarReader, header.Size)}
		var buffer bytes.Buffer
		if expectedName == entryOrder[0] || expectedName == entryOrder[3] {
			_, err = io.Copy(io.MultiWriter(hash, &buffer), bounded)
		} else if expectedName == entryOrder[1] {
			err = sink.database(io.TeeReader(bounded, hash), header.Size)
		} else {
			secretCount, err = scanSecretRecords(io.TeeReader(bounded, hash), sink.secret)
		}
		if err != nil || bounded.count != header.Size {
			return Verification{}, fmt.Errorf("read MCB1 entry %q: %w", expectedName, err)
		}
		actualSizes[expectedName] = bounded.count
		actualHashes[expectedName] = hex.EncodeToString(hash.Sum(nil))
		if expectedName == entryOrder[0] {
			if err := decodeExactJSON(buffer.Bytes(), &manifest); err != nil {
				return Verification{}, fmt.Errorf("invalid MCB1 manifest: %w", err)
			}
			if err := validateManifest(manifest); err != nil {
				return Verification{}, err
			}
		} else if expectedName == entryOrder[3] {
			if err := decodeExactJSON(buffer.Bytes(), &expected); err != nil {
				return Verification{}, fmt.Errorf("invalid MCB1 checksums: %w", err)
			}
		}
	}
	if header, err := tarReader.Next(); err != io.EOF || header != nil {
		return Verification{}, fmt.Errorf("MCB1 contains undeclared tar entries")
	}
	trailing, err := io.Copy(io.Discard, plain)
	if err != nil {
		return Verification{}, fmt.Errorf("authenticate MCB1 payload: %w", err)
	}
	if trailing != 0 {
		return Verification{}, fmt.Errorf("MCB1 contains trailing plaintext")
	}
	if err := validateSizes(manifest, actualSizes); err != nil {
		return Verification{}, err
	}
	if expected.Algorithm != "SHA-256" || len(expected.Entries) != 3 {
		return Verification{}, fmt.Errorf("invalid MCB1 checksum policy")
	}
	for _, name := range entryOrder[:3] {
		if expected.Entries[name] == "" || expected.Entries[name] != actualHashes[name] {
			return Verification{}, fmt.Errorf("MCB1 checksum mismatch for %q", name)
		}
	}
	return Verification{Manifest: manifest, Secrets: secretCount}, nil
}

type countingReader struct {
	reader io.Reader
	count  int64
}

func (r *countingReader) Read(data []byte) (int, error) {
	n, err := r.reader.Read(data)
	r.count += int64(n)
	return n, err
}

func validateAgeHeader(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	header, err := age.ExtractHeader(io.LimitReader(file, maxAgeHeaderBytes+1))
	if err != nil || len(header) > maxAgeHeaderBytes {
		return fmt.Errorf("invalid or oversized age header")
	}
	lines := strings.Split(string(header), "\n")
	stanzas := 0
	for _, line := range lines {
		if !strings.HasPrefix(line, "-> ") {
			continue
		}
		stanzas++
		fields := strings.Fields(line)
		if len(fields) != 4 || fields[1] != "scrypt" {
			return fmt.Errorf("MCB1 requires exactly one scrypt recipient")
		}
		logN, err := strconv.Atoi(fields[3])
		if err != nil || logN < minimumScryptLogN || logN > maximumScryptLogN {
			return fmt.Errorf("MCB1 scrypt work factor is outside policy")
		}
	}
	if stanzas != 1 {
		return fmt.Errorf("MCB1 requires exactly one scrypt recipient")
	}
	return nil
}

func entryLimit(name string) int64 {
	switch name {
	case entryOrder[0]:
		return maxManifestBytes
	case entryOrder[1]:
		return maxDatabaseBytes
	case entryOrder[2]:
		return maxSecretsBytes
	case entryOrder[3]:
		return maxChecksumsBytes
	default:
		return 0
	}
}

func decodeExactJSON(data []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return fmt.Errorf("additional JSON value")
	}
	return nil
}

func validateManifest(manifest Manifest) error {
	if manifest.Profile != Profile || manifest.ProfileVersion != ProfileVersion || manifest.ApplicationVersion == "" || manifest.SchemaVersion < 1 || manifest.CreatedAt.IsZero() || len(manifest.Entries) != len(entryOrder) {
		return fmt.Errorf("unsupported or incomplete MCB1 manifest")
	}
	for index, entry := range manifest.Entries {
		if entry.Name != entryOrder[index] || entry.Size < 0 || entry.Size > entryLimit(entry.Name) {
			return fmt.Errorf("invalid MCB1 manifest entry")
		}
	}
	return nil
}

func validateSizes(manifest Manifest, actual map[string]int64) error {
	for _, entry := range manifest.Entries {
		if actual[entry.Name] != entry.Size {
			return fmt.Errorf("MCB1 size mismatch for %q", entry.Name)
		}
	}
	return nil
}

func verifySQLite(ctx context.Context, path string) error {
	dsn := "file:" + filepath.ToSlash(path) + "?mode=ro&immutable=1"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open backed-up sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	if err := storage.CheckIntegrity(ctx, db); err != nil {
		return fmt.Errorf("backed-up sqlite integrity: %w", err)
	}
	if err := storage.CheckSchemaCompatibility(ctx, db); err != nil {
		return fmt.Errorf("backed-up sqlite schema: %w", err)
	}
	return nil
}

func syncDirectory(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open backup directory for sync: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync backup directory: %w", err)
	}
	return nil
}

// Reserved for restore, where JSONL is decoded incrementally from a bounded
// reader rather than retained in memory.
func scanSecretRecords(reader io.Reader, callback func(secretRecord) error) (int, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 32<<10), 32<<10)
	count := 0
	identifiers := make(map[string]struct{})
	names := make(map[string]struct{})
	for scanner.Scan() {
		var record secretRecord
		if err := decodeExactJSON(scanner.Bytes(), &record); err != nil {
			return count, err
		}
		if record.ID == "" || !secretNamePattern.MatchString(record.Name) || record.ResourceVersion < 1 || record.CreatedAt.IsZero() || record.UpdatedAt.Before(record.CreatedAt) || len(record.Value) < 1 || len(record.Value) > maxSecretValueBytes {
			clear(record.Value)
			return count, fmt.Errorf("invalid secret record")
		}
		if _, exists := identifiers[record.ID]; exists {
			clear(record.Value)
			return count, fmt.Errorf("duplicate secret identifier")
		}
		if _, exists := names[record.Name]; exists {
			clear(record.Value)
			return count, fmt.Errorf("duplicate secret name")
		}
		identifiers[record.ID] = struct{}{}
		names[record.Name] = struct{}{}
		if err := callback(record); err != nil {
			clear(record.Value)
			return count, err
		}
		clear(record.Value)
		count++
	}
	return count, scanner.Err()
}
