package app

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sxamx/modelcairn/internal/adminauth"
	"github.com/sxamx/modelcairn/internal/adminsettings"
	"github.com/sxamx/modelcairn/internal/config"
	"github.com/sxamx/modelcairn/internal/storage"
)

const maxPlanFileBytes = 4 * config.MaxInputBytes

func runConfig(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, interactive bool) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "config requires validate, plan, apply, or export")
		return 2
	}
	switch args[0] {
	case "validate":
		return runConfigValidate(args[1:], stdout, stderr)
	case "plan":
		return runConfigPlan(ctx, args[1:], stdout, stderr)
	case "apply":
		return runConfigApply(ctx, args[1:], stdin, stdout, stderr, interactive)
	case "export":
		return runConfigExport(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown config command %q\n", args[0])
		return 2
	}
}

func runConfigValidate(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("config validate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: modelcairn config validate <file>")
		return 2
	}
	if _, err := readConfiguration(flags.Arg(0)); err != nil {
		return writeCLIError(stderr, err)
	}
	fmt.Fprintln(stdout, "configuration valid")
	return 0
}

func runConfigPlan(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("config plan", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dataDir := flags.String("data-dir", defaultDataDirectory(), "private ModelCairn data directory")
	allowDelete := flags.Bool("allow-delete", false, "allow explicit state: absent")
	out := flags.String("out", "", "write authenticated JSON plan")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: modelcairn config plan [options] <file>")
		return 2
	}
	doc, err := readConfiguration(flags.Arg(0))
	if err != nil {
		return writeCLIError(stderr, err)
	}
	installation, err := storage.OpenInstallation(ctx, *dataDir)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	defer installation.Close()
	plan, err := config.NewManager(installation).Plan(ctx, doc, *allowDelete)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	encoded, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return writeCLIError(stderr, err)
	}
	encoded = append(encoded, '\n')
	if len(encoded) > maxPlanFileBytes {
		return writeCLIError(stderr, errors.New("plan_too_large"))
	}
	if *out != "" {
		if err := writePrivateFile(*out, encoded); err != nil {
			return writeCLIError(stderr, err)
		}
		fmt.Fprintf(stdout, "plan written to %s\n", *out)
		return 0
	}
	_, err = stdout.Write(encoded)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	return 0
}

func runConfigApply(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, interactive bool) int {
	flags := flag.NewFlagSet("config apply", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dataDir := flags.String("data-dir", defaultDataDirectory(), "private ModelCairn data directory")
	allowDelete := flags.Bool("allow-delete", false, "allow explicit state: absent")
	planPath := flags.String("plan", "", "authenticated plan file")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: modelcairn config apply [options] <file>")
		return 2
	}
	doc, err := readConfiguration(flags.Arg(0))
	if err != nil {
		return writeCLIError(stderr, err)
	}
	if *planPath == "" && !interactive {
		fmt.Fprintln(stderr, "a plan file is required when stdin is not interactive")
		return 2
	}
	var token string
	if *planPath != "" {
		token, err = readPlanToken(*planPath)
		if err != nil {
			return writeCLIError(stderr, err)
		}
	}
	installation, err := storage.OpenInstallation(ctx, *dataDir)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	defer installation.Close()
	manager := config.NewManager(installation)
	if *planPath == "" {
		plan, err := manager.Plan(ctx, doc, *allowDelete)
		if err != nil {
			return writeCLIError(stderr, err)
		}
		token = plan.Token
		for _, change := range plan.Changes {
			fmt.Fprintf(stdout, "%s %s/%s\n", change.Action, change.Kind, change.Name)
		}
		fmt.Fprint(stdout, "Apply this plan? [y/N] ")
		answer, err := bufio.NewReader(io.LimitReader(stdin, 1024)).ReadString('\n')
		if err != nil && err != io.EOF {
			return writeCLIError(stderr, err)
		}
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Fprintln(stdout, "cancelled")
			return 0
		}
	}
	if err := manager.Apply(ctx, token, doc, *allowDelete, storage.Actor{Type: "cli"}); err != nil {
		return writeCLIError(stderr, err)
	}
	fmt.Fprintln(stdout, "configuration applied")
	return 0
}

func readPlanToken(path string) (string, error) {
	raw, err := readBoundedFile(path, maxPlanFileBytes)
	if err != nil {
		return "", err
	}
	var plan config.PersistedPlan
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&plan); err != nil || plan.Token == "" || decoder.Decode(&struct{}{}) != io.EOF {
		return "", storage.ErrInvalidPlan
	}
	return plan.Token, nil
}

func runConfigExport(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("config export", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dataDir := flags.String("data-dir", defaultDataDirectory(), "private ModelCairn data directory")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "config export accepts no positional arguments")
		return 2
	}
	installation, err := storage.OpenInstallation(ctx, *dataDir)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	defer installation.Close()
	raw, err := config.NewManager(installation).Export(ctx)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	if _, err := stdout.Write(raw); err != nil {
		return writeCLIError(stderr, err)
	}
	return 0
}

func readConfiguration(path string) (*config.Document, error) {
	raw, err := readBoundedFile(path, config.MaxInputBytes)
	if err != nil {
		return nil, err
	}
	return config.Parse(raw)
}
func readBoundedFile(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, errors.New("input_too_large")
	}
	return raw, nil
}
func writePrivateFile(path string, data []byte) (result error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	closed, complete := false, false
	defer func() {
		if !closed {
			_ = file.Close()
		}
		if !complete {
			_ = os.Remove(path)
		}
	}()
	if _, err = file.Write(data); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		closed = true
		return err
	}
	closed = true
	complete = true
	return nil
}

func writeCLIError(stderr io.Writer, err error) int {
	code := 1
	if errors.Is(err, storage.ErrInvalidPlan) || errors.Is(err, storage.ErrPlanExpired) || errors.Is(err, storage.ErrPlanAlreadyUsed) {
		code = 4
	}
	if storage.IsRepositoryCode(err, storage.CodeVersionConflict) || storage.IsRepositoryCode(err, storage.CodeAlreadyExists) || storage.IsRepositoryCode(err, storage.CodeNotFound) || storage.IsRepositoryCode(err, storage.CodeResourceInUse) {
		code = 3
	}
	if storage.IsRepositoryCode(err, storage.CodeInvalidResource) || err.Error() == "invalid_secret" || err.Error() == "input_too_large" || err.Error() == "interactive_terminal_required" {
		code = 2
	}
	if errors.Is(err, adminauth.ErrInvalidPassword) || errors.Is(err, adminauth.ErrInvalidParameters) || storage.IsRepositoryCode(err, storage.CodeInvalidUsername) {
		code = 2
	}
	var settingsDiagnostic *adminsettings.Error
	if errors.As(err, &settingsDiagnostic) {
		code = 2
		if len(settingsDiagnostic.Diagnostics) == 0 {
			fmt.Fprintln(stderr, "invalid_admin_settings")
		} else {
			first := settingsDiagnostic.Diagnostics[0]
			fmt.Fprintf(stderr, "%s at %s\n", first.Code, first.Path)
		}
		return code
	}
	var diagnostic *config.Error
	if errors.As(err, &diagnostic) {
		code = 2
		if len(diagnostic.Diagnostics) > 0 && diagnostic.Diagnostics[0].Code == "version_conflict" {
			code = 3
		}
		if len(diagnostic.Diagnostics) == 0 {
			fmt.Fprintln(stderr, "invalid_configuration")
		} else {
			first := diagnostic.Diagnostics[0]
			fmt.Fprintf(stderr, "%s at %s\n", first.Code, first.Path)
		}
		return code
	}
	fmt.Fprintln(stderr, err)
	return code
}
