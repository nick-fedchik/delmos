package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"delmos/internal/auth"
	"delmos/internal/project"

	"github.com/google/uuid"
)

type createTraceLinkRequest struct {
	SourceID         uuid.UUID  `json:"source_id"`
	SourceRevisionID *uuid.UUID `json:"source_revision_id"`
	TargetID         uuid.UUID  `json:"target_id"`
	TargetRevisionID *uuid.UUID `json:"target_revision_id"`
	RelationKind     string     `json:"relation_kind"`
}

type traceLinkView struct {
	ID               string  `json:"id"`
	SourceID         string  `json:"source_id"`
	SourceRevisionID *string `json:"source_revision_id,omitempty"`
	TargetID         string  `json:"target_id"`
	TargetRevisionID *string `json:"target_revision_id,omitempty"`
	RelationKind     string  `json:"relation_kind"`
	IsSuspect        bool    `json:"is_suspect"`
}

type tracePathEntryView struct {
	Depth            int     `json:"depth"`
	SourceID         string  `json:"source_id"`
	SourceCode       string  `json:"source_code"`
	TargetID         string  `json:"target_id"`
	TargetCode       string  `json:"target_code"`
	TargetType       string  `json:"target_type"`
	SourceRevisionID *string `json:"source_revision_id,omitempty"`
	TargetRevisionID *string `json:"target_revision_id,omitempty"`
	RelationKind     string  `json:"relation_kind"`
	IsSuspect        bool    `json:"is_suspect"`
	IsCycle          bool    `json:"is_cycle"`
}

func optionalUUIDString(value *uuid.UUID) *string {
	if value == nil {
		return nil
	}
	result := value.String()
	return &result
}

func toTraceLinkView(link project.TraceLink) traceLinkView {
	return traceLinkView{ID: link.ID.String(), SourceID: link.SourceID.String(), SourceRevisionID: optionalUUIDString(link.SourceRevisionID), TargetID: link.TargetID.String(), TargetRevisionID: optionalUUIDString(link.TargetRevisionID), RelationKind: link.RelationKind, IsSuspect: link.IsSuspect}
}

func handleCreateTraceLink(authSvc *auth.Service, projects *project.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, projectID, ok := projectScopePermission(w, r, authSvc, "trace.create")
		if !ok {
			return
		}
		var request createTraceLinkRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBodyBytes)).Decode(&request); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_body", "некоректне тіло запиту")
			return
		}
		link, err := projects.CreateTraceLink(r.Context(), actorID, projectID, request.SourceID, request.TargetID, request.SourceRevisionID, request.TargetRevisionID, request.RelationKind)
		switch {
		case errors.Is(err, project.ErrInvalidTraceRelation), errors.Is(err, project.ErrTraceEndpointInvalid):
			writeJSONError(w, http.StatusUnprocessableEntity, "invalid_trace_link", err.Error())
		case errors.Is(err, project.ErrTraceLinkAlreadyExists):
			writeJSONError(w, http.StatusConflict, "trace_link_exists", err.Error())
		case err != nil:
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося створити зв'язок")
		default:
			writeJSON(w, http.StatusCreated, toTraceLinkView(link))
		}
	}
}

func handleTraverseTraceability(authSvc *auth.Service, projects *project.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, projectID, ok := projectScopePermission(w, r, authSvc, "wp.read")
		if !ok {
			return
		}
		sourceID, err := uuid.Parse(r.PathValue("source_id"))
		if err != nil {
			writeJSONError(w, http.StatusNotFound, "not_found", "work product не знайдено")
			return
		}
		entries, err := projects.TraverseTraceability(r.Context(), projectID, sourceID)
		if errors.Is(err, project.ErrTraceEndpointInvalid) {
			writeJSONError(w, http.StatusNotFound, "not_found", "work product не знайдено")
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося прочитати граф простежуваності")
			return
		}
		views := make([]tracePathEntryView, 0, len(entries))
		for _, entry := range entries {
			views = append(views, tracePathEntryView{Depth: entry.Depth, SourceID: entry.SourceID.String(), SourceCode: entry.SourceCode, TargetID: entry.TargetID.String(), TargetCode: entry.TargetCode, TargetType: entry.TargetType, SourceRevisionID: optionalUUIDString(entry.SourceRevisionID), TargetRevisionID: optionalUUIDString(entry.TargetRevisionID), RelationKind: entry.RelationKind, IsSuspect: entry.IsSuspect, IsCycle: entry.IsCycle})
		}
		writeJSON(w, http.StatusOK, views)
	}
}

// handleListTraceLinks повертає реєстр зв'язків проєкту — аудиторський список
// перегляду для Change Impact Analysis (GRP-03). ?suspect=true фільтрує лише
// підозрілі зв'язки, що очікують підтвердження рецензентом.
func handleListTraceLinks(authSvc *auth.Service, projects *project.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, projectID, ok := projectScopePermission(w, r, authSvc, "wp.read")
		if !ok {
			return
		}
		onlySuspect := r.URL.Query().Get("suspect") == "true"
		links, err := projects.ListTraceLinks(r.Context(), projectID, onlySuspect)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося прочитати зв'язки простежуваності")
			return
		}
		views := make([]traceLinkView, 0, len(links))
		for _, link := range links {
			views = append(views, toTraceLinkView(link))
		}
		writeJSON(w, http.StatusOK, views)
	}
}

// handleAcknowledgeTraceLink знімає прапорець is_suspect за явною дією
// рецензента після підтвердження відповідності (VECTOR_AND_GRAPH_DATA.md §3.3).
func handleAcknowledgeTraceLink(authSvc *auth.Service, projects *project.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, projectID, ok := projectScopePermission(w, r, authSvc, "trace.create")
		if !ok {
			return
		}
		linkID, err := uuid.Parse(r.PathValue("trace_link_id"))
		if err != nil {
			writeJSONError(w, http.StatusNotFound, "not_found", "зв'язок простежуваності не знайдено")
			return
		}
		if err := projects.AcknowledgeTraceLink(r.Context(), actorID, projectID, linkID); err != nil {
			if errors.Is(err, project.ErrTraceLinkNotFound) {
				writeJSONError(w, http.StatusNotFound, "not_found", "зв'язок простежуваності не знайдено")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося зняти прапорець підозрілості")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
