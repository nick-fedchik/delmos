package server

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"delmos/internal/auth"
	"delmos/internal/project"
)

type createSpecificationRequest struct {
	Code     string                       `json:"code"`
	Title    string                       `json:"title"`
	Body     string                       `json:"body"`
	Metadata map[string]any               `json:"metadata"`
	Manifest specificationManifestRequest `json:"manifest"`
}

type reviseSpecificationRequest struct {
	ExpectedRowVersion int64                        `json:"expected_row_version"`
	Body               string                       `json:"body"`
	Metadata           map[string]any               `json:"metadata"`
	Manifest           specificationManifestRequest `json:"manifest"`
}

type specificationManifestRequest struct {
	ManifestVersion string                           `json:"manifest_version"`
	Sections        []project.CompositionSection     `json:"sections"`
	Occurrences     []specificationOccurrenceRequest `json:"occurrences"`
	ElementsHash    string                           `json:"elements_hash"`
}

type specificationOccurrenceRequest struct {
	OccurrenceID        uuid.UUID `json:"occurrence_id"`
	SectionID           string    `json:"section_id"`
	TargetWorkProductID uuid.UUID `json:"target_wp_id"`
	TargetRevisionID    uuid.UUID `json:"target_revision_id"`
	TargetPayloadHash   string    `json:"target_payload_hash"`
	Order               int       `json:"order"`
}

func (request specificationManifestRequest) manifest() (project.CompositionManifest, error) {
	elementsHash, err := hex.DecodeString(request.ElementsHash)
	if err != nil {
		return project.CompositionManifest{}, project.ErrInvalidCompositionManifest
	}

	occurrences := make([]project.CompositionOccurrence, 0, len(request.Occurrences))
	for _, occurrence := range request.Occurrences {
		occurrences = append(occurrences, project.CompositionOccurrence{
			OccurrenceID: occurrence.OccurrenceID, SectionID: occurrence.SectionID,
			TargetWorkProductID: occurrence.TargetWorkProductID, TargetRevisionID: occurrence.TargetRevisionID,
			TargetPayloadHash: occurrence.TargetPayloadHash, Order: occurrence.Order,
		})
	}
	return project.CompositionManifest{
		ManifestVersion: request.ManifestVersion,
		Sections:        request.Sections, Occurrences: occurrences, ElementsHash: elementsHash,
	}, nil
}

func handleCreateSpecification(authSvc *auth.Service, projects *project.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, projectID, ok := projectScopePermission(w, r, authSvc, "wp.create")
		if !ok {
			return
		}

		var request createSpecificationRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBodyBytes)).Decode(&request); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_body", "некоректне тіло запиту")
			return
		}
		manifest, err := request.Manifest.manifest()
		if err != nil {
			writeJSONError(w, http.StatusUnprocessableEntity, "invalid_manifest", err.Error())
			return
		}

		workProduct, revision, specification, err := projects.CreateSpecification(r.Context(), actorID, projectID,
			request.Code, request.Title, request.Body, request.Metadata, manifest)
		switch {
		case errors.Is(err, project.ErrInvalidCode), errors.Is(err, project.ErrInvalidCompositionManifest), errors.Is(err, project.ErrCompositionCycle), errors.Is(err, project.ErrCompositionTargetInvalid):
			writeJSONError(w, http.StatusUnprocessableEntity, "invalid_manifest", err.Error())
			return
		case errors.Is(err, project.ErrWorkProductCodeTaken):
			writeJSONError(w, http.StatusConflict, "code_taken", err.Error())
			return
		case err != nil:
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося створити специфікацію")
			return
		}

		writeJSON(w, http.StatusCreated, specificationView{
			ID: specification.ID.String(), WorkProduct: toWorkProductView(workProduct, revision), Manifest: toSpecificationManifestView(specification.Manifest),
		})
	}
}

type specificationView struct {
	ID          string                    `json:"specification_id"`
	WorkProduct workProductView           `json:"work_product"`
	Manifest    specificationManifestView `json:"manifest"`
}

type specificationManifestView struct {
	ManifestVersion string                          `json:"manifest_version"`
	Sections        []project.CompositionSection    `json:"sections"`
	Occurrences     []project.CompositionOccurrence `json:"occurrences"`
	ElementsHash    string                          `json:"elements_hash"`
}

func toSpecificationManifestView(manifest project.CompositionManifest) specificationManifestView {
	return specificationManifestView{
		ManifestVersion: manifest.ManifestVersion,
		Sections:        manifest.Sections,
		Occurrences:     manifest.Occurrences,
		ElementsHash:    hex.EncodeToString(manifest.ElementsHash),
	}
}

func handleGetSpecification(authSvc *auth.Service, projects *project.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, projectID, ok := projectScopePermission(w, r, authSvc, "wp.read")
		if !ok {
			return
		}
		specificationID, err := uuid.Parse(r.PathValue("specification_id"))
		if err != nil {
			writeJSONError(w, http.StatusNotFound, "not_found", "специфікацію не знайдено")
			return
		}

		workProduct, revision, specification, err := projects.GetSpecification(r.Context(), projectID, specificationID)
		if errors.Is(err, project.ErrSpecificationNotFound) {
			writeJSONError(w, http.StatusNotFound, "not_found", "специфікацію не знайдено")
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося прочитати специфікацію")
			return
		}

		writeJSON(w, http.StatusOK, specificationView{
			ID: specification.ID.String(), WorkProduct: toWorkProductView(workProduct, revision), Manifest: toSpecificationManifestView(specification.Manifest),
		})
	}
}

func handleReviseSpecification(authSvc *auth.Service, projects *project.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, projectID, ok := projectScopePermission(w, r, authSvc, "wp.edit")
		if !ok {
			return
		}
		specificationID, err := uuid.Parse(r.PathValue("specification_id"))
		if err != nil {
			writeJSONError(w, http.StatusNotFound, "not_found", "специфікацію не знайдено")
			return
		}
		var request reviseSpecificationRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBodyBytes)).Decode(&request); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_body", "некоректне тіло запиту")
			return
		}
		manifest, err := request.Manifest.manifest()
		if err != nil {
			writeJSONError(w, http.StatusUnprocessableEntity, "invalid_manifest", err.Error())
			return
		}
		workProduct, revision, specification, err := projects.ReviseSpecification(r.Context(), actorID, projectID, specificationID,
			request.ExpectedRowVersion, request.Body, request.Metadata, manifest)
		switch {
		case errors.Is(err, project.ErrSpecificationNotFound):
			writeJSONError(w, http.StatusNotFound, "not_found", "специфікацію не знайдено")
		case errors.Is(err, project.ErrVersionConflict):
			writeJSONError(w, http.StatusConflict, "version_conflict", err.Error())
		case errors.Is(err, project.ErrWorkProductObsolete):
			writeJSONError(w, http.StatusUnprocessableEntity, "obsolete", err.Error())
		case errors.Is(err, project.ErrInvalidCompositionManifest), errors.Is(err, project.ErrCompositionCycle), errors.Is(err, project.ErrCompositionTargetInvalid):
			writeJSONError(w, http.StatusUnprocessableEntity, "invalid_manifest", err.Error())
		case err != nil:
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося створити ревізію специфікації")
		default:
			writeJSON(w, http.StatusCreated, specificationView{
				ID: specification.ID.String(), WorkProduct: toWorkProductView(workProduct, revision), Manifest: toSpecificationManifestView(specification.Manifest),
			})
		}
	}
}
