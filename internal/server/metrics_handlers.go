package server

import (
	"net/http"
	"time"

	"delmos/internal/auth"
	"delmos/internal/metrics"
)

// handleMetricObservations віддає останні вимірювання проєкту.
//
// Ендпойнт лише читає: обчислення виконує фоновий збирач (SWR-22.1), тож
// запит браузера не може ані запустити перерахунок, ані підмінити показник.
func handleMetricObservations(store *metrics.Store, authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, projectID, ok := projectScopePermission(w, r, authSvc, "economics.read")
		if !ok {
			return
		}
		observations, err := store.ListLatest(r.Context(), projectID, time.Now().UTC())
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "metrics_unavailable", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"observations": observations})
	}
}
