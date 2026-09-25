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

type createProjectRequest struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type planView struct {
	Code           string         `json:"code"`
	Type           string         `json:"type"`
	Title          string         `json:"title"`
	Status         string         `json:"status"`
	RevisionNumber int            `json:"revision_number"`
	Body           string         `json:"body"`
	Metadata       map[string]any `json:"metadata"`
	PayloadHash    string         `json:"payload_hash"`
}

type projectView struct {
	ID          string   `json:"id"`
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Plan        planView `json:"plan"`
	Permissions []string `json:"permissions,omitempty"`
}

type projectSummaryView struct {
	ID     string `json:"id"`
	Code   string `json:"code"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

func toProjectView(detail project.ProjectDetail, permissions ...map[string]bool) projectView {
	var permKeys []string
	if len(permissions) > 0 && permissions[0] != nil {
		for k, v := range permissions[0] {
			if v {
				permKeys = append(permKeys, k)
			}
		}
	}
	return projectView{
		ID: detail.Project.ID.String(), Code: detail.Project.Code, Name: detail.Project.Name,
		Description: detail.Project.Description, Status: detail.Project.Status,
		Plan: planView{
			Code: detail.Plan.Code, Type: detail.Plan.Type, Title: detail.Plan.Title, Status: detail.Plan.Status,
			RevisionNumber: detail.PlanLatest.RevisionNumber, Body: detail.PlanLatest.Body,
			Metadata: detail.PlanLatest.Metadata, PayloadHash: base64.RawURLEncoding.EncodeToString(detail.PlanLatest.PayloadHash),
		},
		Permissions: permKeys,
	}
}

// handleCreateProject атомарно створює Project + PLAN-001 (CORE-CONTRACT-001: createProject).
func handleCreateProject(projects *project.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authContextFrom(r.Context())
		if !ok || !actor.HasPermission("scopes.manage") {
			writeJSONError(w, http.StatusForbidden, "forbidden", "недостатньо прав для створення проєкту")
			return
		}

		var req createProjectRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBodyBytes)).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_body", "некоректне тіло запиту")
			return
		}
		if req.Code == "" || req.Name == "" {
			writeJSONError(w, http.StatusUnprocessableEntity, "invalid_body", "code і name обов'язкові")
			return
		}

		detail, err := projects.CreateWithPlan(r.Context(), actor.UserID, req.Code, req.Name, req.Description)
		switch {
		case errors.Is(err, project.ErrCodeTaken):
			writeJSONError(w, http.StatusConflict, "code_taken", err.Error())
			return
		case err != nil:
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося створити проєкт")
			return
		}

		writeJSON(w, http.StatusCreated, toProjectView(detail))
	}
}

// handleListProjects повертає лише проєкти, де актор має чинний Project-scope RoleBinding.
func handleListProjects(projects *project.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authContextFrom(r.Context())
		if !ok {
			writeJSONError(w, http.StatusUnauthorized, "unauthenticated", "сесія відсутня")
			return
		}

		summaries, err := projects.ListForActor(r.Context(), actor.UserID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося прочитати список проєктів")
			return
		}

		views := make([]projectSummaryView, 0, len(summaries))
		for _, s := range summaries {
			views = append(views, projectSummaryView{ID: s.ID.String(), Code: s.Code, Name: s.Name, Status: s.Status})
		}

		writeJSON(w, http.StatusOK, views)
	}
}

// handleGetProject повертає проєкт лише за наявності project.read у цьому Project scope;
// невідомий і недоступний проєкт повертають однакову 404 (SWR-44 §2).
func handleGetProject(authSvc *auth.Service, projects *project.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authContextFrom(r.Context())
		if !ok {
			writeJSONError(w, http.StatusUnauthorized, "unauthenticated", "сесія відсутня")
			return
		}

		projectID, err := uuid.Parse(r.PathValue("project_id"))
		if err != nil {
			writeJSONError(w, http.StatusNotFound, "not_found", "проєкт не знайдено")
			return
		}

		permissions, err := authSvc.ActiveProjectPermissions(r.Context(), actor.UserID, projectID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося перевірити права доступу")
			return
		}
		if !permissions["project.read"] {
			writeJSONError(w, http.StatusNotFound, "not_found", "проєкт не знайдено")
			return
		}

		detail, err := projects.Get(r.Context(), projectID)
		switch {
		case errors.Is(err, project.ErrNotFound):
			writeJSONError(w, http.StatusNotFound, "not_found", "проєкт не знайдено")
			return
		case err != nil:
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося прочитати проєкт")
			return
		}

		writeJSON(w, http.StatusOK, toProjectView(detail, permissions))
	}
}
