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

	"github.com/sxamx/modelcairn/internal/adminauth"
	"github.com/sxamx/modelcairn/internal/adminsettings"
	"github.com/sxamx/modelcairn/internal/storage"
)

func runAdmin(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, interactive bool) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "admin requires bootstrap or reset-password")
		return 2
	}
	switch args[0] {
	case "bootstrap":
		return runAdminBootstrap(ctx, args[1:], stdin, stdout, stderr, interactive)
	case "reset-password":
		return runAdminResetPassword(ctx, args[1:], stdin, stdout, stderr, interactive)
	default:
		fmt.Fprintf(stderr, "unknown admin command %q\n", args[0])
		return 2
	}
}

func runAdminBootstrap(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, interactive bool) int {
	flags := flag.NewFlagSet("admin bootstrap", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dataDir := flags.String("data-dir", defaultDataDirectory(), "private ModelCairn data directory")
	username := flags.String("username", "", "initial administrator username")
	settingsPath := flags.String("settings", "", "initial AdminSettings YAML or JSON file")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 || *username == "" || *settingsPath == "" {
		fmt.Fprintln(stderr, "usage: modelcairn admin bootstrap --username name --settings file [--data-dir path]")
		return 2
	}
	spec, err := readInitialAdminSettings(*settingsPath)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	password, err := readPasswordInput(stdin, stderr, interactive, true)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	defer clear(password)
	i, err := storage.OpenInstallation(ctx, *dataDir)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	defer i.Close()
	if _, _, err := storage.BootstrapAdmin(ctx, i, *username, password, spec); err != nil {
		return writeCLIError(stderr, err)
	}
	fmt.Fprintln(stdout, "administrator and initial settings created")
	return 0
}

func runAdminResetPassword(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, interactive bool) int {
	flags := flag.NewFlagSet("admin reset-password", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dataDir := flags.String("data-dir", defaultDataDirectory(), "private ModelCairn data directory")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: modelcairn admin reset-password [--data-dir path]")
		return 2
	}
	password, err := readPasswordInput(stdin, stderr, interactive, true)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	defer clear(password)
	i, err := storage.OpenInstallation(ctx, *dataDir)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	defer i.Close()
	if _, err := storage.ResetAdminPassword(ctx, i, password); err != nil {
		return writeCLIError(stderr, err)
	}
	fmt.Fprintln(stdout, "administrator password reset; existing sessions revoked")
	return 0
}

func readInitialAdminSettings(path string) (adminsettings.Resolved, error) {
	file, err := os.Open(path)
	if err != nil {
		return adminsettings.Resolved{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, adminsettings.MaxInputBytes+1))
	if err != nil {
		return adminsettings.Resolved{}, err
	}
	doc, err := adminsettings.Parse(data, adminsettings.Initial)
	if err != nil {
		return adminsettings.Resolved{}, err
	}
	return adminsettings.ResolveInitial(doc)
}

func readPasswordInput(input io.Reader, stderr io.Writer, interactive, confirm bool) ([]byte, error) {
	if interactive {
		file, ok := input.(*os.File)
		if !ok || !term.IsTerminal(int(file.Fd())) {
			return nil, errors.New("interactive_terminal_required")
		}
		fmt.Fprint(stderr, "Administrator password: ")
		value, err := term.ReadPassword(int(file.Fd()))
		fmt.Fprintln(stderr)
		if err != nil {
			clear(value)
			return nil, err
		}
		if confirm {
			fmt.Fprint(stderr, "Confirm password: ")
			again, confirmErr := term.ReadPassword(int(file.Fd()))
			fmt.Fprintln(stderr)
			if confirmErr != nil || !bytes.Equal(value, again) {
				clear(value)
				clear(again)
				return nil, errors.New("password_confirmation_mismatch")
			}
			clear(again)
		}
		if err := adminauth.ValidatePassword(value); err != nil {
			clear(value)
			return nil, err
		}
		return value, nil
	}
	value, err := io.ReadAll(io.LimitReader(input, adminauth.MaxPasswordBytes+3))
	if err != nil {
		clear(value)
		return nil, err
	}
	value = bytes.TrimSuffix(value, []byte("\n"))
	value = bytes.TrimSuffix(value, []byte("\r"))
	if err := adminauth.ValidatePassword(value); err != nil {
		clear(value)
		return nil, err
	}
	return value, nil
}
