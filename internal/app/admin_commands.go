package app

import (
	"bytes"
	"context"
	"encoding/json"
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
		fmt.Fprintln(stderr, "admin requires bootstrap, reset-password, login, whoami, or logout")
		return 2
	}
	switch args[0] {
	case "bootstrap":
		return runAdminBootstrap(ctx, args[1:], stdin, stdout, stderr, interactive)
	case "reset-password":
		return runAdminResetPassword(ctx, args[1:], stdin, stdout, stderr, interactive)
	case "login":
		return runAdminLogin(ctx, args[1:], stdin, stdout, stderr, interactive)
	case "whoami":
		return runAdminWhoami(ctx, args[1:], stdout, stderr)
	case "logout":
		return runAdminLogout(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintln(stderr, "unknown admin command")
		return 2
	}
}

func onlineAdminFlags(name string) (*flag.FlagSet, *string, *string) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	server := flags.String("server", "", "administrative server origin")
	sessionFile := flags.String("session-file", "", "private session file")
	return flags, server, sessionFile
}

func onlineAdminClient(server, sessionFile string) (*adminClient, string, error) {
	client, err := newAdminClient(server)
	if err != nil {
		return nil, "", err
	}
	if sessionFile == "" {
		sessionFile, err = defaultOnlineSessionPath(client.server)
	}
	return client, sessionFile, err
}

func runAdminLogin(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, interactive bool) int {
	flags, server, sessionFile := onlineAdminFlags("admin login")
	username := flags.String("username", "", "administrator username")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *server == "" || *username == "" {
		fmt.Fprintln(stderr, "usage: modelcairn admin login --server origin --username name [--session-file path]")
		return 2
	}
	password, err := readPasswordInput(stdin, stderr, interactive, false)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	defer clear(password)
	client, path, err := onlineAdminClient(*server, *sessionFile)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	session, view, err := client.login(ctx, *username, password)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	if err := saveOnlineSession(path, session); err != nil {
		return writeCLIError(stderr, err)
	}
	fmt.Fprintf(stdout, "authenticated as %s\n", view.Admin.Username)
	return 0
}

func runAdminWhoami(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags, server, sessionFile := onlineAdminFlags("admin whoami")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *server == "" {
		fmt.Fprintln(stderr, "usage: modelcairn admin whoami --server origin [--session-file path]")
		return 2
	}
	client, path, err := onlineAdminClient(*server, *sessionFile)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	session, err := loadOnlineSession(path, client.server)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	response, data, err := client.authenticated(ctx, session, "GET", "/api/v1/admin/session/me", nil, "")
	if err != nil {
		return writeCLIError(stderr, err)
	}
	if response.StatusCode != 200 {
		return writeCLIError(stderr, fmt.Errorf("admin_session_http_%d", response.StatusCode))
	}
	var view onlineSessionView
	if json.Unmarshal(data, &view) != nil || view.CSRFToken == "" {
		return writeCLIError(stderr, errors.New("invalid_admin_response"))
	}
	session.CSRFToken, session.ExpiresAt = view.CSRFToken, view.ExpiresAt
	if err := saveOnlineSession(path, session); err != nil {
		return writeCLIError(stderr, err)
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(view.Admin); err != nil {
		return writeCLIError(stderr, err)
	}
	return 0
}

func runAdminLogout(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags, server, sessionFile := onlineAdminFlags("admin logout")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *server == "" {
		fmt.Fprintln(stderr, "usage: modelcairn admin logout --server origin [--session-file path]")
		return 2
	}
	client, path, err := onlineAdminClient(*server, *sessionFile)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	session, err := loadOnlineSession(path, client.server)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	response, _, err := client.authenticated(ctx, session, "DELETE", "/api/v1/admin/session", nil, "")
	if err != nil {
		return writeCLIError(stderr, err)
	}
	if response.StatusCode != 204 {
		return writeCLIError(stderr, fmt.Errorf("admin_logout_http_%d", response.StatusCode))
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return writeCLIError(stderr, err)
	}
	fmt.Fprintln(stdout, "administrative session revoked")
	return 0
}

func runAdminBootstrap(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, interactive bool) int {
	flags := flag.NewFlagSet("admin bootstrap", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	dataDir := flags.String("data-dir", defaultDataDirectory(), "private ModelCairn data directory")
	username := flags.String("username", "", "initial administrator username")
	settingsPath := flags.String("settings", "", "initial AdminSettings YAML or JSON file")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(stderr, "invalid admin bootstrap arguments; passwords must use terminal or stdin")
		return 2
	}
	if flags.NArg() != 0 || *username == "" || *settingsPath == "" {
		fmt.Fprintln(stderr, "usage: modelcairn admin bootstrap --username name --settings file [--data-dir path]")
		return 2
	}
	if err := storage.ValidateAdminUsername(*username); err != nil {
		return writeCLIError(stderr, err)
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
	flags.SetOutput(io.Discard)
	dataDir := flags.String("data-dir", defaultDataDirectory(), "private ModelCairn data directory")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(stderr, "invalid admin reset-password arguments; passwords must use terminal or stdin")
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
			return nil, errors.New("password_input_unavailable")
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
		return nil, errors.New("password_input_unavailable")
	}
	value = bytes.TrimSuffix(value, []byte("\n"))
	value = bytes.TrimSuffix(value, []byte("\r"))
	if err := adminauth.ValidatePassword(value); err != nil {
		clear(value)
		return nil, err
	}
	return value, nil
}
