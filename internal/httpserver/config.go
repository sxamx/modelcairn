package httpserver

import (
	"errors"
	"net/http"
	"time"

	"github.com/sxamx/modelcairn/internal/config"
	"github.com/sxamx/modelcairn/internal/storage"
)

func (a *adminAPI) validateConfiguration(w http.ResponseWriter, r *http.Request) {
	session, csrf, status := a.authorize(r, true)
	if status != 0 {
		writeAdminError(w, status, boundaryCode(status), false)
		return
	}
	if _, err := storage.UseAdminSession(r.Context(), a.installation, session, csrf, true, time.Now()); err != nil {
		writeSessionError(w, err)
		return
	}
	if !settingsMediaType(r) {
		writeAdminError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", false)
		return
	}
	body, err := readBody(r, config.MaxInputBytes)
	if err != nil {
		writeAdminError(w, bodyStatus(err), "invalid_configuration", false)
		return
	}
	if _, err := config.Parse(body); err != nil {
		writeConfigError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"valid": true, "issues": []any{}})
}

func (a *adminAPI) planConfiguration(w http.ResponseWriter, r *http.Request) {
	session, csrf, status := a.authorize(r, true)
	if status != 0 {
		writeAdminError(w, status, boundaryCode(status), false)
		return
	}
	if !settingsMediaType(r) {
		writeAdminError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", false)
		return
	}
	allowDelete, err := allowDeleteQuery(r)
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_query", false)
		return
	}
	if _, err := storage.UseAdminSession(r.Context(), a.installation, session, csrf, true, time.Now()); err != nil {
		writeSessionError(w, err)
		return
	}
	body, err := readBody(r, config.MaxInputBytes)
	if err != nil {
		writeAdminError(w, bodyStatus(err), "invalid_configuration", false)
		return
	}
	doc, err := config.Parse(body)
	if err != nil {
		writeConfigError(w, err)
		return
	}
	plan, err := a.configuration.Plan(r.Context(), doc, allowDelete)
	if err != nil {
		writeConfigError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"valid": true, "changes": changeViews(plan.Changes), "planToken": plan.Token, "expiresAt": plan.ExpiresAt})
}

func (a *adminAPI) applyConfiguration(w http.ResponseWriter, r *http.Request) {
	session, csrf, status := a.authorize(r, true)
	if status != 0 {
		writeAdminError(w, status, boundaryCode(status), false)
		return
	}
	if !settingsMediaType(r) {
		writeAdminError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", false)
		return
	}
	allowDelete, err := allowDeleteQuery(r)
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_query", false)
		return
	}
	plan, err := singleHeader(r, "X-ModelCairn-Plan-Token")
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_plan", false)
		return
	}
	body, err := readBody(r, config.MaxInputBytes)
	if err != nil {
		writeAdminError(w, bodyStatus(err), "invalid_configuration", false)
		return
	}
	doc, err := config.Parse(body)
	if err != nil {
		writeConfigError(w, err)
		return
	}
	result, err := a.configuration.ApplySession(r.Context(), plan, doc, allowDelete, session, csrf)
	if err != nil {
		if errors.Is(err, storage.ErrAdminSessionInvalid) {
			writeSessionError(w, err)
			return
		}
		writeConfigError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"applied": true, "changes": changeViews(result.Changes), "appliedAt": result.AppliedAt})
}

func (a *adminAPI) exportConfiguration(w http.ResponseWriter, r *http.Request) {
	session, csrf, status := a.authorize(r, false)
	if status != 0 {
		writeAdminError(w, status, boundaryCode(status), false)
		return
	}
	if _, err := storage.UseAdminSession(r.Context(), a.installation, session, csrf, true, time.Now()); err != nil {
		writeSessionError(w, err)
		return
	}
	data, err := a.configuration.Export(r.Context())
	if err != nil {
		writeAdminError(w, http.StatusServiceUnavailable, "unavailable", true)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func allowDeleteQuery(r *http.Request) (bool, error) {
	values, present := r.URL.Query()["allowDelete"]
	if !present {
		return false, nil
	}
	if len(values) != 1 {
		return false, errors.New("invalid_query")
	}
	switch values[0] {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, errors.New("invalid_query")
	}
}
func changeViews(changes []config.Change) []map[string]string {
	result := make([]map[string]string, 0, len(changes))
	for _, change := range changes {
		result = append(result, map[string]string{"operation": change.Action, "kind": string(change.Kind), "name": change.Name})
	}
	return result
}
func writeConfigError(w http.ResponseWriter, err error) {
	status, code := http.StatusServiceUnavailable, "unavailable"
	if errors.Is(err, storage.ErrInvalidPlan) || errors.Is(err, storage.ErrPlanExpired) {
		status, code = http.StatusBadRequest, "invalid_plan"
	}
	if errors.Is(err, storage.ErrPlanAlreadyUsed) {
		status, code = http.StatusConflict, "plan_already_used"
	}
	if storage.IsRepositoryCode(err, storage.CodeVersionConflict) {
		status, code = http.StatusConflict, "version_conflict"
	}
	var parsed *config.Error
	if errors.As(err, &parsed) && len(parsed.Diagnostics) != 0 {
		status, code = http.StatusBadRequest, parsed.Diagnostics[0].Code
		if code == config.CodeInputTooLarge {
			status = http.StatusRequestEntityTooLarge
		}
	}
	writeAdminError(w, status, code, status >= 500)
}
