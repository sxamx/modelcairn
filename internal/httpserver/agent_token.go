package httpserver

import (
	"errors"
	"net/http"

	"github.com/sxamx/modelcairn/internal/storage"
)

func (a *adminAPI) getAgentTokenStatus(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeRead(w, r) {
		return
	}
	status, err := a.agentTokens.Status(r.Context(), r.PathValue("name"))
	if err != nil {
		writeAgentTokenError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tokenStatus": status})
}

func (a *adminAPI) issueAgentToken(w http.ResponseWriter, r *http.Request) {
	session, csrf, status := a.authorize(r, true)
	if status != 0 {
		writeAdminError(w, status, boundaryCode(status), false)
		return
	}
	issued, err := a.agentTokens.IssueSession(r.Context(), r.PathValue("name"), session, csrf)
	if err != nil {
		writeAgentTokenError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, issued)
}

func (a *adminAPI) revokeAgentToken(w http.ResponseWriter, r *http.Request) {
	session, csrf, status := a.authorize(r, true)
	if status != 0 {
		writeAdminError(w, status, boundaryCode(status), false)
		return
	}
	if err := a.agentTokens.RevokeSession(r.Context(), r.PathValue("name"), session, csrf); err != nil {
		writeAgentTokenError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

func writeAgentTokenError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, storage.ErrAdminSessionInvalid):
		writeSessionError(w, err)
	case storage.IsRepositoryCode(err, storage.CodeNotFound):
		writeAdminError(w, http.StatusNotFound, "not_found", false)
	case storage.IsRepositoryCode(err, storage.CodeAlreadyExists):
		writeAdminError(w, http.StatusConflict, "token_not_issuable", false)
	case storage.IsRepositoryCode(err, storage.CodeInvalidResource):
		writeAdminError(w, http.StatusBadRequest, "invalid_agent_token", false)
	default:
		writeAdminError(w, http.StatusServiceUnavailable, "unavailable", true)
	}
}
