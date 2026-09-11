package app

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sxamx/modelcairn/internal/adminsettings"
	"github.com/sxamx/modelcairn/internal/storage"
)

func bootstrapTransport(t *testing.T, spec adminsettings.Resolved) string {
	t.Helper()
	dir := t.TempDir()
	i, err := storage.OpenInstallation(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = storage.BootstrapAdmin(context.Background(), i, "owner", []byte("a secure password"), spec)
	closeErr := i.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	return dir
}

func TestServeUsesPersistedListenerAndDirectTLS(t *testing.T) {
	// Reuse httptest's local certificate, trusting it explicitly in this client.
	fixture := httptest.NewTLSServer(http.NotFoundHandler())
	cert := fixture.TLS.Certificates[0]
	fixture.Close()
	certDir := t.TempDir()
	spec := adminsettings.Defaults()
	spec.Listen = unusedAddress(t)
	spec.PublicOrigin = "https://" + spec.Listen
	spec.Transport = adminsettings.DirectTLS
	spec.TLSCertificatePath = filepath.Join(certDir, "cert.pem")
	spec.TLSPrivateKeyPath = filepath.Join(certDir, "key.pem")
	key, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(spec.TLSCertificatePath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(spec.TLSPrivateKeyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}), 0600); err != nil {
		t.Fatal(err)
	}
	dir := bootstrapTransport(t, spec)
	ctx, cancel := context.WithCancel(context.Background())
	var out, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() { done <- Run(ctx, []string{"serve", "--data-dir", dir}, &out, &stderr) }()
	defer func() {
		cancel()
		if code := <-done; code != 0 {
			t.Errorf("serve exit=%d output=%s", code, out.String())
		}
	}()
	// Do not consume done here: deferred cleanup is its sole reader.
	pool := x509.NewCertPool()
	parsed, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	pool.AddCert(parsed)
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: time.Second}
	deadline := time.Now().Add(5 * time.Second)
	for {
		response, err := client.Get(spec.PublicOrigin + "/healthz")
		if err == nil {
			response.Body.Close()
			if response.StatusCode != http.StatusOK || response.TLS == nil {
				t.Fatal("configured endpoint did not serve TLS")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("configured TLS listener did not start", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestServeRejectsConflictingOverrideAndMissingTLSMaterial(t *testing.T) {
	spec := adminsettings.Defaults()
	spec.Listen = unusedAddress(t)
	spec.PublicOrigin = "https://" + spec.Listen
	spec.Transport = adminsettings.DirectTLS
	spec.TLSCertificatePath = filepath.Join(t.TempDir(), "missing-certificate-canary.pem")
	spec.TLSPrivateKeyPath = filepath.Join(t.TempDir(), "missing-key-canary.pem")
	dir := bootstrapTransport(t, spec)
	for _, args := range [][]string{
		{"serve", "--data-dir", dir, "--listen", "0.0.0.0:8080"},
		{"serve", "--data-dir", dir},
	} {
		var out, stderr bytes.Buffer
		if code := Run(context.Background(), args, &out, &stderr); code != 1 {
			t.Fatalf("exit=%d", code)
		}
		if strings.Contains(out.String(), "http server started") || strings.Contains(out.String(), "canary") {
			t.Fatal("unsafe startup or path disclosure")
		}
	}
}
