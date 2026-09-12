package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"unicode/utf8"

	"golang.org/x/term"

	"github.com/sxamx/modelcairn/internal/storage"
)

func runSecret(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, interactive bool) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "secret requires set, metadata, rotate, or delete")
		return 2
	}
	switch args[0] {
	case "set":
		return runSecretSet(ctx, args[1:], stdin, stdout, stderr, interactive)
	case "metadata":
		return runSecretMetadata(ctx, args[1:], stdout, stderr)
	case "rotate":
		return runSecretRotate(ctx, args[1:], stdout, stderr)
	case "delete":
		return runSecretDelete(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown secret command %q\n", args[0])
		return 2
	}
}

func secretFlags(name string, stderr io.Writer) (*flag.FlagSet, *string) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	return flags, flags.String("data-dir", defaultDataDirectory(), "private ModelCairn data directory")
}

func runSecretSet(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, interactive bool) int {
	flags, dataDir := secretFlags("secret set", stderr)
	server := flags.String("server", "", "administrative server origin")
	sessionFile := flags.String("session-file", "", "private online session file")
	version := flags.Int64("version", 0, "expected version when replacing a secret")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 1 || *version < 0 {
		fmt.Fprintln(stderr, "usage: modelcairn secret set [--data-dir path] [--version n] <name>")
		return 2
	}
	if !validOnlineFlagMix(flags, *server, *sessionFile) {
		fmt.Fprintln(stderr, "--server cannot be combined with --data-dir; --session-file requires --server")
		return 2
	}
	value, err := readSecretInput(stdin, stderr, interactive)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	defer clear(value)
	if *server != "" {
		return runSecretSetOnline(ctx, *server, *sessionFile, flags.Arg(0), *version, value, stdout, stderr)
	}
	installation, err := storage.OpenInstallation(ctx, *dataDir)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	defer installation.Close()
	metadata, err := installation.Secrets().Put(ctx, storage.PutSecret{Name: flags.Arg(0), Value: value, ExpectedVersion: *version}, storage.Actor{Type: "cli"})
	if err != nil {
		return writeCLIError(stderr, err)
	}
	return writeRedactedJSON(stdout, stderr, installation.Secrets().Redactor(), metadata)
}

func runSecretMetadata(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags, dataDir := secretFlags("secret metadata", stderr)
	server := flags.String("server", "", "administrative server origin")
	sessionFile := flags.String("session-file", "", "private online session file")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 1 {
		fmt.Fprintln(stderr, "usage: modelcairn secret metadata [--data-dir path] [name]")
		return 2
	}
	if !validOnlineFlagMix(flags, *server, *sessionFile) {
		fmt.Fprintln(stderr, "--server cannot be combined with --data-dir; --session-file requires --server")
		return 2
	}
	if *server != "" {
		name := ""
		if flags.NArg() == 1 {
			name = flags.Arg(0)
		}
		return runSecretMetadataOnline(ctx, *server, *sessionFile, name, stdout, stderr)
	}
	installation, err := storage.OpenInstallation(ctx, *dataDir)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	defer installation.Close()
	if flags.NArg() == 1 {
		item, err := installation.Secrets().GetMetadata(ctx, flags.Arg(0))
		if err != nil {
			return writeCLIError(stderr, err)
		}
		return writeRedactedJSON(stdout, stderr, installation.Secrets().Redactor(), item)
	}
	items, err := installation.Secrets().ListMetadata(ctx)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	return writeRedactedJSON(stdout, stderr, installation.Secrets().Redactor(), items)
}

func runSecretRotate(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags, dataDir := secretFlags("secret rotate", stderr)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "secret rotate accepts no positional arguments")
		return 2
	}
	installation, err := storage.OpenInstallation(ctx, *dataDir)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	defer installation.Close()
	version, err := installation.Secrets().RotateMasterKey(ctx, storage.Actor{Type: "cli"})
	if err != nil {
		return writeCLIError(stderr, err)
	}
	fmt.Fprintf(stdout, "master key rotated to version %d\n", version)
	return 0
}

func runSecretDelete(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags, dataDir := secretFlags("secret delete", stderr)
	server := flags.String("server", "", "administrative server origin")
	sessionFile := flags.String("session-file", "", "private online session file")
	version := flags.Int64("version", 0, "expected secret version")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 1 || *version < 1 {
		fmt.Fprintln(stderr, "usage: modelcairn secret delete --version n [--data-dir path] <name>")
		return 2
	}
	if !validOnlineFlagMix(flags, *server, *sessionFile) {
		fmt.Fprintln(stderr, "--server cannot be combined with --data-dir; --session-file requires --server")
		return 2
	}
	if *server != "" {
		return runSecretDeleteOnline(ctx, *server, *sessionFile, flags.Arg(0), *version, stdout, stderr)
	}
	installation, err := storage.OpenInstallation(ctx, *dataDir)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	defer installation.Close()
	if err := installation.Secrets().Delete(ctx, flags.Arg(0), *version, storage.Actor{Type: "cli"}); err != nil {
		return writeCLIError(stderr, err)
	}
	fmt.Fprintln(stdout, "secret deleted")
	return 0
}

func runSecretSetOnline(ctx context.Context, server, sessionFile, name string, version int64, value []byte, stdout, stderr io.Writer) int {
	payload, err := json.Marshal(map[string]string{"value": string(value)})
	if err != nil {
		return writeCLIError(stderr, err)
	}
	defer clear(payload)
	client, session, path, err := onlineSessionClient(server, sessionFile)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	headers := map[string]string{}
	if version > 0 {
		headers["If-Match"] = `"` + strconv.FormatInt(version, 10) + `"`
	}
	response, data, err := client.authenticatedWithRecovery(ctx, &session, path, http.MethodPut, "/api/v1/admin/secrets/"+url.PathEscape(name), payload, "application/json", headers)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	want := http.StatusCreated
	if version > 0 {
		want = http.StatusOK
	}
	if response.StatusCode != want {
		return writeCLIError(stderr, fmt.Errorf("secret_http_%d", response.StatusCode))
	}
	var metadata storage.SecretMetadata
	if json.Unmarshal(data, &metadata) != nil || metadata.Name == "" || metadata.ResourceVersion < 1 {
		return writeCLIError(stderr, errors.New("invalid_admin_response"))
	}
	return encodeCLIJSON(stdout, stderr, metadata)
}

func runSecretMetadataOnline(ctx context.Context, server, sessionFile, name string, stdout, stderr io.Writer) int {
	client, session, path, err := onlineSessionClient(server, sessionFile)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	if name != "" {
		response, data, err := client.authenticatedWithRecovery(ctx, &session, path, http.MethodGet, "/api/v1/admin/secrets/"+url.PathEscape(name), nil, "")
		if err != nil {
			return writeCLIError(stderr, err)
		}
		if response.StatusCode != http.StatusOK {
			return writeCLIError(stderr, fmt.Errorf("secret_http_%d", response.StatusCode))
		}
		var metadata storage.SecretMetadata
		if json.Unmarshal(data, &metadata) != nil || metadata.Name == "" {
			return writeCLIError(stderr, errors.New("invalid_admin_response"))
		}
		return encodeCLIJSON(stdout, stderr, metadata)
	}
	items := []storage.SecretMetadata{}
	cursor := ""
	for {
		endpoint := "/api/v1/admin/secrets?limit=200"
		if cursor != "" {
			endpoint += "&cursor=" + url.QueryEscape(cursor)
		}
		response, data, err := client.authenticatedWithRecovery(ctx, &session, path, http.MethodGet, endpoint, nil, "")
		if err != nil {
			return writeCLIError(stderr, err)
		}
		if response.StatusCode != http.StatusOK {
			return writeCLIError(stderr, fmt.Errorf("secret_http_%d", response.StatusCode))
		}
		var page struct {
			Items      []storage.SecretMetadata `json:"items"`
			NextCursor *string                  `json:"nextCursor"`
		}
		if json.Unmarshal(data, &page) != nil || page.Items == nil {
			return writeCLIError(stderr, errors.New("invalid_admin_response"))
		}
		items = append(items, page.Items...)
		if len(items) > 10000 {
			return writeCLIError(stderr, errors.New("admin_response_too_large"))
		}
		if page.NextCursor == nil {
			break
		}
		if *page.NextCursor == "" || len(*page.NextCursor) > 512 || *page.NextCursor == cursor {
			return writeCLIError(stderr, errors.New("invalid_admin_response"))
		}
		cursor = *page.NextCursor
	}
	return encodeCLIJSON(stdout, stderr, items)
}

func runSecretDeleteOnline(ctx context.Context, server, sessionFile, name string, version int64, stdout, stderr io.Writer) int {
	client, session, path, err := onlineSessionClient(server, sessionFile)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	headers := map[string]string{"If-Match": `"` + strconv.FormatInt(version, 10) + `"`}
	response, _, err := client.authenticatedWithRecovery(ctx, &session, path, http.MethodDelete, "/api/v1/admin/secrets/"+url.PathEscape(name), nil, "", headers)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	if response.StatusCode != http.StatusNoContent {
		return writeCLIError(stderr, fmt.Errorf("secret_http_%d", response.StatusCode))
	}
	fmt.Fprintln(stdout, "secret deleted")
	return 0
}

func readSecretInput(input io.Reader, stderr io.Writer, interactive bool) ([]byte, error) {
	var value []byte
	var err error
	if interactive {
		file, ok := input.(*os.File)
		if !ok || !term.IsTerminal(int(file.Fd())) {
			return nil, errors.New("interactive_terminal_required")
		}
		fmt.Fprint(stderr, "Secret value: ")
		value, err = term.ReadPassword(int(file.Fd()))
		fmt.Fprintln(stderr)
	} else {
		// Read the maximum value, an optional CRLF terminator, and one extra
		// byte so trailing input can never be silently ignored.
		value, err = io.ReadAll(io.LimitReader(input, 16387))
		value = bytes.TrimSuffix(value, []byte("\n"))
		value = bytes.TrimSuffix(value, []byte("\r"))
	}
	if err != nil {
		clear(value)
		return nil, err
	}
	if len(value) < 8 || len(value) > 16384 || !utf8.Valid(value) {
		clear(value)
		return nil, errors.New("invalid_secret")
	}
	return value, nil
}

func writeJSON(stdout, stderr io.Writer, value any) int {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return writeCLIError(stderr, err)
	}
	return 0
}

type byteRedactor interface {
	Bytes([]byte) []byte
}

func writeRedactedJSON(stdout, stderr io.Writer, redactor byteRedactor, value any) int {
	var output bytes.Buffer
	if code := writeJSON(&output, stderr, value); code != 0 {
		return code
	}
	if _, err := stdout.Write(redactor.Bytes(output.Bytes())); err != nil {
		return writeCLIError(stderr, err)
	}
	return 0
}
