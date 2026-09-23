package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"delmos/internal/auth"
	"delmos/internal/project"
	"delmos/internal/repository"
)

type bindRepositoryRequest struct {
	RemoteURL     string `json:"remote_url"`
	DefaultBranch string `json:"default_branch"`
}

type repositoryBindingView struct {
	RemoteURL     string  `json:"remote_url"`
	DefaultBranch string  `json:"default_branch"`
	Status        string  `json:"status"`
	LastError     *string `json:"last_error,omitempty"`
}

func toRepositoryBindingView(b repository.Binding) repositoryBindingView {
	return repositoryBindingView{RemoteURL: b.RemoteURL, DefaultBranch: b.DefaultBranch, Status: b.Status, LastError: b.LastError}
}

func handleBindRepository(authSvc *auth.Service, repos *repository.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, projectID, ok := projectScopePermission(w, r, authSvc, "repository.manage")
		if !ok {
			return
		}

		var req bindRepositoryRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBodyBytes)).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_body", "некоректне тіло запиту")
			return
		}
		if req.RemoteURL == "" {
			writeJSONError(w, http.StatusUnprocessableEntity, "invalid_body", "remote_url обов'язковий")
			return
		}

		binding, err := repos.Bind(r.Context(), actorID, projectID, req.RemoteURL, req.DefaultBranch)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося прив'язати сховище")
			return
		}

		writeJSON(w, http.StatusCreated, toRepositoryBindingView(binding))
	}
}

func handleGetRepository(authSvc *auth.Service, repos *repository.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, projectID, ok := projectScopePermission(w, r, authSvc, "project.read")
		if !ok {
			return
		}

		binding, err := repos.Get(r.Context(), projectID)
		switch {
		case errors.Is(err, repository.ErrBindingNotFound):
			writeJSONError(w, http.StatusNotFound, "not_found", "проєкт не прив'язаний до сховища")
			return
		case err != nil:
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося прочитати прив'язку сховища")
			return
		}

		writeJSON(w, http.StatusOK, toRepositoryBindingView(binding))
	}
}

type exportResultView struct {
	CommitSHA string `json:"commit_sha"`
}

func handleExportWorkProduct(authSvc *auth.Service, projects *project.Store, repos *repository.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authContextFrom(r.Context())
		if !ok {
			writeJSONError(w, http.StatusUnauthorized, "unauthenticated", "сесія відсутня")
			return
		}

		actorID, projectID, ok := projectScopePermission(w, r, authSvc, "repository.manage")
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

		result, err := repos.ExportWorkProduct(r.Context(), actorID, projectID, detail.WorkProduct.Code, detail.Latest.Body,
			actor.Login, actor.Login+"@delmos.local")
		switch {
		case errors.Is(err, repository.ErrBindingNotFound):
			writeJSONError(w, http.StatusUnprocessableEntity, "no_repository", "проєкт не прив'язаний до сховища")
			return
		case err != nil:
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося експортувати work product у сховище")
			return
		}

		writeJSON(w, http.StatusOK, exportResultView{CommitSHA: result.CommitSHA})
	}
}
