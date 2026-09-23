package server

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"delmos/internal/auth"
	"delmos/internal/project"
)

type createWorkProductRequest struct {
	Code     string         `json:"code"`
	Type     string         `json:"type"`
	Title    string         `json:"title"`
	Body     string         `json:"body"`
	Metadata map[string]any `json:"metadata"`
}

type reviseWorkProductRequest struct {
	ExpectedRowVersion int64          `json:"expected_row_version"`
	Body               string         `json:"body"`
	Metadata           map[string]any `json:"metadata"`
}

type retireWorkProductRequest struct {
	ExpectedRowVersion int64 `json:"expected_row_version"`
}

type workProductRevisionView struct {
	RevisionNumber int            `json:"revision_number"`
	Body           string         `json:"body"`
	Metadata       map[string]any `json:"metadata"`
	PayloadHash    string         `json:"payload_hash"`
	ContentHash    string         `json:"content_hash"`
}

type workProductView struct {
	ID         string                  `json:"id"`
	Code       string                  `json:"code"`
	Type       string                  `json:"type"`
	Title      string                  `json:"title"`
	Status     string                  `json:"status"`
	RowVersion int64                   `json:"row_version"`
	Latest     workProductRevisionView `json:"latest_revision"`
}

type workProductSummaryView struct {
	ID     string `json:"id"`
	Code   string `json:"code"`
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

func toWorkProductView(wp project.WorkProduct, revision project.WorkProductRevision) workProductView {
	return workProductView{
		ID: wp.ID.String(), Code: wp.Code, Type: wp.Type, Title: wp.Title, Status: wp.Status, RowVersion: wp.RowVersion,
		Latest: workProductRevisionView{
			RevisionNumber: revision.RevisionNumber, Body: revision.Body, Metadata: revision.Metadata,
			PayloadHash: base64.RawURLEncoding.EncodeToString(revision.PayloadHash),
			ContentHash: base64.RawURLEncoding.EncodeToString(revision.ContentHash),
		},
	}
}

// projectScopePermission перевіряє дозвіл актора в конкретному Project scope і
// повертає 404 (не 403) на невідомий/недоступний проєкт (SWR-44 §2).
func projectScopePermission(w http.ResponseWriter, r *http.Request, authSvc *auth.Service, permission string) (uuid.UUID, uuid.UUID, bool) {
	actor, ok := authContextFrom(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthenticated", "сесія відсутня")
		return uuid.Nil, uuid.Nil, false
	}

	projectID, err := uuid.Parse(r.PathValue("project_id"))
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "not_found", "проєкт не знайдено")
		return uuid.Nil, uuid.Nil, false
	}

	permissions, err := authSvc.ActiveProjectPermissions(r.Context(), actor.UserID, projectID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося перевірити права доступу")
		return uuid.Nil, uuid.Nil, false
	}
	if !permissions[permission] {
		writeJSONError(w, http.StatusNotFound, "not_found", "проєкт не знайдено")
		return uuid.Nil, uuid.Nil, false
	}

	return actor.UserID, projectID, true
}

func handleCreateWorkProduct(authSvc *auth.Service, projects *project.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, projectID, ok := projectScopePermission(w, r, authSvc, "wp.create")
		if !ok {
			return
		}

		var req createWorkProductRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBodyBytes)).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_body", "некоректне тіло запиту")
			return
		}

		wp, revision, err := projects.CreateWorkProduct(r.Context(), actorID, projectID, req.Code, req.Type, req.Title, req.Body, req.Metadata)
		switch {
		case errors.Is(err, project.ErrInvalidWorkProductType):
			writeJSONError(w, http.StatusUnprocessableEntity, "invalid_type", err.Error())
			return
		case errors.Is(err, project.ErrInvalidCode):
			writeJSONError(w, http.StatusUnprocessableEntity, "invalid_body", err.Error())
			return
		case errors.Is(err, project.ErrWorkProductCodeTaken):
			writeJSONError(w, http.StatusConflict, "code_taken", err.Error())
			return
		case err != nil:
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося створити work product")
			return
		}

		writeJSON(w, http.StatusCreated, toWorkProductView(wp, revision))
	}
}

func handleListWorkProducts(authSvc *auth.Service, projects *project.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, projectID, ok := projectScopePermission(w, r, authSvc, "project.read")
		if !ok {
			return
		}

		summaries, err := projects.ListWorkProducts(r.Context(), projectID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося прочитати список work products")
			return
		}

		views := make([]workProductSummaryView, 0, len(summaries))
		for _, s := range summaries {
			views = append(views, workProductSummaryView{ID: s.ID.String(), Code: s.Code, Type: s.Type, Title: s.Title, Status: s.Status})
		}

		writeJSON(w, http.StatusOK, views)
	}
}

func handleGetWorkProduct(authSvc *auth.Service, projects *project.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, projectID, ok := projectScopePermission(w, r, authSvc, "wp.read")
		if !ok {
			return
		}

		workProductID, err := uuid.Parse(r.PathValue("work_product_id"))
		if err != nil {
			writeJSONError(w, http.StatusNotFound, "not_found", "work product не знайдено")
			return
		}

		detail, err := projects.GetWorkProduct(r.Context(), projectID, workProductID)
		switch {
		case errors.Is(err, project.ErrWorkProductNotFound):
			writeJSONError(w, http.StatusNotFound, "not_found", "work product не знайдено")
			return
		case err != nil:
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося прочитати work product")
			return
		}

		writeJSON(w, http.StatusOK, toWorkProductView(detail.WorkProduct, detail.Latest))
	}
}

func handleReviseWorkProduct(authSvc *auth.Service, projects *project.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, projectID, ok := projectScopePermission(w, r, authSvc, "wp.edit")
		if !ok {
			return
		}

		workProductID, err := uuid.Parse(r.PathValue("work_product_id"))
		if err != nil {
			writeJSONError(w, http.StatusNotFound, "not_found", "work product не знайдено")
			return
		}

		var req reviseWorkProductRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBodyBytes)).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_body", "некоректне тіло запиту")
			return
		}

		revision, err := projects.ReviseWorkProduct(r.Context(), actorID, projectID, workProductID, req.ExpectedRowVersion, req.Body, req.Metadata)
		switch {
		case errors.Is(err, project.ErrWorkProductNotFound):
			writeJSONError(w, http.StatusNotFound, "not_found", "work product не знайдено")
			return
		case errors.Is(err, project.ErrWorkProductObsolete):
			writeJSONError(w, http.StatusUnprocessableEntity, "obsolete", err.Error())
			return
		case errors.Is(err, project.ErrVersionConflict):
			writeJSONError(w, http.StatusConflict, "version_conflict", err.Error())
			return
		case err != nil:
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося створити ревізію")
			return
		}

		writeJSON(w, http.StatusCreated, workProductRevisionView{
			RevisionNumber: revision.RevisionNumber, Body: revision.Body, Metadata: revision.Metadata,
			PayloadHash: base64.RawURLEncoding.EncodeToString(revision.PayloadHash),
			ContentHash: base64.RawURLEncoding.EncodeToString(revision.ContentHash),
		})
	}
}

func handleRetireWorkProduct(authSvc *auth.Service, projects *project.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, projectID, ok := projectScopePermission(w, r, authSvc, "wp.retire")
		if !ok {
			return
		}

		workProductID, err := uuid.Parse(r.PathValue("work_product_id"))
		if err != nil {
			writeJSONError(w, http.StatusNotFound, "not_found", "work product не знайдено")
			return
		}

		var req retireWorkProductRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBodyBytes)).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_body", "некоректне тіло запиту")
			return
		}

		err = projects.RetireWorkProduct(r.Context(), actorID, projectID, workProductID, req.ExpectedRowVersion)
		switch {
		case errors.Is(err, project.ErrWorkProductNotFound):
			writeJSONError(w, http.StatusNotFound, "not_found", "work product не знайдено")
			return
		case errors.Is(err, project.ErrVersionConflict):
			writeJSONError(w, http.StatusConflict, "version_conflict", err.Error())
			return
		case err != nil:
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося вивести work product з експлуатації")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
