// Package testupstream provides a deterministic HTTP upstream for router tests.
package testupstream

import (
	"io"
	"net/http"
	"sync"
	"time"
)

const maxObservedBodyBytes = 2 << 20

// Step is one scripted outcome. Fixtures must not contain production secrets.
type Step struct {
	Status     int
	Header     http.Header
	Body       string
	Delay      time.Duration
	Disconnect bool
	Started    chan<- struct{}
}

// Observation deliberately retains only metadata, never headers or body data.
type Observation struct {
	Sequence             int
	Method               string
	Path                 string
	ContentType          string
	AuthorizationPresent bool
	BodyBytes            int64
	BodyLimitExceeded    bool
}

// Scripted serves each Step once, in order of request arrival.
type Scripted struct {
	mu           sync.Mutex
	steps        []Step
	observations []Observation
}

func New(steps ...Step) *Scripted { return &Scripted{steps: append([]Step(nil), steps...)} }

func (s *Scripted) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	bodyBytes, _ := io.Copy(io.Discard, io.LimitReader(r.Body, maxObservedBodyBytes+1))
	s.mu.Lock()
	sequence := len(s.observations) + 1
	s.observations = append(s.observations, Observation{
		Sequence: sequence, Method: r.Method, Path: r.URL.Path,
		ContentType:          r.Header.Get("Content-Type"),
		AuthorizationPresent: r.Header.Get("Authorization") != "",
		BodyBytes:            bodyBytes, BodyLimitExceeded: bodyBytes > maxObservedBodyBytes,
	})
	if len(s.steps) == 0 {
		s.mu.Unlock()
		http.Error(w, `{"error":{"message":"script exhausted"}}`, http.StatusInternalServerError)
		return
	}
	step := s.steps[0]
	s.steps = s.steps[1:]
	s.mu.Unlock()
	if step.Started != nil {
		select {
		case step.Started <- struct{}{}:
		default:
		}
	}

	if step.Delay > 0 {
		timer := time.NewTimer(step.Delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-r.Context().Done():
			return
		}
	}
	if step.Disconnect {
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "disconnect unavailable", http.StatusInternalServerError)
			return
		}
		connection, _, err := hijacker.Hijack()
		if err == nil {
			_ = connection.Close()
		}
		return
	}
	for name, values := range step.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	status := step.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = io.WriteString(w, step.Body)
}

func (s *Scripted) Observations() []Observation {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Observation(nil), s.observations...)
}

func (s *Scripted) Remaining() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.steps)
}
