package backupmcb1

import (
	"context"
	"fmt"
	"io"

	"github.com/sxamx/modelcairn/internal/storage"
)

type RestoreResult struct {
	Verification
	Generation string
}

// Restore verifies the complete archive before creating any generation, then
// validates it again while streaming plaintext secrets into new ciphertext.
func Restore(ctx context.Context, path string, passphrase []byte, dataDir string) (RestoreResult, error) {
	if dataDir == "" {
		return RestoreResult{}, fmt.Errorf("restore data directory is required")
	}
	if _, err := Verify(ctx, path, passphrase); err != nil {
		return RestoreResult{}, fmt.Errorf("restore preflight: %w", err)
	}
	builder, err := storage.BeginGenerationRestore(dataDir)
	if err != nil {
		return RestoreResult{}, err
	}
	defer builder.Abort()
	verification, err := readArchive(path, passphrase, archiveSink{
		database: func(reader io.Reader, size int64) error {
			return builder.WriteDatabase(ctx, reader, size)
		},
		secret: func(record secretRecord) error {
			defer clear(record.Value)
			return builder.RestoreSecret(ctx, storage.RestoredSecret{ID: record.ID, Name: record.Name,
				ResourceVersion: record.ResourceVersion, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}, record.Value)
		},
	})
	if err != nil {
		return RestoreResult{}, fmt.Errorf("stage restored generation: %w", err)
	}
	if err := builder.Seal(ctx); err != nil {
		return RestoreResult{}, fmt.Errorf("seal restored generation: %w", err)
	}
	if err := builder.Activate(); err != nil {
		return RestoreResult{}, fmt.Errorf("activate restored generation: %w", err)
	}
	return RestoreResult{Verification: verification, Generation: builder.Generation()}, nil
}
