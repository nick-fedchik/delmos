package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"delmos/internal/auth"
	"delmos/internal/project"
)

type submitRequest struct {
	Assignments []struct {
		UserID uuid.UUID `json:"user_id"`
		Role   string    `json:"assignment_role"`
	} `json:"assignments"`
}

// handleSubmitWorkProduct подає останню ревізію артефакту на рецензію.
//
// Призначення передаються явно: право дає змогу діяти, а призначення визначає,
// над чим саме (ADR-009 §1). Коди відповіді за ADR-009 §4 — 409 на конфлікт
// стану, 422 на невиконаний інваріант.
func handleSubmitWorkProduct(authSvc *auth.Service, projects *project.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, projectID, ok := projectScopePermission(w, r, authSvc, "wp.submit")
		if !ok {
			return
		}
		workProductID, err := uuid.Parse(r.PathValue("work_product_id"))
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_work_product_id", "некоректний ідентифікатор артефакту")
			return
		}

		var req submitRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_body", "очікувався масив assignments")
			return
		}
		assignments := make([]project.ReviewAssignment, 0, len(req.Assignments))
		for _, a := range req.Assignments {
			if a.Role != "reviewer" && a.Role != "approver" {
				writeJSONError(w, http.StatusBadRequest, "invalid_assignment_role",
					"assignment_role має бути reviewer або approver")
				return
			}
			assignments = append(assignments, project.ReviewAssignment{UserID: a.UserID, Role: a.Role})
		}

		request, err := projects.SubmitWorkProduct(r.Context(), actor, projectID, workProductID, assignments)
		switch {
		case err == nil:
			writeJSON(w, http.StatusCreated, request)
		case errors.Is(err, project.ErrWorkProductNotFound):
			writeJSONError(w, http.StatusNotFound, "work_product_not_found", "артефакт не знайдено")
		case errors.Is(err, project.ErrReviewRequestOpen):
			writeJSONError(w, http.StatusConflict, "review_request_open", "артефакт уже подано на рецензію")
		case errors.Is(err, project.ErrNotDraft):
			writeJSONError(w, http.StatusUnprocessableEntity, "not_draft", err.Error())
		case errors.Is(err, project.ErrNoRevision):
			writeJSONError(w, http.StatusUnprocessableEntity, "no_revision", "артефакт не має жодної ревізії")
		case errors.Is(err, project.ErrAssigneeIsAuthor):
			writeJSONError(w, http.StatusUnprocessableEntity, "assignee_is_author", err.Error())
		default:
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося подати артефакт на рецензію")
		}
	}
}
