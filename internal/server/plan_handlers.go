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

type planDetailView struct {
	WorkProductID           string                      `json:"work_product_id"`
	Code                    string                      `json:"code"`
	Title                   string                      `json:"title"`
	Status                  string                      `json:"status"`
	RowVersion              int64                       `json:"row_version"`
	RevisionID              string                      `json:"revision_id"`
	RevisionNumber          int                         `json:"revision_number"`
	PayloadHash             string                      `json:"payload_hash"`
	Body                    string                      `json:"body"`
	TemplateKey             string                      `json:"template_key"`
	TemplateVersion         int                         `json:"template_version"`
	Manifest                project.GenericPlanManifest `json:"manifest"`
	Permissions             []string                    `json:"permissions,omitempty"`
	EffectivePlanRevisionID *string                     `json:"effective_plan_revision_id,omitempty"`
	ConfigGeneration        int64                       `json:"config_generation"`
	Phases                  []project.ProjectPhase      `json:"phases,omitempty"`
	Milestones              []project.ProjectMilestone  `json:"milestones,omitempty"`
}

type revisePlanRequest struct {
	ExpectedRowVersion int64                       `json:"expected_row_version"`
	Body               string                      `json:"body"`
	Manifest           project.GenericPlanManifest `json:"manifest"`
}

type applyPlanRequest struct {
	ExpectedRowVersion int64   `json:"expected_row_version"`
	RevisionID         *string `json:"revision_id,omitempty"`
}

type entityDefinitionView struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	OwnerModule string `json:"owner_module"`
	Description string `json:"description"`
}

func toPlanDetailView(detail project.PlanDetail, permissions ...map[string]bool) planDetailView {
	var permKeys []string
	if len(permissions) > 0 && permissions[0] != nil {
		for k, v := range permissions[0] {
			if v {
				permKeys = append(permKeys, k)
			}
		}
	}
	var effRevID *string
	if detail.EffectivePlanRevisionID != nil {
		s := detail.EffectivePlanRevisionID.String()
		effRevID = &s
	}
	return planDetailView{
		WorkProductID: detail.WorkProduct.ID.String(), Code: detail.WorkProduct.Code, Title: detail.WorkProduct.Title,
		Status: detail.WorkProduct.Status, RowVersion: detail.WorkProduct.RowVersion, RevisionID: detail.Revision.ID.String(),
		RevisionNumber: detail.Revision.RevisionNumber, PayloadHash: base64.RawURLEncoding.EncodeToString(detail.Revision.PayloadHash),
		Body: detail.Revision.Body, TemplateKey: detail.TemplateKey, TemplateVersion: detail.TemplateVersion, Manifest: detail.Manifest,
		Permissions: permKeys, EffectivePlanRevisionID: effRevID, ConfigGeneration: detail.ConfigGeneration,
		Phases: detail.Phases, Milestones: detail.Milestones,
	}
}

func handleGetPlan(authSvc *auth.Service, projects *project.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, projectID, ok := projectScopePermission(w, r, authSvc, "project.read")
		if !ok {
			return
		}
		detail, err := projects.GetPlan(r.Context(), projectID)
		switch {
		case errors.Is(err, project.ErrPlanNotFound):
			writeJSONError(w, http.StatusNotFound, "not_found", "план проєкту не знайдено")
		case err != nil:
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося прочитати план проєкту")
		default:
			permissions, _ := authSvc.ActiveProjectPermissions(r.Context(), actorID, projectID)
			writeJSON(w, http.StatusOK, toPlanDetailView(detail, permissions))
		}
	}
}

func handleListEntityDefinitions(projects *project.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authContextFrom(r.Context()); !ok {
			writeJSONError(w, http.StatusUnauthorized, "unauthenticated", "сесія відсутня")
			return
		}
		definitions, err := projects.ListEntityDefinitions(r.Context())
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося прочитати реєстр сутностей")
			return
		}
		views := make([]entityDefinitionView, 0, len(definitions))
		for _, definition := range definitions {
			views = append(views, entityDefinitionView{Key: definition.Key, Name: definition.Name, Category: definition.Category, OwnerModule: definition.OwnerModule, Description: definition.Description})
		}
		writeJSON(w, http.StatusOK, views)
	}
}

func handleRevisePlan(authSvc *auth.Service, projects *project.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, projectID, ok := projectScopePermission(w, r, authSvc, "wp.edit")
		if !ok {
			return
		}
		var req revisePlanRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBodyBytes)).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_body", "некоректне тіло запиту")
			return
		}
		detail, err := projects.RevisePlan(r.Context(), actorID, projectID, req.ExpectedRowVersion, req.Body, req.Manifest)
		switch {
		case errors.Is(err, project.ErrInvalidPlanManifest):
			writeJSONError(w, http.StatusUnprocessableEntity, "invalid_plan_manifest", err.Error())
		case errors.Is(err, project.ErrVersionConflict):
			writeJSONError(w, http.StatusConflict, "version_conflict", err.Error())
		case errors.Is(err, project.ErrPlanNotFound):
			writeJSONError(w, http.StatusNotFound, "not_found", "план проєкту не знайдено")
		case err != nil:
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося створити ревізію плану")
		default:
			writeJSON(w, http.StatusCreated, toPlanDetailView(detail))
		}
	}
}

func handleApplyPlan(authSvc *auth.Service, projects *project.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, projectID, ok := projectScopePermission(w, r, authSvc, "plan.apply")
		if !ok {
			return
		}
		var req applyPlanRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBodyBytes)).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_body", "некоректне тіло запиту")
			return
		}

		var targetRevUUID *uuid.UUID
		if req.RevisionID != nil && *req.RevisionID != "" {
			u, err := uuid.Parse(*req.RevisionID)
			if err != nil {
				writeJSONError(w, http.StatusBadRequest, "invalid_body", "некоректний revision_id")
				return
			}
			targetRevUUID = &u
		}

		applied, err := projects.ApplyPlan(r.Context(), actorID, projectID, targetRevUUID, req.ExpectedRowVersion)
		switch {
		case errors.Is(err, project.ErrVersionConflict):
			writeJSONError(w, http.StatusConflict, "version_conflict", err.Error())
		case errors.Is(err, project.ErrPlanNotFound):
			writeJSONError(w, http.StatusNotFound, "not_found", "план проєкту не знайдено")
		case errors.Is(err, project.ErrInvalidPlanManifest):
			writeJSONError(w, http.StatusUnprocessableEntity, "invalid_plan_manifest", err.Error())
		case err != nil:
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося застосувати план проєкту")
		default:
			writeJSON(w, http.StatusOK, applied)
		}
	}
}

type createStakeholderRequest struct {
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	ContactRef string `json:"contact_ref"`
	Interest   string `json:"interest"`
}

func handleListStakeholders(authSvc *auth.Service, projects *project.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, projectID, ok := projectScopePermission(w, r, authSvc, "project.read")
		if !ok {
			return
		}
		items, err := projects.ListStakeholders(r.Context(), projectID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося прочитати стейкхолдерів")
			return
		}
		if items == nil {
			items = []project.Stakeholder{}
		}
		writeJSON(w, http.StatusOK, items)
	}
}

func handleCreateStakeholder(authSvc *auth.Service, projects *project.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, projectID, ok := projectScopePermission(w, r, authSvc, "wp.create")
		if !ok {
			return
		}
		var req createStakeholderRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBodyBytes)).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_body", "некоректне тіло запиту")
			return
		}
		if req.Name == "" || (req.Kind != "user" && req.Kind != "organization" && req.Kind != "external_party") {
			writeJSONError(w, http.StatusUnprocessableEntity, "invalid_body", "name та коректний kind обов'язкові")
			return
		}
		item, err := projects.CreateStakeholder(r.Context(), actorID, projectID, req.Kind, req.Name, req.ContactRef, req.Interest)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося створити стейкхолдера")
			return
		}
		writeJSON(w, http.StatusCreated, item)
	}
}

type createRiskRequest struct {
	Title            string `json:"title"`
	Description      string `json:"description"`
	Impact           string `json:"impact"`
	Likelihood       string `json:"likelihood"`
	ResponseStrategy string `json:"response_strategy"`
	OwnerRef         string `json:"owner_ref"`
}

func handleListRisks(authSvc *auth.Service, projects *project.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, projectID, ok := projectScopePermission(w, r, authSvc, "project.read")
		if !ok {
			return
		}
		items, err := projects.ListRisks(r.Context(), projectID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося прочитати ризики")
			return
		}
		if items == nil {
			items = []project.ProjectRisk{}
		}
		writeJSON(w, http.StatusOK, items)
	}
}

func handleCreateRisk(authSvc *auth.Service, projects *project.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, projectID, ok := projectScopePermission(w, r, authSvc, "wp.create")
		if !ok {
			return
		}
		var req createRiskRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBodyBytes)).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_body", "некоректне тіло запиту")
			return
		}
		if req.Title == "" {
			writeJSONError(w, http.StatusUnprocessableEntity, "invalid_body", "title обов'язковий")
			return
		}
		item, err := projects.CreateRisk(r.Context(), actorID, projectID, req.Title, req.Description, req.Impact, req.Likelihood, req.ResponseStrategy, req.OwnerRef)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося створити ризик")
			return
		}
		writeJSON(w, http.StatusCreated, item)
	}
}
