package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"delmos/internal/auth"
)

type grantRoleRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role_key"`
	Reason string `json:"reason"`
}

type roleBindingView struct {
	ID string `json:"id"`
}

// handleGrantSystemRole надає System-scope RoleBinding (SWR-43); самопризначення
// вище власної стелі тут не перевіряється — див. TODO у ADR для наступної версії.
func handleGrantSystemRole(authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authContextFrom(r.Context())
		if !ok || !actor.HasPermission("access.grant") {
			writeJSONError(w, http.StatusForbidden, "forbidden", "недостатньо прав для надання ролі")
			return
		}

		var req grantRoleRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBodyBytes)).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_body", "некоректне тіло запиту")
			return
		}

		userID, err := uuid.Parse(req.UserID)
		if err != nil || req.Role == "" || req.Reason == "" {
			writeJSONError(w, http.StatusUnprocessableEntity, "invalid_body", "user_id, role_key і reason обов'язкові")
			return
		}

		bindingID, err := authSvc.GrantSystemRole(r.Context(), actor.UserID, userID, req.Role, req.Reason)
		switch {
		case errors.Is(err, auth.ErrRoleUnknown):
			writeJSONError(w, http.StatusUnprocessableEntity, "unknown_role", err.Error())
			return
		case errors.Is(err, auth.ErrUserUnknown):
			writeJSONError(w, http.StatusUnprocessableEntity, "unknown_user", err.Error())
			return
		case err != nil:
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося надати роль")
			return
		}

		writeJSON(w, http.StatusCreated, roleBindingView{ID: bindingID.String()})
	}
}

// handleRevokeRoleBinding відкликає RoleBinding для наступних запитів (SWR-48 §1).
func handleRevokeRoleBinding(authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authContextFrom(r.Context())
		if !ok || !actor.HasPermission("access.revoke") {
			writeJSONError(w, http.StatusForbidden, "forbidden", "недостатньо прав для відкликання ролі")
			return
		}

		bindingID, err := uuid.Parse(r.PathValue("binding_id"))
		if err != nil {
			writeJSONError(w, http.StatusNotFound, "not_found", "прив'язку не знайдено")
			return
		}

		if _, err := authSvc.RevokeRoleBinding(r.Context(), actor.UserID, bindingID); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося відкликати роль")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func handleSearchReviewRoleProjects(authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authContextFrom(r.Context())
		if !ok || !actor.HasPermission("access.grant") {
			writeJSONError(w, http.StatusForbidden, "forbidden", "недостатньо прав для керування ролями")
			return
		}
		projects, err := authSvc.SearchReviewRoleProjects(r.Context(), strings.TrimSpace(r.URL.Query().Get("query")))
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося знайти проєкти")
			return
		}
		writeJSON(w, http.StatusOK, projects)
	}
}

func handleSearchReviewRoleUsers(authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authContextFrom(r.Context())
		if !ok || !actor.HasPermission("access.grant") {
			writeJSONError(w, http.StatusForbidden, "forbidden", "недостатньо прав для керування ролями")
			return
		}
		query := strings.TrimSpace(r.URL.Query().Get("query"))
		if len([]rune(query)) < 2 || len([]rune(query)) > 80 {
			writeJSONError(w, http.StatusBadRequest, "invalid_query", "вкажіть від 2 до 80 символів для пошуку")
			return
		}
		users, err := authSvc.SearchReviewRoleUsers(r.Context(), query, actor.UserID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося знайти користувачів")
			return
		}
		writeJSON(w, http.StatusOK, users)
	}
}

func handleListProjectReviewRoleBindings(authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authContextFrom(r.Context())
		if !ok || (!actor.HasPermission("access.grant") && !actor.HasPermission("access.revoke")) {
			writeJSONError(w, http.StatusForbidden, "forbidden", "недостатньо прав для керування ролями")
			return
		}
		projectID, err := uuid.Parse(r.PathValue("project_id"))
		if err != nil {
			writeJSONError(w, http.StatusNotFound, "not_found", "проєкт не знайдено")
			return
		}
		bindings, err := authSvc.ListProjectReviewRoleBindings(r.Context(), projectID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося прочитати ролі проєкту")
			return
		}
		writeJSON(w, http.StatusOK, bindings)
	}
}

func handleGrantProjectReviewRole(authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authContextFrom(r.Context())
		if !ok || !actor.HasPermission("access.grant") {
			writeJSONError(w, http.StatusForbidden, "forbidden", "недостатньо прав для надання ролі")
			return
		}
		projectID, err := uuid.Parse(r.PathValue("project_id"))
		if err != nil {
			writeJSONError(w, http.StatusNotFound, "not_found", "проєкт не знайдено")
			return
		}
		var req grantRoleRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBodyBytes)).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_body", "некоректне тіло запиту")
			return
		}
		userID, err := uuid.Parse(req.UserID)
		if err != nil || strings.TrimSpace(req.Reason) == "" {
			writeJSONError(w, http.StatusUnprocessableEntity, "invalid_body", "користувач і підстава обов'язкові")
			return
		}
		bindingID, err := authSvc.GrantProjectReviewRole(r.Context(), actor.UserID, userID, projectID, req.Role, req.Reason)
		switch {
		case errors.Is(err, auth.ErrReviewRoleInvalid), errors.Is(err, auth.ErrReviewRoleSelfGrant):
			writeJSONError(w, http.StatusUnprocessableEntity, "invalid_role", err.Error())
		case errors.Is(err, auth.ErrReviewRoleTargetMissing):
			writeJSONError(w, http.StatusNotFound, "not_found", err.Error())
		case errors.Is(err, auth.ErrReviewRoleActive):
			writeJSONError(w, http.StatusConflict, "role_active", err.Error())
		case err != nil:
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося надати роль")
		default:
			writeJSON(w, http.StatusCreated, roleBindingView{ID: bindingID.String()})
		}
	}
}

func handleRevokeProjectReviewRole(authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authContextFrom(r.Context())
		if !ok || !actor.HasPermission("access.revoke") {
			writeJSONError(w, http.StatusForbidden, "forbidden", "недостатньо прав для відкликання ролі")
			return
		}
		projectID, projectErr := uuid.Parse(r.PathValue("project_id"))
		bindingID, bindingErr := uuid.Parse(r.PathValue("binding_id"))
		if projectErr != nil || bindingErr != nil {
			writeJSONError(w, http.StatusNotFound, "not_found", "прив'язку не знайдено")
			return
		}
		revoked, err := authSvc.RevokeProjectReviewRole(r.Context(), actor.UserID, projectID, bindingID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося відкликати роль")
			return
		}
		if !revoked {
			writeJSONError(w, http.StatusNotFound, "not_found", "прив'язку не знайдено")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
