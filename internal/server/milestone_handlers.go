package server

import (
	"errors"
	"net/http"

	"delmos/internal/auth"
	"delmos/internal/automation"
	"delmos/internal/project"
)

func handleAcceptMilestone(authSvc *auth.Service, projects *project.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, projectID, ok := projectScopePermission(w, r, authSvc, "milestone.accept")
		if !ok {
			return
		}
		decision, err := projects.AcceptMilestone(r.Context(), actorID, projectID, r.PathValue("milestone_key"))
		var violation *automation.RuleViolationError
		switch {
		case err == nil:
			writeJSON(w, http.StatusOK, decision)
		case errors.Is(err, project.ErrMilestoneNotFound):
			writeJSONError(w, http.StatusNotFound, "milestone_not_found", err.Error())
		case errors.As(err, &violation), errors.Is(err, project.ErrSelfGateDecision), errors.Is(err, project.ErrMilestoneNotReady),
			errors.Is(err, project.ErrPlanNotApproved), errors.Is(err, project.ErrUnsupportedAcceptanceRule):
			writeJSONError(w, http.StatusUnprocessableEntity, "gate_rejected", err.Error())
		default:
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося прийняти віху")
		}
	}
}
