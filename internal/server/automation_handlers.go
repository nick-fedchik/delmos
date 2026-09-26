package server

import (
	"net/http"

	"delmos/internal/automation"
)

// handleAutomationStatus — адміністративне спостереження за чергою рушія
// автоматизації (RUNBOOK.md, ADM-007): кількості доставок за статусом і
// перелік dead-letter записів, що потребують розслідування. Гейтиться правом
// system.observe (system.administrator, 0013_automation_engine.sql).
func handleAutomationStatus(engine *automation.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authContextFrom(r.Context())
		if !ok || !actor.HasPermission("system.observe") {
			writeJSONError(w, http.StatusForbidden, "forbidden", "недостатньо прав для перегляду стану рушія автоматизації")
			return
		}

		if engine == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "automation_unavailable", "рушій автоматизації не запущено")
			return
		}
		snapshot, err := engine.Snapshot(r.Context())
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося прочитати стан рушія автоматизації")
			return
		}
		writeJSON(w, http.StatusOK, snapshot)
	}
}
