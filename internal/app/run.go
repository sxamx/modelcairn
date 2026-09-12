package app

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/term"

	"github.com/sxamx/modelcairn/internal/adminsettings"
	"github.com/sxamx/modelcairn/internal/buildinfo"
	server "github.com/sxamx/modelcairn/internal/httpserver"
	"github.com/sxamx/modelcairn/internal/redact"
	"github.com/sxamx/modelcairn/internal/storage"
)

const (
	defaultAddress         = "127.0.0.1:8080"
	defaultShutdownTimeout = 10 * time.Second
)

// Run executes the CLI and returns a process exit code.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	return run(ctx, args, os.Stdin, stdout, stderr, stdinIsTerminal(os.Stdin))
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, interactive bool) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}

	switch args[0] {
	case "serve":
		return runServe(ctx, args[1:], stdout, stderr)
	case "version":
		fmt.Fprintln(stdout, buildinfo.String())
		return 0
	case "config":
		return runConfig(ctx, args[1:], stdin, stdout, stderr, interactive)
	case "secret":
		return runSecret(ctx, args[1:], stdin, stdout, stderr, interactive)
	case "admin":
		return runAdmin(ctx, args[1:], stdin, stdout, stderr, interactive)
	case "agent-token":
		return runAgentToken(ctx, args[1:], stdout, stderr)
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func stdinIsTerminal(input io.Reader) bool {
	file, ok := input.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

func runServe(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	address := flags.String("listen", defaultAddress, "HTTP listen address")
	shutdownTimeout := flags.Duration("shutdown-timeout", defaultShutdownTimeout, "graceful shutdown timeout")
	dataDir := flags.String("data-dir", defaultDataDirectory(), "private ModelCairn data directory")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "serve does not accept positional arguments: %q\n", flags.Args())
		return 2
	}
	if *shutdownTimeout <= 0 {
		fmt.Fprintln(stderr, "shutdown-timeout must be greater than zero")
		return 2
	}

	logger := slog.New(slog.NewJSONHandler(stdout, nil))
	probe := server.NewReadinessProbe()
	installation, err := storage.OpenInstallation(ctx, *dataDir)
	if err != nil {
		logger.Error("installation startup failed", "code", storageErrorCode(err), "error", err)
		return 1
	}
	defer func() {
		if err := installation.Close(); err != nil {
			logger.Error("installation close failed", "error", err)
		}
	}()
	logger = slog.New(redact.NewHandler(slog.NewJSONHandler(stdout, nil), installation.Secrets().Redactor()))
	probe.Set(server.ComponentPersistence, true, "")
	probe.Set(server.ComponentSecretStore, true, "")
	settings, err := storage.ReadAdminSettings(ctx, installation.DB())
	var httpServer *http.Server
	var transportTLS *tls.Config
	if storage.IsRepositoryCode(err, storage.CodeNotFound) {
		// A fresh installation remains reachable for health checks while local
		// bootstrap is pending; no administrative routes are exposed yet.
		httpServer = server.New(*address, probe, logger)
	} else if err != nil {
		logger.Error("administrative settings unavailable", "code", storageErrorCode(err))
		return 1
	} else {
		listenExplicit := false
		flags.Visit(func(f *flag.Flag) {
			if f.Name == "listen" {
				listenExplicit = true
			}
		})
		if listenExplicit && *address != settings.Spec.Listen {
			logger.Error("listen override conflicts with administrative settings", "code", "configuration_invalid")
			return 1
		}
		*address = settings.Spec.Listen
		if settings.Spec.Transport == adminsettings.DirectTLS {
			certificate, loadErr := tls.LoadX509KeyPair(settings.Spec.TLSCertificatePath, settings.Spec.TLSPrivateKeyPath)
			if loadErr != nil {
				logger.Error("administrative TLS material unavailable", "code", "tls_unavailable")
				return 1
			}
			transportTLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}}
		}
		probe.Set(server.ComponentConfiguration, true, "")
		httpServer, err = server.NewAdmin(*address, probe, logger, installation, settings.Spec)
		if err != nil {
			logger.Error("administrative server initialization failed", "code", "configuration_invalid")
			return 1
		}
	}
	listener, err := net.Listen("tcp", *address)
	if err != nil {
		logger.Error("http server bind failed", "address", *address, "error", err)
		return 1
	}
	if transportTLS != nil {
		listener = tls.NewListener(listener, transportTLS)
	}

	errCh := make(chan error, 1)
	go func() { errCh <- httpServer.Serve(listener) }()
	logger.Info("http server started", "address", listener.Addr().String(), "version", buildinfo.Version)

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server failed", "error", err)
			return 1
		}
		return 0
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), *shutdownTimeout)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("http server shutdown failed", "error", err)
			return 1
		}
		logger.Info("http server stopped")
		return 0
	}
}

func defaultDataDirectory() string {
	if configured := os.Getenv("MODELCAIRN_DATA_DIR"); configured != "" {
		return configured
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(".", ".modelcairn")
	}
	return filepath.Join(base, "modelcairn")
}

func storageErrorCode(err error) string {
	if errors.Is(err, storage.ErrInstallationInUse) {
		return "installation_in_use"
	}
	return "installation_unavailable"
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "ModelCairn — lightweight self-hosted AI provider gateway")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  modelcairn serve [--listen address] [--shutdown-timeout duration] [--data-dir path]")
	fmt.Fprintln(w, "  modelcairn config validate <file>")
	fmt.Fprintln(w, "  modelcairn config plan [--data-dir path] [--allow-delete] [--out file] <file>")
	fmt.Fprintln(w, "  modelcairn config apply [--data-dir path] [--allow-delete] [--plan file] <file>")
	fmt.Fprintln(w, "  modelcairn config export [--data-dir path]")
	fmt.Fprintln(w, "  modelcairn secret set [--data-dir path] [--version n] <name>")
	fmt.Fprintln(w, "  modelcairn secret metadata [--data-dir path] [name]")
	fmt.Fprintln(w, "  modelcairn secret rotate [--data-dir path]")
	fmt.Fprintln(w, "  modelcairn secret delete --version n [--data-dir path] <name>")
	fmt.Fprintln(w, "  modelcairn admin bootstrap --username name --settings file [--data-dir path]")
	fmt.Fprintln(w, "  modelcairn admin reset-password [--data-dir path]")
	fmt.Fprintln(w, "  modelcairn admin login --server origin --username name [--session-file path]")
	fmt.Fprintln(w, "  modelcairn admin whoami --server origin [--session-file path]")
	fmt.Fprintln(w, "  modelcairn admin logout --server origin [--session-file path]")
	fmt.Fprintln(w, "  modelcairn agent-token status --server origin [--session-file path] <name>")
	fmt.Fprintln(w, "  modelcairn agent-token issue --server origin [--session-file path] <name>")
	fmt.Fprintln(w, "  modelcairn agent-token revoke --server origin [--session-file path] <name>")
	fmt.Fprintln(w, "  modelcairn version")
}
