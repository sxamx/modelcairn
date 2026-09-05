package httpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const (
	readHeaderTimeout = 5 * time.Second
	idleTimeout       = 60 * time.Second
)

type componentState struct {
	Ready  bool   `json:"ready"`
	Reason string `json:"reason,omitempty"`
}

type Component string

const (
	ComponentConfiguration Component = "configuration"
	ComponentPersistence   Component = "persistence"
	ComponentSecretStore   Component = "secretStore"
)

// ReadinessProbe tracks only local dependencies. Provider health does not make
// the entire administrative plane unready.
type ReadinessProbe struct {
	mu         sync.RWMutex
	components map[string]componentState
}

func NewReadinessProbe() *ReadinessProbe {
	return &ReadinessProbe{components: map[string]componentState{
		string(ComponentConfiguration): {Reason: "not initialized"},
		string(ComponentPersistence):   {Reason: "not initialized"},
		string(ComponentSecretStore):   {Reason: "not initialized"},
	}}
}

func (p *ReadinessProbe) Set(name Component, ready bool, reason string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	key := string(name)
	if _, exists := p.components[key]; !exists {
		return false
	}
	p.components[key] = componentState{Ready: ready, Reason: reason}
	return true
}

func (p *ReadinessProbe) snapshot() (bool, map[string]componentState) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	ready := true
	components := make(map[string]componentState, len(p.components))
	for name, state := range p.components {
		components[name] = state
		ready = ready && state.Ready
	}
	return ready, components
}

func New(address string, readiness *ReadinessProbe, logger *slog.Logger) *http.Server {
	if readiness == nil {
		readiness = NewReadinessProbe()
	}
	if logger == nil {
		logger = slog.Default()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		ready, components := readiness.snapshot()
		status := http.StatusOK
		state := "ready"
		if !ready {
			status = http.StatusServiceUnavailable
			state = "not_ready"
		}
		writeJSON(w, status, map[string]any{"status": state, "components": components})
	})

	return &http.Server{
		Addr:              address,
		Handler:           requestLog(logger, mux),
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func requestLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		logger.Info("http request", "method", r.Method, "route", route, "durationMs", time.Since(started).Milliseconds())
	})
}
