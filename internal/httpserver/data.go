package httpserver

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/sxamx/modelcairn/internal/chatcompletions"
	"github.com/sxamx/modelcairn/internal/router"
	"github.com/sxamx/modelcairn/internal/storage"
)

type dataAPI struct {
	tokens *storage.AgentTokenService
	loader *router.Loader
	engine *router.Engine
}

func (a *dataAPI) chatCompletions(w http.ResponseWriter, r *http.Request) {
	requestID := newRequestID()
	if !mediaType(r, "application/json") {
		writeDataError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "", requestID)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, chatcompletions.MaxRequestBytes+1))
	if err != nil {
		writeDataError(w, http.StatusBadRequest, "invalid_request", "", requestID)
		return
	}
	defer clear(body)
	if len(body) > chatcompletions.MaxRequestBytes {
		writeDataError(w, http.StatusRequestEntityTooLarge, "request_too_large", "", requestID)
		return
	}
	parsed, err := chatcompletions.Parse(body)
	if err != nil {
		var contract *chatcompletions.Error
		if errors.As(err, &contract) {
			writeDataError(w, http.StatusBadRequest, contract.Code, contract.Param, requestID)
		} else {
			writeDataError(w, http.StatusBadRequest, "invalid_request", "", requestID)
		}
		return
	}
	authorization := r.Header.Values("Authorization")
	if len(authorization) != 1 {
		writeDataError(w, http.StatusUnauthorized, "invalid_agent_token", "", requestID)
		return
	}
	bearer, ok := bearerToken(authorization[0])
	if !ok {
		writeDataError(w, http.StatusUnauthorized, "invalid_agent_token", "", requestID)
		return
	}
	_, routeID, err := a.tokens.AuthenticateAlias(r.Context(), bearer, parsed.Request.Model)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrAgentTokenInvalid):
			writeDataError(w, http.StatusUnauthorized, "invalid_agent_token", "", requestID)
		case errors.Is(err, storage.ErrAgentRouteForbidden):
			writeDataError(w, http.StatusForbidden, "route_forbidden", "model", requestID)
		case storage.IsRepositoryCode(err, storage.CodeNotFound):
			writeDataError(w, http.StatusNotFound, "route_not_found", "model", requestID)
		default:
			writeDataError(w, http.StatusServiceUnavailable, "authentication_unavailable", "", requestID)
		}
		return
	}
	snapshot, err := a.loader.Load(r.Context(), parsed.Request.Model, parsed.Capabilities)
	if err != nil || snapshot.RouteID != routeID {
		var loadErr *router.LoadError
		switch {
		case errors.As(err, &loadErr) && loadErr.Code == router.CodeRouteDisabled:
			writeDataError(w, http.StatusUnprocessableEntity, "route_disabled", "model", requestID)
		case errors.As(err, &loadErr) && loadErr.Code == router.CodeRouteNotFound:
			writeDataError(w, http.StatusNotFound, "route_not_found", "model", requestID)
		default:
			writeDataError(w, http.StatusServiceUnavailable, "router_unavailable", "", requestID)
		}
		return
	}
	result, err := a.engine.Run(r.Context(), snapshot, parsed.Request)
	if err != nil {
		writeRunError(w, err, result, requestID)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-ModelCairn-Request-ID", requestID)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result.Upstream.Body)
}

func bearerToken(value string) (string, bool) {
	if len(value) < 8 || !strings.EqualFold(value[:7], "Bearer ") || strings.ContainsAny(value[7:], " \t\r\n,") {
		return "", false
	}
	return value[7:], true
}

func writeRunError(w http.ResponseWriter, err error, result router.RunResult, requestID string) {
	var runErr *router.RunError
	if !errors.As(err, &runErr) {
		writeDataError(w, http.StatusServiceUnavailable, "router_unavailable", "", requestID)
		return
	}
	status := http.StatusBadGateway
	switch runErr.Code {
	case "no_eligible_destination":
		status = http.StatusUnprocessableEntity
	case "fallback_exhausted":
		if result.Upstream.StatusCode == http.StatusTooManyRequests {
			status = http.StatusTooManyRequests
		} else {
			status = http.StatusServiceUnavailable
		}
	case "total_timeout":
		status = http.StatusGatewayTimeout
	case "request_cancelled":
		return
	case "streaming_not_supported":
		status = http.StatusUnprocessableEntity
	}
	writeDataError(w, status, runErr.Code, "", requestID)
}

func writeDataError(w http.ResponseWriter, status int, code, param, requestID string) {
	type detail struct {
		Message   string  `json:"message"`
		Type      string  `json:"type"`
		Code      string  `json:"code"`
		Param     *string `json:"param"`
		RequestID string  `json:"request_id"`
	}
	var parameter *string
	if param != "" {
		parameter = &param
	}
	writeJSON(w, status, map[string]any{"error": detail{Message: "Request could not be completed.", Type: "modelcairn_error", Code: code, Param: parameter, RequestID: requestID}})
}

func newRequestID() string {
	var value [12]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "req_unavailable"
	}
	return "req_" + hex.EncodeToString(value[:])
}
