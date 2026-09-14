package httpserver

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/sxamx/modelcairn/internal/storage"
)

type activityCursor struct {
	Version   int    `json:"v"`
	StartedAt string `json:"startedAt"`
	ID        string `json:"id"`
	Filter    string `json:"filter"`
}

func (a *adminAPI) listOperationalRequests(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeRead(w, r) {
		return
	}
	query, err := decodeOperationalQuery(r)
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_query", false)
		return
	}
	page, err := storage.ListOperationalRequests(r.Context(), a.installation.DB(), query)
	if err != nil {
		writeAdminError(w, http.StatusServiceUnavailable, "unavailable", true)
		return
	}
	var next any
	if page.NextID != "" {
		next = encodeActivityCursor(page.NextStartedAt, page.NextID, activityQueryScope(query))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": page.Items, "nextCursor": next})
}

func (a *adminAPI) listOperationalAttempts(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeRead(w, r) {
		return
	}
	id := r.PathValue("id")
	if id == "" || len(id) > 128 {
		writeAdminError(w, http.StatusBadRequest, "invalid_request_id", false)
		return
	}
	items, err := storage.ListOperationalAttempts(r.Context(), a.installation.DB(), id)
	if err != nil {
		writeAdminError(w, http.StatusServiceUnavailable, "unavailable", true)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func decodeOperationalQuery(r *http.Request) (storage.OperationalRequestQuery, error) {
	values := r.URL.Query()
	for key, list := range values {
		if key != "outcome" && key != "alias" && key != "from" && key != "to" && key != "cursor" && key != "limit" || len(list) != 1 {
			return storage.OperationalRequestQuery{}, errors.New("invalid query")
		}
	}
	query := storage.OperationalRequestQuery{Limit: 50, Outcome: values.Get("outcome"), Alias: values.Get("alias")}
	if len(query.Alias) > 128 {
		return query, errors.New("alias too long")
	}
	if query.Outcome != "" && query.Outcome != "success" && query.Outcome != "error" && query.Outcome != "partial" && query.Outcome != "cancelled" && query.Outcome != "indeterminate" {
		return query, errors.New("invalid outcome")
	}
	if raw := values.Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 200 {
			return query, errors.New("invalid limit")
		}
		query.Limit = value
	}
	for raw, target := range map[string]**time.Time{"from": &query.From, "to": &query.To} {
		if value := values.Get(raw); value != "" {
			parsed, err := time.Parse(time.RFC3339, value)
			if err != nil {
				return query, err
			}
			parsed = parsed.UTC()
			*target = &parsed
		}
	}
	if query.From != nil && query.To != nil && !query.From.Before(*query.To) {
		return query, errors.New("invalid range")
	}
	if raw := values.Get("cursor"); raw != "" {
		cursor, err := decodeActivityCursor(raw)
		if err != nil {
			return query, err
		}
		parsed, err := time.Parse(time.RFC3339Nano, cursor.StartedAt)
		if err != nil {
			return query, err
		}
		query.BeforeStarted = &parsed
		query.BeforeID = cursor.ID
		if cursor.Filter != activityQueryScope(query) {
			return query, errors.New("cursor filter mismatch")
		}
	}
	return query, nil
}

func encodeActivityCursor(startedAt, id, filter string) string {
	data, _ := json.Marshal(activityCursor{1, startedAt, id, filter})
	return base64.RawURLEncoding.EncodeToString(data)
}

func activityQueryScope(query storage.OperationalRequestQuery) string {
	stamp := func(value *time.Time) string {
		if value == nil {
			return ""
		}
		return value.UTC().Format(time.RFC3339Nano)
	}
	return query.Outcome + "\x00" + query.Alias + "\x00" + stamp(query.From) + "\x00" + stamp(query.To)
}
func decodeActivityCursor(raw string) (activityCursor, error) {
	var value activityCursor
	if len(raw) > 512 {
		return value, errors.New("cursor too long")
	}
	data, err := base64.RawURLEncoding.Strict().DecodeString(raw)
	if err != nil || len(data) > 256 {
		return value, errors.New("invalid cursor")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&value) != nil || decoder.Decode(new(any)) != io.EOF || value.Version != 1 || value.StartedAt == "" || value.ID == "" || len(value.ID) > 128 {
		return value, errors.New("invalid cursor")
	}
	return value, nil
}
