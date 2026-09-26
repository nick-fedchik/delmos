package server

import (
	"errors"
	"net/http"
	"time"

	"delmos/internal/auth"
	"delmos/internal/economics"
)

// handleEarnedValue повертає показники здобутої цінності проєкту за ДСТУ
// ISO 21508 (SWR-41). Право economics.read відокремлене від wp.read: ставки
// собівартості та кошторис бачить не кожен, хто читає інженерні артефакти
// (ADR-006).
//
// Необовʼязковий параметр as_of задає контрольну дату; без нього береться
// поточна. Дата в майбутньому не відхиляється свідомо — це штатний сценарій
// прогнозу на кінець фази.
func handleEarnedValue(authSvc *auth.Service, store *economics.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, projectID, ok := projectScopePermission(w, r, authSvc, "economics.read")
		if !ok {
			return
		}

		asOf := time.Now().UTC()
		if raw := r.URL.Query().Get("as_of"); raw != "" {
			parsed, err := time.Parse(time.DateOnly, raw)
			if err != nil {
				writeJSONError(w, http.StatusBadRequest, "invalid_as_of",
					"параметр as_of має бути датою у форматі YYYY-MM-DD")
				return
			}
			asOf = parsed
		}

		snapshot, err := store.ComputeEarnedValue(r.Context(), projectID, asOf)
		if err != nil {
			if errors.Is(err, economics.ErrNoApprovedBaseline) {
				writeJSONError(w, http.StatusConflict, "no_approved_baseline",
					"для проєкту немає затвердженого базового кошторису")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "internal_error",
				"не вдалося розрахувати показники здобутої цінності")
			return
		}
		writeJSON(w, http.StatusOK, snapshot)
	}
}
