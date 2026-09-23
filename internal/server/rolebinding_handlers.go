package server

import (
	"encoding/json"
	"errors"
	"net/http"

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
