// Package redact removes exact registered secret values from output boundaries.
package redact

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
)

const Replacement = "[REDACTED]"

var ErrInvalidValue = errors.New("invalid_redaction_value")

type entry struct {
	value []byte
	refs  int
}

type Redactor struct {
	mu      sync.RWMutex
	entries map[[sha256.Size]byte]*entry
	ordered []*entry
}

func New() *Redactor { return &Redactor{entries: make(map[[sha256.Size]byte]*entry)} }

// Register retains a private copy and returns an idempotent removal function.
func (r *Redactor) Register(value []byte) (func(), error) {
	if r == nil || len(value) < 8 || len(value) > 16384 {
		return nil, ErrInvalidValue
	}
	digest := sha256.Sum256(value)
	r.mu.Lock()
	item := r.entries[digest]
	if item != nil && !bytes.Equal(item.value, value) {
		r.mu.Unlock()
		return nil, errors.New("redaction_digest_collision")
	}
	if item == nil {
		item = &entry{value: bytes.Clone(value)}
		r.entries[digest] = item
		r.ordered = append(r.ordered, item)
		sort.SliceStable(r.ordered, func(a, b int) bool { return len(r.ordered[a].value) > len(r.ordered[b].value) })
	}
	item.refs++
	r.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			r.mu.Lock()
			defer r.mu.Unlock()
			item := r.entries[digest]
			if item == nil {
				return
			}
			item.refs--
			if item.refs > 0 {
				return
			}
			delete(r.entries, digest)
			for index, candidate := range r.ordered {
				if candidate == item {
					r.ordered = append(r.ordered[:index], r.ordered[index+1:]...)
					break
				}
			}
			clear(item.value)
		})
	}, nil
}

func (r *Redactor) Bytes(value []byte) []byte {
	result := bytes.Clone(value)
	if r == nil {
		return result
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, item := range r.ordered {
		result = bytes.ReplaceAll(result, item.value, []byte(Replacement))
	}
	return result
}

func (r *Redactor) String(value string) string { return string(r.Bytes([]byte(value))) }

type handler struct {
	next     slog.Handler
	redactor *Redactor
	bindings []binding
}

type binding struct {
	attributes []slog.Attr
	group      string
}

func NewHandler(next slog.Handler, redactor *Redactor) slog.Handler {
	if next == nil {
		panic("nil slog handler")
	}
	return &handler{next: next, redactor: redactor}
}

func (h *handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *handler) Handle(ctx context.Context, record slog.Record) error {
	rewritten := slog.NewRecord(record.Time, record.Level, h.redactor.String(record.Message), record.PC)
	record.Attrs(func(attribute slog.Attr) bool {
		rewritten.AddAttrs(h.redactAttr(attribute))
		return true
	})
	next := h.next
	for _, item := range h.bindings {
		if item.group != "" {
			next = next.WithGroup(h.redactor.String(item.group))
			continue
		}
		attributes := make([]slog.Attr, len(item.attributes))
		for index, attribute := range item.attributes {
			attributes[index] = h.redactAttr(attribute)
		}
		next = next.WithAttrs(attributes)
	}
	return next.Handle(ctx, rewritten)
}

func (h *handler) WithAttrs(attributes []slog.Attr) slog.Handler {
	bindings := append([]binding(nil), h.bindings...)
	bindings = append(bindings, binding{attributes: append([]slog.Attr(nil), attributes...)})
	return &handler{next: h.next, redactor: h.redactor, bindings: bindings}
}

func (h *handler) WithGroup(name string) slog.Handler {
	bindings := append([]binding(nil), h.bindings...)
	bindings = append(bindings, binding{group: name})
	return &handler{next: h.next, redactor: h.redactor, bindings: bindings}
}

func (h *handler) redactAttr(attribute slog.Attr) slog.Attr {
	attribute.Key = h.redactor.String(attribute.Key)
	attribute.Value = attribute.Value.Resolve()
	switch attribute.Value.Kind() {
	case slog.KindString:
		attribute.Value = slog.StringValue(h.redactor.String(attribute.Value.String()))
	case slog.KindGroup:
		group := attribute.Value.Group()
		for index := range group {
			group[index] = h.redactAttr(group[index])
		}
		attribute.Value = slog.GroupValue(group...)
	case slog.KindAny:
		switch value := attribute.Value.Any().(type) {
		case []byte:
			attribute.Value = slog.StringValue(h.redactor.String(string(value)))
		case error:
			attribute.Value = slog.StringValue(h.redactor.String(value.Error()))
		case fmt.Stringer:
			attribute.Value = slog.StringValue(h.redactor.String(value.String()))
		default:
			// Arbitrary values may contain nested strings that the downstream JSON
			// handler would serialize without passing through this boundary.
			attribute.Value = slog.StringValue(h.redactor.String(fmt.Sprint(value)))
		}
	}
	return attribute
}
