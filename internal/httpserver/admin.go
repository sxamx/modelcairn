package httpserver

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sxamx/modelcairn/internal/adminsettings"
	"github.com/sxamx/modelcairn/internal/config"
	"github.com/sxamx/modelcairn/internal/storage"
)

const loginBodyLimit = 16 << 10

type adminAPI struct {
	installation  *storage.Installation
	login         *storage.AdminLoginService
	settings      *storage.AdminSettingsService
	configuration *config.Manager
	repository    *storage.Repository
	agentTokens   *storage.AgentTokenService
	boundary      adminBoundary
	data          *dataAPI
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
type adminView struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}
type sessionView struct {
	Admin     adminView `json:"admin"`
	CSRFToken string    `json:"csrfToken"`
	ExpiresAt time.Time `json:"expiresAt"`
}
type settingsDocument struct {
	APIVersion      string                 `json:"apiVersion"`
	Kind            string                 `json:"kind"`
	ResourceVersion int64                  `json:"resourceVersion"`
	Spec            adminsettings.Resolved `json:"spec"`
}
type settingsState struct {
	Desired         settingsDocument `json:"desired"`
	Effective       settingsDocument `json:"effective"`
	RestartRequired bool             `json:"restartRequired"`
}

func (a *adminAPI) routes(mux *http.ServeMux) {
	if a.data != nil {
		mux.HandleFunc("POST /v1/chat/completions", a.data.chatCompletions)
	}
	mux.HandleFunc("POST /api/v1/admin/session", a.createSession)
	mux.HandleFunc("DELETE /api/v1/admin/session", a.deleteSession)
	mux.HandleFunc("GET /api/v1/admin/session/me", a.currentSession)
	mux.HandleFunc("GET /api/v1/admin/settings", a.getSettings)
	mux.HandleFunc("POST /api/v1/admin/settings/plan", a.planSettings)
	mux.HandleFunc("POST /api/v1/admin/settings/apply", a.applySettings)
	mux.HandleFunc("POST /api/v1/admin/config/validate", a.validateConfiguration)
	mux.HandleFunc("POST /api/v1/admin/config/plan", a.planConfiguration)
	mux.HandleFunc("POST /api/v1/admin/config/apply", a.applyConfiguration)
	mux.HandleFunc("GET /api/v1/admin/config/export", a.exportConfiguration)
	mux.HandleFunc("GET /api/v1/admin/resources/{kind}", a.listResources)
	mux.HandleFunc("POST /api/v1/admin/resources/{kind}", a.createResource)
	mux.HandleFunc("GET /api/v1/admin/resources/{kind}/{name}", a.getResource)
	mux.HandleFunc("PUT /api/v1/admin/resources/{kind}/{name}", a.updateResource)
	mux.HandleFunc("DELETE /api/v1/admin/resources/{kind}/{name}", a.deleteResource)
	mux.HandleFunc("GET /api/v1/admin/secrets", a.listSecrets)
	mux.HandleFunc("GET /api/v1/admin/secrets/{name}", a.getSecret)
	mux.HandleFunc("PUT /api/v1/admin/secrets/{name}", a.putSecret)
	mux.HandleFunc("DELETE /api/v1/admin/secrets/{name}", a.deleteSecret)
	mux.HandleFunc("GET /api/v1/admin/agent-tokens/{name}/status", a.getAgentTokenStatus)
	mux.HandleFunc("POST /api/v1/admin/agent-tokens/{name}/issue", a.issueAgentToken)
	mux.HandleFunc("POST /api/v1/admin/agent-tokens/{name}/revoke", a.revokeAgentToken)
}

func (a *adminAPI) createSession(w http.ResponseWriter, r *http.Request) {
	client, err := a.boundary.client(r)
	if err != nil || a.boundary.requireOrigin(r) != nil {
		writeAdminError(w, http.StatusForbidden, "boundary_rejected", false)
		return
	}
	if !mediaType(r, "application/json") {
		writeAdminError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", false)
		return
	}
	input, err := decodeLogin(r)
	if err != nil {
		writeAdminError(w, bodyStatus(err), "malformed", false)
		return
	}
	credentials, err := a.login.Login(r.Context(), client, input.Username, []byte(input.Password))
	if err != nil {
		status, code, retry := http.StatusServiceUnavailable, storage.LoginUnavailable, time.Duration(0)
		switch {
		case storage.IsLoginCode(err, storage.LoginMalformed):
			status, code = http.StatusBadRequest, storage.LoginMalformed
		case storage.IsLoginCode(err, storage.LoginInvalidCredentials):
			status, code = http.StatusUnauthorized, storage.LoginInvalidCredentials
		case storage.IsLoginCode(err, storage.LoginThrottled):
			status, code = http.StatusTooManyRequests, storage.LoginThrottled
			var e *storage.LoginError
			if errors.As(err, &e) {
				retry = e.RetryAfter
			}
		}
		if retry > 0 {
			w.Header().Set("Retry-After", strconv.FormatInt(int64((retry+time.Second-1)/time.Second), 10))
		}
		writeAdminError(w, status, code, status >= 500 || status == http.StatusTooManyRequests)
		return
	}
	secure := a.boundary.transport != adminsettings.LoopbackHTTP
	// Session lifetime is enforced in SQLite on every request. A fixed browser
	// expiry based on initial idle time would log out an actively used session.
	http.SetCookie(w, &http.Cookie{Name: "mc_session", Value: credentials.SessionToken, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode})
	writeJSON(w, http.StatusOK, sessionResponse(credentials.Admin, credentials.CSRFToken, credentials.ExpiresAt))
}

func (a *adminAPI) currentSession(w http.ResponseWriter, r *http.Request) {
	if _, err := a.boundary.client(r); err != nil || a.boundary.requireRecoveryOrigin(r) != nil {
		writeAdminError(w, http.StatusForbidden, "boundary_rejected", false)
		return
	}
	token, err := sessionCookie(r)
	if err != nil {
		writeAdminError(w, http.StatusUnauthorized, "authentication_required", false)
		return
	}
	result, err := storage.RotateAdminSessionCSRF(r.Context(), a.installation, token, time.Now())
	if err != nil {
		writeSessionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sessionResponse(result.Admin, result.CSRFToken, result.ExpiresAt))
}

func (a *adminAPI) deleteSession(w http.ResponseWriter, r *http.Request) {
	if _, err := a.boundary.client(r); err != nil || a.boundary.requireOrigin(r) != nil {
		writeAdminError(w, http.StatusForbidden, "boundary_rejected", false)
		return
	}
	token, err := sessionCookie(r)
	csrf, csrfErr := singleHeader(r, "X-CSRF-Token")
	if err != nil {
		writeAdminError(w, http.StatusUnauthorized, "authentication_required", false)
		return
	}
	if csrfErr != nil {
		writeAdminError(w, http.StatusForbidden, "csrf_rejected", false)
		return
	}
	if err := storage.RevokeAdminSession(r.Context(), a.installation, token, csrf, time.Now()); err != nil {
		writeSessionError(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "mc_session", Value: "", Path: "/", HttpOnly: true, Secure: a.boundary.transport != adminsettings.LoopbackHTTP, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

func (a *adminAPI) authorize(r *http.Request, requireOrigin bool) (string, string, int) {
	if _, err := a.boundary.client(r); err != nil || requireOrigin && a.boundary.requireOrigin(r) != nil {
		return "", "", http.StatusForbidden
	}
	session, err := sessionCookie(r)
	if err != nil {
		return "", "", http.StatusUnauthorized
	}
	csrf, err := singleHeader(r, "X-CSRF-Token")
	if err != nil {
		return "", "", http.StatusForbidden
	}
	return session, csrf, 0
}

func (a *adminAPI) getSettings(w http.ResponseWriter, r *http.Request) {
	session, csrf, status := a.authorize(r, false)
	if status != 0 {
		writeAdminError(w, status, boundaryCode(status), false)
		return
	}
	if _, err := storage.UseAdminSession(r.Context(), a.installation, session, csrf, true, time.Now()); err != nil {
		writeSessionError(w, err)
		return
	}
	state, err := a.settings.State(r.Context())
	if err != nil {
		writeAdminError(w, http.StatusServiceUnavailable, "unavailable", true)
		return
	}
	writeJSON(w, http.StatusOK, stateResponse(state))
}

func (a *adminAPI) planSettings(w http.ResponseWriter, r *http.Request) {
	session, csrf, status := a.authorize(r, true)
	if status != 0 {
		writeAdminError(w, status, boundaryCode(status), false)
		return
	}
	if !settingsMediaType(r) {
		writeAdminError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", false)
		return
	}
	if _, err := storage.UseAdminSession(r.Context(), a.installation, session, csrf, true, time.Now()); err != nil {
		writeSessionError(w, err)
		return
	}
	body, err := readBody(r, adminsettings.MaxInputBytes)
	if err != nil {
		writeAdminError(w, bodyStatus(err), "invalid_settings", false)
		return
	}
	plan, err := a.settings.Plan(r.Context(), body)
	if err != nil {
		writeAdminError(w, settingsErrorStatus(err), settingsErrorCode(err), false)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"desired": document(plan.Desired), "changedFields": plan.ChangedFields, "planToken": plan.Token, "expiresAt": plan.ExpiresAt})
}

func (a *adminAPI) applySettings(w http.ResponseWriter, r *http.Request) {
	session, csrf, status := a.authorize(r, true)
	if status != 0 {
		writeAdminError(w, status, boundaryCode(status), false)
		return
	}
	if !settingsMediaType(r) {
		writeAdminError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", false)
		return
	}
	plan, err := singleHeader(r, "X-ModelCairn-Plan-Token")
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_plan", false)
		return
	}
	body, err := readBody(r, adminsettings.MaxInputBytes)
	if err != nil {
		writeAdminError(w, bodyStatus(err), "invalid_settings", false)
		return
	}
	result, err := a.settings.ApplySession(r.Context(), body, plan, session, csrf)
	if err != nil {
		if errors.Is(err, storage.ErrAdminSessionInvalid) {
			writeSessionError(w, err)
			return
		}
		status := settingsErrorStatus(err)
		if errors.Is(err, storage.ErrAdminSessionInvalid) {
			status = http.StatusUnauthorized
		}
		writeAdminError(w, status, settingsErrorCode(err), false)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"applied": true, "appliedAt": result.AppliedAt, "settings": stateResponse(result.State)})
}

func sessionResponse(admin storage.AdminIdentity, csrf string, expires time.Time) sessionView {
	return sessionView{adminView{admin.ID, admin.Username}, csrf, expires}
}
func writeSessionError(w http.ResponseWriter, err error) {
	if errors.Is(err, storage.ErrAdminCSRFInvalid) {
		writeAdminError(w, http.StatusForbidden, "csrf_rejected", false)
	} else if errors.Is(err, storage.ErrAdminSessionInvalid) {
		writeAdminError(w, http.StatusUnauthorized, "authentication_required", false)
	} else {
		writeAdminError(w, http.StatusServiceUnavailable, "unavailable", true)
	}
}
func document(r storage.AdminSettingsRecord) settingsDocument {
	return settingsDocument{adminsettings.APIVersion, adminsettings.DocumentKind, r.ResourceVersion, r.Spec}
}
func stateResponse(s storage.AdminSettingsState) settingsState {
	return settingsState{document(s.Desired), document(s.Effective), s.RestartRequired}
}
func mediaType(r *http.Request, want string) bool {
	return strings.EqualFold(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]), want)
}
func settingsMediaType(r *http.Request) bool {
	value := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]))
	return value == "application/json" || value == "application/yaml" || value == "text/yaml"
}
func singleHeader(r *http.Request, name string) (string, error) {
	values := r.Header.Values(name)
	if len(values) != 1 || values[0] == "" || strings.Contains(values[0], ",") {
		return "", errors.New("invalid_header")
	}
	return values[0], nil
}

var errBodyTooLarge = errors.New("body_too_large")

func readBody(r *http.Request, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errBodyTooLarge
	}
	return data, nil
}
func decodeLogin(r *http.Request) (loginRequest, error) {
	var out loginRequest
	data, err := readBody(r, loginBodyLimit)
	if err != nil {
		return out, err
	}
	if !utf8.Valid(data) {
		return out, errors.New("invalid_login_body")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return out, errors.New("invalid_login_body")
	}
	seen := map[string]bool{}
	for decoder.More() {
		key, err := decoder.Token()
		name, ok := key.(string)
		if err != nil || !ok || seen[name] {
			return out, errors.New("invalid_login_body")
		}
		seen[name] = true
		switch name {
		case "username":
			err = decoder.Decode(&out.Username)
		case "password":
			err = decoder.Decode(&out.Password)
		default:
			return out, errors.New("invalid_login_body")
		}
		if err != nil {
			return out, errors.New("invalid_login_body")
		}
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') || !seen["username"] || !seen["password"] {
		return out, errors.New("invalid_login_body")
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return out, errors.New("invalid_login_body")
	}
	return out, nil
}

func decodeJSON(r *http.Request, limit int64, out any) error {
	data, err := readBody(r, limit)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return errors.New("trailing_json")
	}
	return nil
}
func bodyStatus(err error) int {
	if errors.Is(err, errBodyTooLarge) {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusBadRequest
}
func boundaryCode(status int) string {
	if status == http.StatusUnauthorized {
		return "authentication_required"
	}
	return "boundary_rejected"
}
func settingsErrorStatus(err error) int {
	if storage.IsRepositoryCode(err, storage.CodeVersionConflict) || errors.Is(err, storage.ErrPlanAlreadyUsed) {
		return http.StatusConflict
	}
	var e *adminsettings.Error
	if errors.As(err, &e) && len(e.Diagnostics) > 0 && e.Diagnostics[0].Code == adminsettings.CodeInputTooLarge {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusBadRequest
}
func settingsErrorCode(err error) string {
	if errors.Is(err, storage.ErrAdminSessionInvalid) {
		return "authentication_required"
	}
	if errors.Is(err, storage.ErrPlanAlreadyUsed) {
		return "plan_already_used"
	}
	if storage.IsRepositoryCode(err, storage.CodeVersionConflict) {
		return "version_conflict"
	}
	return "invalid_settings"
}
func writeAdminError(w http.ResponseWriter, status int, code string, retryable bool) {
	id := make([]byte, 12)
	if _, err := rand.Read(id); err != nil {
		id = []byte("request-id-unavailable")
	}
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": code, "requestId": hex.EncodeToString(id), "retryable": retryable}})
}
