package httpserver

import (
	"net/http"
	"time"

	"github.com/sxamx/modelcairn/internal/storage"
)

func (a *adminAPI) getOverview(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeRead(w, r) {
		return
	}
	overview, err := storage.ReadOperationalOverview(r.Context(), a.installation.DB(), time.Now())
	if err != nil {
		writeAdminError(w, http.StatusServiceUnavailable, "unavailable", true)
		return
	}
	recent := make([]map[string]any, 0, len(overview.RecentRequests))
	for _, item := range overview.RecentRequests {
		recent = append(recent, map[string]any{"id": item.ID, "requestedAlias": item.RequestedAlias, "startedAt": item.StartedAt, "completedAt": item.CompletedAt, "outcome": item.Outcome, "httpStatus": item.HTTPStatus, "durationMs": item.DurationMillis, "attempts": item.Attempts})
	}
	writeJSON(w, http.StatusOK, map[string]any{"resourceCounts": overview.ResourceCounts, "requests24h": map[string]int64{"total": overview.RequestsTotal, "success": overview.RequestsSuccess, "error": overview.RequestsError}, "activeCooldowns": overview.ActiveCooldowns, "recentRequests": recent, "generatedAt": overview.GeneratedAt})
}
