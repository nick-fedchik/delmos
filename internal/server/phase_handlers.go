package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"delmos/internal/auth"
	"delmos/internal/project"
)

type phaseTransitionRequest struct {
	TargetStatus string `json:"target_status"`
}

// handleTransitionPhase переводить фазу проєкту в наступний стан життєвого
// циклу. Право plan.apply, бо перехід фази — це введення в дію стану плану,
// а не редагування його змісту.
//
// Коди відповіді за ADR-009 §4: 403 — немає дозволу, 422 — інваріант не
// виконано (вето правила чи неприпустимий перехід).
func handleTransitionPhase(authSvc *auth.Service, projects *project.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, projectID, ok := projectScopePermission(w, r, authSvc, "plan.apply")
		if !ok {
			return
		}

		var req phaseTransitionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_body", "очікувалося поле target_status")
			return
		}
		if req.TargetStatus == "" {
			writeJSONError(w, http.StatusBadRequest, "invalid_body", "поле target_status обов'язкове")
			return
		}

		result, err := projects.TransitionPhase(r.Context(), actor, projectID,
			r.PathValue("phase_key"), req.TargetStatus, time.Now().UTC())
		switch {
		case err == nil:
			writeJSON(w, http.StatusOK, result)
		case errors.Is(err, project.ErrPhaseNotFound):
			writeJSONError(w, http.StatusNotFound, "phase_not_found", "фазу не знайдено в проєкті")
		case errors.Is(err, project.ErrPhaseTransitionInvalid):
			writeJSONError(w, http.StatusUnprocessableEntity, "invalid_transition", err.Error())
		case errors.Is(err, project.ErrPhaseGateRejected):
			writeJSONError(w, http.StatusUnprocessableEntity, "gate_rejected", err.Error())
		default:
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося змінити статус фази")
		}
	}
}
