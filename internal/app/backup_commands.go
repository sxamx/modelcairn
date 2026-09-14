package app

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"

	"github.com/sxamx/modelcairn/internal/backupmcb1"
	"github.com/sxamx/modelcairn/internal/buildinfo"
	"github.com/sxamx/modelcairn/internal/storage"
)

const maxBackupPassphraseInput = 1024

func runBackup(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, interactive bool) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "backup requires create, verify, or restore")
		return 2
	}
	switch args[0] {
	case "create":
		return runBackupCreate(ctx, args[1:], stdin, stdout, stderr, interactive)
	case "verify":
		return runBackupVerify(ctx, args[1:], stdin, stdout, stderr, interactive)
	case "restore":
		return runBackupRestore(ctx, args[1:], stdin, stdout, stderr, interactive)
	default:
		fmt.Fprintln(stderr, "unknown backup command")
		return 2
	}
}

func runBackupRestore(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, interactive bool) int {
	flags := flag.NewFlagSet("backup restore", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	dataDir := flags.String("data-dir", defaultDataDirectory(), "private ModelCairn data directory")
	if flags.Parse(args) != nil || flags.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: modelcairn backup restore [--data-dir path] file.mcb.age")
		return 2
	}
	passphrase, err := readBackupPassphrase(stdin, stderr, interactive, false)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	defer clear(passphrase)
	result, err := backupmcb1.Restore(ctx, flags.Arg(0), passphrase, *dataDir)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	fmt.Fprintf(stdout, "backup restored and activated: generation=%s schema=%d secrets=%d; restart ModelCairn and verify readiness\n",
		result.Generation, result.Manifest.SchemaVersion, result.Secrets)
	return 0
}

func runBackupCreate(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, interactive bool) int {
	flags := flag.NewFlagSet("backup create", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	dataDir := flags.String("data-dir", defaultDataDirectory(), "private ModelCairn data directory")
	output := flags.String("out", "", "new .mcb.age output path")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *output == "" {
		fmt.Fprintln(stderr, "usage: modelcairn backup create --out file.mcb.age [--data-dir path]")
		return 2
	}
	passphrase, err := readBackupPassphrase(stdin, stderr, interactive, true)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	defer clear(passphrase)
	installation, err := storage.OpenInstallation(ctx, *dataDir)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	defer installation.Close()
	verification, err := backupmcb1.Create(ctx, backupmcb1.CreateOptions{Installation: installation,
		Destination: *output, Passphrase: passphrase, ApplicationVersion: buildinfo.Version})
	if err != nil {
		return writeCLIError(stderr, err)
	}
	fmt.Fprintf(stdout, "backup created and verified: profile=%s version=%d schema=%d secrets=%d\n",
		verification.Manifest.Profile, verification.Manifest.ProfileVersion, verification.Manifest.SchemaVersion, verification.Secrets)
	return 0
}

func runBackupVerify(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, interactive bool) int {
	flags := flag.NewFlagSet("backup verify", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	if flags.Parse(args) != nil || flags.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: modelcairn backup verify file.mcb.age")
		return 2
	}
	passphrase, err := readBackupPassphrase(stdin, stderr, interactive, false)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	defer clear(passphrase)
	verification, err := backupmcb1.Verify(ctx, flags.Arg(0), passphrase)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	fmt.Fprintf(stdout, "backup verified: profile=%s version=%d schema=%d secrets=%d created=%s\n",
		verification.Manifest.Profile, verification.Manifest.ProfileVersion, verification.Manifest.SchemaVersion,
		verification.Secrets, verification.Manifest.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	return 0
}

func readBackupPassphrase(input io.Reader, stderr io.Writer, interactive, confirm bool) ([]byte, error) {
	if interactive {
		file, ok := input.(*os.File)
		if !ok || !term.IsTerminal(int(file.Fd())) {
			return nil, errors.New("interactive_terminal_required")
		}
		fmt.Fprint(stderr, "Backup passphrase: ")
		value, err := term.ReadPassword(int(file.Fd()))
		fmt.Fprintln(stderr)
		if err != nil {
			clear(value)
			return nil, errors.New("passphrase_input_unavailable")
		}
		if confirm {
			fmt.Fprint(stderr, "Confirm backup passphrase: ")
			again, confirmationErr := term.ReadPassword(int(file.Fd()))
			fmt.Fprintln(stderr)
			if confirmationErr != nil || !bytes.Equal(value, again) {
				clear(value)
				clear(again)
				return nil, errors.New("passphrase_confirmation_mismatch")
			}
			clear(again)
		}
		if len(value) < 8 || len(value) > maxBackupPassphraseInput {
			clear(value)
			return nil, errors.New("backup_passphrase_invalid")
		}
		return value, nil
	}
	value, err := io.ReadAll(io.LimitReader(input, maxBackupPassphraseInput+3))
	if err != nil {
		clear(value)
		return nil, errors.New("passphrase_input_unavailable")
	}
	value = bytes.TrimSuffix(value, []byte("\n"))
	value = bytes.TrimSuffix(value, []byte("\r"))
	if len(value) < 8 || len(value) > maxBackupPassphraseInput {
		clear(value)
		return nil, errors.New("backup_passphrase_invalid")
	}
	return value, nil
}
