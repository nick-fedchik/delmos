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

type decisionRequest struct {
	Reason       string `json:"reason"`
	OperationKey string `json:"operation_key"`
}

// handleReviewDecision обслуговує review / approval / request_changes.
//
// Право перевіряється на HTTP-шарі, призначення, SoD і кворум — атомарно в
// транзакції сховища (ADR-009 §4). Повтор із тим самим operation_key
// повертає попередній результат, а не другий підпис.
func handleReviewDecision(authSvc *auth.Service, projects *project.Store, kind, permission string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, projectID, ok := projectScopePermission(w, r, authSvc, permission)
		if !ok {
			return
		}
		workProductID, err := uuid.Parse(r.PathValue("work_product_id"))
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_work_product_id", "некоректний ідентифікатор артефакту")
			return
		}

		var req decisionRequest
		// Тіло необов'язкове для review та approval.
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&req)
		}

		decision, err := projects.RecordDecision(r.Context(), actor, projectID, workProductID,
			kind, req.Reason, req.OperationKey)
		switch {
		case err == nil:
			writeJSON(w, http.StatusCreated, decision)
		case errors.Is(err, project.ErrWorkProductNotFound):
			writeJSONError(w, http.StatusNotFound, "work_product_not_found", "артефакт не знайдено")
		case errors.Is(err, project.ErrNotAssigned):
			writeJSONError(w, http.StatusForbidden, "not_assigned", err.Error())
		case errors.Is(err, project.ErrSelfDecision):
			writeJSONError(w, http.StatusUnprocessableEntity, "self_decision", err.Error())
		case errors.Is(err, project.ErrStaleRevision):
			writeJSONError(w, http.StatusConflict, "stale_revision", err.Error())
		case errors.Is(err, project.ErrNotInReview):
			writeJSONError(w, http.StatusUnprocessableEntity, "not_in_review", err.Error())
		case errors.Is(err, project.ErrReviewRequestMissing):
			writeJSONError(w, http.StatusUnprocessableEntity, "no_open_request", err.Error())
		case errors.Is(err, project.ErrNoPositiveReview):
			writeJSONError(w, http.StatusUnprocessableEntity, "no_positive_review", err.Error())
		case errors.Is(err, project.ErrReasonRequired):
			writeJSONError(w, http.StatusUnprocessableEntity, "reason_required", err.Error())
		default:
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося зафіксувати рішення")
		}
	}
}
