package app

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sxamx/modelcairn/internal/storage"
)

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("Run() code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "modelcairn") {
		t.Fatalf("version output %q does not identify ModelCairn", stdout.String())
	}
}

func TestServiceAndOfflineOwnerContendInBothOrders(t *testing.T) {
	t.Run("offline_owner_first", func(t *testing.T) {
		dir := t.TempDir()
		owner, err := storage.AcquireLock(dir)
		if err != nil {
			t.Fatal(err)
		}
		defer owner.Close()
		var stdout, stderr bytes.Buffer
		code := Run(context.Background(), []string{"serve", "--data-dir", dir, "--listen", "127.0.0.1:0"}, &stdout, &stderr)
		if code != 1 || !strings.Contains(stdout.String(), `"code":"installation_in_use"`) {
			t.Fatalf("service contender code=%d output=%s", code, stdout.String())
		}
		if _, err := os.Stat(filepath.Join(dir, "modelcairn.db")); !os.IsNotExist(err) {
			t.Fatalf("losing service touched SQLite: %v", err)
		}
	})

	t.Run("service_owner_first", func(t *testing.T) {
		dir := t.TempDir()
		address := unusedAddress(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var stdout, stderr bytes.Buffer
		done := make(chan int, 1)
		go func() { done <- Run(ctx, []string{"serve", "--data-dir", dir, "--listen", address}, &stdout, &stderr) }()
		waitForListener(t, address, done)
		if _, err := storage.AcquireLock(dir); !errors.Is(err, storage.ErrInstallationInUse) {
			t.Fatalf("offline contender error=%v", err)
		}
		cancel()
		if code := <-done; code != 0 {
			t.Fatalf("service exit=%d output=%s", code, stdout.String())
		}
	})
}

func unusedAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func waitForListener(t *testing.T, address string, done <-chan int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case code := <-done:
			t.Fatalf("service exited before listening with code %d", code)
		default:
		}
		connection, err := net.DialTimeout("tcp", address, 25*time.Millisecond)
		if err == nil {
			_ = connection.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("service did not begin listening")
}

func TestRunServeRejectsPositionalArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"serve", "unexpected"}, &stdout, &stderr); code != 2 {
		t.Fatalf("Run() code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "does not accept positional arguments") {
		t.Fatalf("stderr = %q, want positional-argument message", stderr.String())
	}
}

func TestRunServeReportsBindFailureWithoutStartedEvent(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer listener.Close()

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"serve", "--data-dir", t.TempDir(), "--listen", listener.Addr().String()}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("Run() code = %d, want 1", code)
	}
	if strings.Contains(stdout.String(), `"msg":"http server started"`) {
		t.Fatalf("server logged a false start event: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"msg":"http server bind failed"`) {
		t.Fatalf("bind failure was not logged: %s", stdout.String())
	}
}

func TestRunServeShutsDownAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	address := unusedAddress(t)
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- Run(ctx, []string{"serve", "--data-dir", t.TempDir(), "--listen", address, "--shutdown-timeout", time.Second.String()}, &stdout, &stderr)
	}()
	waitForListener(t, address, done)
	cancel()
	if code := <-done; code != 0 {
		t.Fatalf("Run() code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), `"msg":"http server stopped"`) {
		t.Fatalf("shutdown was not logged: %s", stdout.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"unknown"}, &stdout, &stderr); code != 2 {
		t.Fatalf("Run() code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("stderr = %q, want unknown-command message", stderr.String())
	}
}
