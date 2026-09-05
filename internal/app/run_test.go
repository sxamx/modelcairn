package app

import (
	"bytes"
	"context"
	"net"
	"strings"
	"testing"
	"time"
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
	code := Run(context.Background(), []string{"serve", "--listen", listener.Addr().String()}, &stdout, &stderr)
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
	cancel()
	var stdout, stderr bytes.Buffer
	if code := Run(ctx, []string{"serve", "--listen", "127.0.0.1:0", "--shutdown-timeout", time.Second.String()}, &stdout, &stderr); code != 0 {
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
