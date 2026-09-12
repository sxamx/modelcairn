package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const adminResponseLimit = 1 << 20

type onlineSession struct {
	Server       string    `json:"server"`
	SessionToken string    `json:"sessionToken"`
	CSRFToken    string    `json:"csrfToken"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

type onlineSessionView struct {
	Admin struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"admin"`
	CSRFToken string    `json:"csrfToken"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type adminClient struct {
	server string
	http   *http.Client
}

func newAdminClient(raw string) (*adminClient, error) {
	origin, err := canonicalAdminOrigin(raw)
	if err != nil {
		return nil, err
	}
	return &adminClient{server: origin, http: &http.Client{
		Timeout:       30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

func canonicalAdminOrigin(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", errors.New("invalid_server_origin")
	}
	parsed.Path = ""
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", errors.New("invalid_server_origin")
	}
	if parsed.Scheme == "http" {
		host := parsed.Hostname()
		ip := net.ParseIP(host)
		if !strings.EqualFold(host, "localhost") && (ip == nil || !ip.IsLoopback()) {
			return "", errors.New("insecure_remote_server")
		}
	}
	return parsed.String(), nil
}

func defaultOnlineSessionPath(server string) (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(server))
	return filepath.Join(base, "modelcairn", "sessions", hex.EncodeToString(digest[:8])+".json"), nil
}

func saveOnlineSession(path string, session onlineSession) error {
	encoded, err := json.Marshal(session)
	if err != nil {
		return err
	}
	defer clear(encoded)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".session-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0600); err != nil {
		return err
	}
	if _, err := temporary.Write(encoded); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		// Windows does not replace an existing destination with Rename. A failed
		// refresh only requires a new login, so use a bounded in-place fallback.
		target, openErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if openErr != nil {
			return openErr
		}
		if chmodErr := target.Chmod(0600); chmodErr != nil {
			_ = target.Close()
			return chmodErr
		}
		_, writeErr := target.Write(encoded)
		if writeErr == nil {
			writeErr = target.Sync()
		}
		if closeErr := target.Close(); writeErr == nil {
			writeErr = closeErr
		}
		if writeErr != nil {
			return writeErr
		}
		_ = os.Remove(temporaryPath)
	}
	committed = true
	return nil
}

func loadOnlineSession(path, server string) (onlineSession, error) {
	file, err := os.Open(path)
	if err != nil {
		return onlineSession{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, adminResponseLimit+1))
	if err != nil || len(data) > adminResponseLimit {
		return onlineSession{}, errors.New("invalid_session_file")
	}
	defer clear(data)
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var session onlineSession
	if decoder.Decode(&session) != nil || decoder.Decode(new(any)) != io.EOF || session.Server != server || session.SessionToken == "" || session.CSRFToken == "" {
		return onlineSession{}, errors.New("invalid_session_file")
	}
	return session, nil
}

func (c *adminClient) login(ctx context.Context, username string, password []byte) (onlineSession, onlineSessionView, error) {
	payload, err := json.Marshal(map[string]string{"username": username, "password": string(password)})
	if err != nil {
		return onlineSession{}, onlineSessionView{}, err
	}
	defer clear(payload)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.server+"/api/v1/admin/session", bytes.NewReader(payload))
	if err != nil {
		return onlineSession{}, onlineSessionView{}, err
	}
	request.Header.Set("Origin", c.server)
	request.Header.Set("Content-Type", "application/json")
	response, data, err := c.execute(request)
	if err != nil {
		return onlineSession{}, onlineSessionView{}, err
	}
	if response.StatusCode != http.StatusOK {
		return onlineSession{}, onlineSessionView{}, fmt.Errorf("admin_login_http_%d", response.StatusCode)
	}
	var view onlineSessionView
	if json.Unmarshal(data, &view) != nil || view.CSRFToken == "" || view.Admin.ID == "" {
		return onlineSession{}, onlineSessionView{}, errors.New("invalid_admin_response")
	}
	var token string
	for _, cookie := range response.Cookies() {
		if cookie.Name == "mc_session" {
			token = cookie.Value
		}
	}
	if token == "" {
		return onlineSession{}, onlineSessionView{}, errors.New("invalid_admin_response")
	}
	return onlineSession{Server: c.server, SessionToken: token, CSRFToken: view.CSRFToken, ExpiresAt: view.ExpiresAt}, view, nil
}

func (c *adminClient) authenticated(ctx context.Context, session onlineSession, method, path string, body []byte, contentType string) (*http.Response, []byte, error) {
	request, err := http.NewRequestWithContext(ctx, method, c.server+path, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	request.Header.Set("Origin", c.server)
	request.Header.Set("X-CSRF-Token", session.CSRFToken)
	request.AddCookie(&http.Cookie{Name: "mc_session", Value: session.SessionToken})
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	return c.execute(request)
}

func (c *adminClient) authenticatedWithRecovery(ctx context.Context, session *onlineSession, sessionFile, method, path string, body []byte, contentType string) (*http.Response, []byte, error) {
	response, data, err := c.authenticated(ctx, *session, method, path, body, contentType)
	if err != nil || response.StatusCode != http.StatusForbidden {
		return response, data, err
	}
	recovery, recoveryData, err := c.authenticated(ctx, *session, http.MethodGet, "/api/v1/admin/session/me", nil, "")
	if err != nil {
		return nil, nil, err
	}
	if recovery.StatusCode != http.StatusOK {
		return response, data, nil
	}
	var view onlineSessionView
	if json.Unmarshal(recoveryData, &view) != nil || view.CSRFToken == "" {
		return nil, nil, errors.New("invalid_admin_response")
	}
	session.CSRFToken, session.ExpiresAt = view.CSRFToken, view.ExpiresAt
	if err := saveOnlineSession(sessionFile, *session); err != nil {
		return nil, nil, err
	}
	return c.authenticated(ctx, *session, method, path, body, contentType)
}

func (c *adminClient) execute(request *http.Request) (*http.Response, []byte, error) {
	response, err := c.http.Do(request)
	if err != nil {
		return nil, nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, adminResponseLimit+1))
	if err != nil {
		return nil, nil, err
	}
	if len(data) > adminResponseLimit {
		clear(data)
		return nil, nil, errors.New("admin_response_too_large")
	}
	return response, data, nil
}
