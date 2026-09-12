package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/sxamx/modelcairn/internal/chatcompletions"
	"github.com/sxamx/modelcairn/internal/router"
	"github.com/sxamx/modelcairn/internal/storage"
)

type dataAPI struct {
	tokens   *storage.AgentTokenService
	loader   *router.Loader
	engine   *router.Engine
	recorder *storage.OperationalRecorder
	logger   *slog.Logger
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
	identity, routeID, err := a.tokens.AuthenticateAlias(r.Context(), bearer, parsed.Request.Model)
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
	startedAt := time.Now().UTC()
	if err := a.recorder.Begin(r.Context(), storage.RequestStart{ID: requestID, AgentTokenID: identity.ResourceID, RouteID: routeID, RequestedAlias: snapshot.Alias, StartedAt: startedAt}); err != nil {
		writeDataError(w, http.StatusServiceUnavailable, "persistence_unavailable", "", requestID)
		return
	}
	if parsed.Request.Stream {
		a.streamChatCompletions(w, r, requestID, snapshot, parsed.Request)
		return
	}
	result, err := a.engine.Run(r.Context(), snapshot, parsed.Request)
	status := http.StatusOK
	if err != nil {
		status = runErrorStatus(err, result)
	}
	a.completeOperational(requestID, status, result.Attempts, outcomeForRunError(err))
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

func (a *dataAPI) streamChatCompletions(w http.ResponseWriter, r *http.Request, requestID string, snapshot router.Snapshot, request chatcompletions.Request) {
	w.Header().Set("X-ModelCairn-Request-ID", requestID)
	result, err := a.engine.RunStream(r.Context(), w, snapshot, request)
	status := streamRunErrorStatus(err, result)
	outcome := outcomeForRunError(err)
	if result.Committed && err != nil && outcome == "error" {
		outcome = "partial"
	}
	a.completeOperational(requestID, status, result.Attempts, outcome)
	if err != nil && !result.Committed {
		writeStreamRunError(w, err, result, requestID)
	}
}

func outcomeForRunError(runErr error) string {
	outcome := "success"
	var typed *router.RunError
	if errors.As(runErr, &typed) {
		outcome = "error"
		if typed.Code == "request_cancelled" {
			outcome = "cancelled"
		}
		if typed.Code == "indeterminate_upstream" {
			outcome = "indeterminate"
		}
	}
	return outcome
}

func (a *dataAPI) completeOperational(requestID string, status int, routerAttempts []router.Attempt, outcome string) {
	attempts := make([]storage.AttemptCompletion, 0, len(routerAttempts))
	for _, attempt := range routerAttempts {
		attempts = append(attempts, storage.AttemptCompletion{Sequence: attempt.Sequence, DestinationID: attempt.DestinationID,
			Outcome: attempt.Outcome, ErrorClass: attempt.ErrorClass, ProviderStatus: attempt.StatusCode,
			ProviderRequestID: attempt.ProviderRequestID, Retryable: attempt.Retryable, FallbackReason: attempt.FallbackReason,
			RetryAfter: attempt.RetryAfter,
			StartedAt:  attempt.StartedAt, CompletedAt: attempt.CompletedAt})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := a.recorder.Complete(ctx, requestID, storage.RequestCompletion{Outcome: outcome, HTTPStatus: status, CompletedAt: time.Now().UTC(), Attempts: attempts}); err != nil {
		a.logger.Error("operational request completion failed", "requestId", requestID, "code", "persistence_unavailable")
	}
}

func writeStreamRunError(w http.ResponseWriter, err error, result router.StreamRunResult, requestID string) {
	var runErr *router.RunError
	if !errors.As(err, &runErr) {
		writeDataError(w, http.StatusServiceUnavailable, "router_unavailable", "", requestID)
		return
	}
	status := streamRunErrorStatus(err, result)
	if status != 499 {
		writeDataError(w, status, runErr.Code, "", requestID)
	}
}

func streamRunErrorStatus(err error, result router.StreamRunResult) int {
	if err == nil || result.Committed {
		return http.StatusOK
	}
	var upstream router.UpstreamResult
	if len(result.Attempts) > 0 {
		upstream.StatusCode = result.Attempts[len(result.Attempts)-1].StatusCode
	}
	return runErrorStatus(err, router.RunResult{Upstream: upstream, Attempts: result.Attempts})
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
	status := runErrorStatus(err, result)
	if status == 499 {
		return
	}
	writeDataError(w, status, runErr.Code, "", requestID)
}

func runErrorStatus(err error, result router.RunResult) int {
	var runErr *router.RunError
	if !errors.As(err, &runErr) {
		return http.StatusServiceUnavailable
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
		return 499
	case "streaming_not_supported":
		status = http.StatusUnprocessableEntity
	}
	return status
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
