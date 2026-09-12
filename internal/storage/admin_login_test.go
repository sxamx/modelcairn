package storage

import (
	"context"
	"errors"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"
)

func TestAdminLoginUniformFailureAndSuccess(t *testing.T) {
	ctx := context.Background()
	i, admin := sessionFixture(t)
	defer i.Close()
	settings, err := ReadAdminSettings(ctx, i.DB())
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewAdminLoginService(i, settings.Spec)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	now := time.Now().UTC()
	service.now = func() time.Time { return now }
	service.statistics.now = func() time.Time { return now }
	unknown := netip.MustParseAddr("192.0.2.1")
	wrong := netip.MustParseAddr("192.0.2.2")
	correct := netip.MustParseAddr("192.0.2.3")
	if _, err := service.Login(ctx, unknown, "missing", []byte("incorrect password")); !IsLoginCode(err, LoginInvalidCredentials) {
		t.Fatalf("unknown=%v", err)
	}
	if _, err := service.Login(ctx, wrong, admin.Username, []byte("incorrect password")); !IsLoginCode(err, LoginInvalidCredentials) {
		t.Fatalf("wrong=%v", err)
	}
	credentials, err := service.Login(ctx, correct, admin.Username, []byte("a secure password"))
	if err != nil {
		t.Fatal(err)
	}
	if credentials.Admin.ID != admin.ID || credentials.SessionToken == "" || credentials.CSRFToken == "" {
		t.Fatal("successful login missing credentials")
	}
	if _, err := UseAdminSession(ctx, i, credentials.SessionToken, credentials.CSRFToken, true, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	service.Close()
	statistics, err := ListFailedLoginStatistics(ctx, i.DB(), now.Add(-time.Minute), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(statistics) != 1 || statistics[0].Reason != LoginInvalidCredentials || statistics[0].Count != 2 {
		t.Fatalf("login statistics=%+v", statistics)
	}
}

func TestAdminLoginBackoffAndInputBounds(t *testing.T) {
	ctx := context.Background()
	i, admin := sessionFixture(t)
	defer i.Close()
	settings, _ := ReadAdminSettings(ctx, i.DB())
	service, err := NewAdminLoginService(i, settings.Spec)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	now := time.Now().UTC()
	service.now = func() time.Time { return now }
	client := netip.MustParseAddr("192.0.2.10")
	if _, err := service.Login(ctx, client, admin.Username, []byte("incorrect password")); !IsLoginCode(err, LoginInvalidCredentials) {
		t.Fatal(err)
	}
	if _, err := service.Login(ctx, client, admin.Username, []byte("a secure password")); !IsLoginCode(err, LoginThrottled) {
		t.Fatalf("backoff=%v", err)
	} else {
		var loginErr *LoginError
		if !errors.As(err, &loginErr) || loginErr.RetryAfter != time.Second {
			t.Fatalf("retry=%v", loginErr)
		}
	}
	now = now.Add(time.Second)
	if _, err := service.Login(ctx, client, admin.Username, []byte("a secure password")); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Login(ctx, netip.Addr{}, admin.Username, []byte("a secure password")); !IsLoginCode(err, LoginMalformed) {
		t.Fatal(err)
	}
	if _, err := service.Login(ctx, netip.MustParseAddr("192.0.2.11"), "", []byte("a secure password")); !IsLoginCode(err, LoginMalformed) {
		t.Fatal(err)
	}
}

func TestLoginAdmissionRateAndBoundedClients(t *testing.T) {
	t0 := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	a := &loginAdmission{global: loginBucket{tokens: 5, updated: t0}, clients: map[netip.Addr]*loginClient{}, globalRate: 30, globalBurst: 5, clientRate: 5, clientBurst: 3, maxClients: 2, clientIdle: 15 * time.Minute}
	first := netip.MustParseAddr("192.0.2.1")
	for range 3 {
		if _, ok := a.allow(first, t0); !ok {
			t.Fatal("initial burst rejected")
		}
	}
	if retry, ok := a.allow(first, t0); ok || retry != 12*time.Second {
		t.Fatalf("rate ok=%v retry=%v", ok, retry)
	}
	if _, ok := a.allow(first, t0.Add(12*time.Second)); !ok {
		t.Fatal("refilled token rejected")
	}
	second := netip.MustParseAddr("192.0.2.2")
	third := netip.MustParseAddr("192.0.2.3")
	if _, ok := a.allow(second, t0.Add(12*time.Second)); !ok {
		t.Fatal("second client rejected")
	}
	if _, ok := a.allow(third, t0.Add(12*time.Second)); ok {
		t.Fatal("client bound exceeded")
	}
	if _, ok := a.allow(third, t0.Add(16*time.Minute)); !ok {
		t.Fatal("expired clients not pruned")
	}
}

func TestAdminLoginAllowsOnlyOneDerivation(t *testing.T) {
	ctx := context.Background()
	i, admin := sessionFixture(t)
	defer i.Close()
	settings, _ := ReadAdminSettings(ctx, i.DB())
	service, err := NewAdminLoginService(i, settings.Spec)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	entered := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	service.verify = func(password []byte, phc string) (bool, error) {
		calls.Add(1)
		close(entered)
		<-release
		return true, nil
	}
	done := make(chan error, 1)
	go func() {
		_, err := service.Login(ctx, netip.MustParseAddr("192.0.2.20"), admin.Username, []byte("a secure password"))
		done <- err
	}()
	<-entered
	if _, err := service.Login(ctx, netip.MustParseAddr("192.0.2.21"), admin.Username, []byte("a secure password")); !IsLoginCode(err, LoginThrottled) {
		t.Fatalf("concurrent=%v", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("derivations=%d", calls.Load())
	}
}

func TestBusyLoginRejectsBeforeWaitingForDatabase(t *testing.T) {
	i, admin := sessionFixture(t)
	defer i.Close()
	settings, err := ReadAdminSettings(context.Background(), i.DB())
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewAdminLoginService(i, settings.Spec)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// An existing login owns the admission permit while another operation owns DB.
	s.derive <- struct{}{}
	defer func() { <-s.derive }()
	tx, err := i.DB().BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = s.Login(ctx, netip.MustParseAddr("192.0.2.30"), admin.Username, []byte("a secure password"))
	if !IsLoginCode(err, LoginThrottled) {
		t.Fatalf("queued before admission: %v", err)
	}
}
