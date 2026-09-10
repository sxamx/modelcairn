package storage

import (
	"context"
	"database/sql"
	"errors"
	"net/netip"
	"sync"
	"time"

	"github.com/sxamx/modelcairn/internal/adminauth"
	"github.com/sxamx/modelcairn/internal/adminsettings"
)

const (
	LoginInvalidCredentials = "invalid_credentials"
	LoginThrottled          = "throttled"
	LoginMalformed          = "malformed"
	LoginUnavailable        = "unavailable"
)

type LoginError struct {
	Code       string
	RetryAfter time.Duration
}

func (e *LoginError) Error() string { return e.Code }
func IsLoginCode(err error, code string) bool {
	var target *LoginError
	return errors.As(err, &target) && target.Code == code
}

type loginBucket struct {
	tokens  float64
	updated time.Time
}
type loginClient struct {
	bucket                 loginBucket
	failures               uint8
	blockedUntil, timeSeen time.Time
}

type loginAdmission struct {
	mu                                                           sync.Mutex
	global                                                       loginBucket
	clients                                                      map[netip.Addr]*loginClient
	globalRate, globalBurst, clientRate, clientBurst, maxClients int
	clientIdle                                                   time.Duration
}

type AdminLoginService struct {
	installation *Installation
	effective    adminsettings.Resolved
	dummyPHC     string
	admission    *loginAdmission
	derive       chan struct{}
	now          func() time.Time
	verify       func([]byte, string) (bool, error)
}

func NewAdminLoginService(i *Installation, effective adminsettings.Resolved) (*AdminLoginService, error) {
	if i == nil {
		return nil, errors.New("installation_required")
	}
	if err := adminsettings.Validate(effective); err != nil {
		return nil, err
	}
	dummy, err := adminauth.Hash([]byte("modelcairn dummy credential"), passwordParameters(effective))
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	return &AdminLoginService{installation: i, effective: effective, dummyPHC: dummy, derive: make(chan struct{}, 1), now: time.Now, verify: adminauth.Verify, admission: &loginAdmission{
		global: loginBucket{tokens: float64(effective.GlobalBurst), updated: now}, clients: make(map[netip.Addr]*loginClient), globalRate: effective.GlobalAttemptsPerMinute, globalBurst: effective.GlobalBurst, clientRate: effective.ClientAttemptsPerMinute, clientBurst: effective.ClientBurst, maxClients: effective.MaxClientEntries, clientIdle: time.Duration(effective.ClientIdleSeconds) * time.Second,
	}}, nil
}

func (s *AdminLoginService) Login(ctx context.Context, client netip.Addr, username string, password []byte) (AdminSessionCredentials, error) {
	if !client.IsValid() || client.IsUnspecified() || ValidateAdminUsername(username) != nil || adminauth.ValidatePassword(password) != nil {
		return AdminSessionCredentials{}, &LoginError{Code: LoginMalformed}
	}
	now := s.now().UTC()
	if retry, ok := s.admission.allow(client, now); !ok {
		return AdminSessionCredentials{}, &LoginError{Code: LoginThrottled, RetryAfter: retry}
	}
	if ctx.Err() != nil {
		return AdminSessionCredentials{}, &LoginError{Code: LoginUnavailable}
	}
	// Bound the entire attempt before waiting for the single SQLite connection.
	// Otherwise concurrent requests can queue behind persistence before admission.
	select {
	case s.derive <- struct{}{}:
	default:
		return AdminSessionCredentials{}, &LoginError{Code: LoginThrottled, RetryAfter: time.Second}
	}
	defer func() { <-s.derive }()
	var verified VerifiedAdmin
	var phc string
	err := s.installation.DB().QueryRowContext(ctx, "SELECT id,username,password_phc,auth_version FROM admin_users WHERE username=?", username).Scan(&verified.ID, &verified.Username, &phc, &verified.AuthVersion)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			phc = s.dummyPHC
			verified = VerifiedAdmin{}
		} else {
			return AdminSessionCredentials{}, &LoginError{Code: LoginUnavailable}
		}
	}
	if ctx.Err() != nil {
		return AdminSessionCredentials{}, &LoginError{Code: LoginUnavailable}
	}
	ok, err := s.verify(password, phc)
	if err != nil {
		return AdminSessionCredentials{}, &LoginError{Code: LoginUnavailable}
	}
	if !ok || verified.ID == "" {
		s.admission.failed(client, s.now().UTC())
		return AdminSessionCredentials{}, &LoginError{Code: LoginInvalidCredentials}
	}
	sessionAt := s.now().UTC()
	credentials, err := CreateAdminSession(ctx, s.installation, verified, s.effective.IdleSeconds, s.effective.AbsoluteSeconds, sessionAt)
	if err != nil {
		if IsRepositoryCode(err, CodeVersionConflict) {
			s.admission.failed(client, s.now().UTC())
			return AdminSessionCredentials{}, &LoginError{Code: LoginInvalidCredentials}
		}
		return AdminSessionCredentials{}, &LoginError{Code: LoginUnavailable}
	}
	s.admission.succeeded(client, sessionAt)
	return credentials, nil
}

func (a *loginAdmission) allow(address netip.Addr, now time.Time) (time.Duration, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	address = address.Unmap()
	a.prune(now)
	client, exists := a.clients[address]
	if !exists {
		if len(a.clients) >= a.maxClients {
			return a.clientIdle, false
		}
		client = &loginClient{bucket: loginBucket{tokens: float64(a.clientBurst), updated: now}}
		a.clients[address] = client
	}
	client.timeSeen = now
	if now.Before(client.blockedUntil) {
		return ceilSecond(client.blockedUntil.Sub(now)), false
	}
	refill(&a.global, a.globalRate, a.globalBurst, now)
	refill(&client.bucket, a.clientRate, a.clientBurst, now)
	if a.global.tokens < 1 {
		return tokenRetry(a.global, a.globalRate, now), false
	}
	if client.bucket.tokens < 1 {
		return tokenRetry(client.bucket, a.clientRate, now), false
	}
	a.global.tokens--
	client.bucket.tokens--
	return 0, true
}
func (a *loginAdmission) failed(address netip.Addr, now time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	client := a.clients[address.Unmap()]
	if client == nil {
		return
	}
	if client.failures < 6 {
		client.failures++
	}
	delay := time.Second << max(0, int(client.failures)-1)
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}
	client.blockedUntil = now.Add(delay)
	client.timeSeen = now
}
func (a *loginAdmission) succeeded(address netip.Addr, now time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if client := a.clients[address.Unmap()]; client != nil {
		client.failures = 0
		client.blockedUntil = time.Time{}
		client.timeSeen = now
	}
}
func (a *loginAdmission) prune(now time.Time) {
	for key, client := range a.clients {
		if !now.Before(client.timeSeen.Add(a.clientIdle)) {
			delete(a.clients, key)
		}
	}
}
func refill(bucket *loginBucket, rate, burst int, now time.Time) {
	if bucket.updated.IsZero() {
		bucket.tokens = float64(burst)
		bucket.updated = now
		return
	}
	if now.After(bucket.updated) {
		bucket.tokens += now.Sub(bucket.updated).Minutes() * float64(rate)
		if bucket.tokens > float64(burst) {
			bucket.tokens = float64(burst)
		}
		bucket.updated = now
	}
}
func tokenRetry(bucket loginBucket, rate int, now time.Time) time.Duration {
	if rate < 1 {
		return time.Minute
	}
	missing := 1 - bucket.tokens
	return ceilSecond(time.Duration(missing / float64(rate) * float64(time.Minute)))
}
func ceilSecond(value time.Duration) time.Duration {
	if value <= 0 {
		return time.Second
	}
	return ((value + time.Second - 1) / time.Second) * time.Second
}
