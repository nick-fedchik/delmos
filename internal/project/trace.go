package project

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrTraceLinkNotFound      = errors.New("зв'язок простежуваності не знайдено")
	ErrInvalidTraceRelation   = errors.New("невідомий тип зв'язку простежуваності")
	ErrTraceEndpointInvalid   = errors.New("кінцева точка зв'язку простежуваності недійсна")
	ErrTraceLinkAlreadyExists = errors.New("такий зв'язок простежуваності вже існує")
)

var traceRelationKinds = map[string]bool{
	"verifies": true, "satisfies": true, "refines": true, "derives_from": true,
}

type TraceLink struct {
	ID               uuid.UUID
	ProjectID        uuid.UUID
	SourceID         uuid.UUID
	SourceRevisionID *uuid.UUID
	TargetID         uuid.UUID
	TargetRevisionID *uuid.UUID
	RelationKind     string
	IsSuspect        bool
}

type TracePathEntry struct {
	Depth            int
	SourceID         uuid.UUID
	SourceCode       string
	TargetID         uuid.UUID
	TargetCode       string
	TargetType       string
	SourceRevisionID *uuid.UUID
	TargetRevisionID *uuid.UUID
	RelationKind     string
	IsSuspect        bool
	IsCycle          bool
}

func (s *Store) CreateTraceLink(ctx context.Context, actorID, projectID, sourceID, targetID uuid.UUID, sourceRevisionID, targetRevisionID *uuid.UUID, relationKind string) (TraceLink, error) {
	if !traceRelationKinds[relationKind] {
		return TraceLink{}, ErrInvalidTraceRelation
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TraceLink{}, fmt.Errorf("початок транзакції зв'язку простежуваності: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := validateTraceEndpoint(ctx, tx, projectID, sourceID, sourceRevisionID); err != nil {
		return TraceLink{}, err
	}
	if err := validateTraceEndpoint(ctx, tx, projectID, targetID, targetRevisionID); err != nil {
		return TraceLink{}, err
	}

	var link TraceLink
	err = tx.QueryRow(ctx,
		`INSERT INTO core.trace_links
		    (project_id, source_id, source_revision_id, target_id, target_revision_id, relation_kind)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, project_id, source_id, source_revision_id, target_id, target_revision_id, relation_kind, is_suspect`,
		projectID, sourceID, sourceRevisionID, targetID, targetRevisionID, relationKind,
	).Scan(&link.ID, &link.ProjectID, &link.SourceID, &link.SourceRevisionID, &link.TargetID,
		&link.TargetRevisionID, &link.RelationKind, &link.IsSuspect)
	if err != nil {
		if isUniqueViolation(err) {
			return TraceLink{}, ErrTraceLinkAlreadyExists
		}
		return TraceLink{}, fmt.Errorf("створення зв'язку простежуваності: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO core.audit_events (actor_user_id, action, scope_type, scope_id, outcome, detail, correlation_id)
		 VALUES ($1, 'trace.create', 'project', $2, 'success', $3, $4)`,
		actorID, projectID, map[string]any{"trace_link_id": link.ID.String(), "relation_kind": relationKind}, uuid.New()); err != nil {
		return TraceLink{}, fmt.Errorf("запис аудиторської події зв'язку: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return TraceLink{}, fmt.Errorf("фіксація зв'язку простежуваності: %w", err)
	}
	return link, nil
}

func validateTraceEndpoint(ctx context.Context, tx pgx.Tx, projectID, workProductID uuid.UUID, revisionID *uuid.UUID) error {
	var actualProjectID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT project_id FROM core.work_products WHERE id = $1`, workProductID).Scan(&actualProjectID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrTraceEndpointInvalid
		}
		return fmt.Errorf("перевірка кінцевої точки зв'язку: %w", err)
	}
	if actualProjectID != projectID {
		return ErrTraceEndpointInvalid
	}
	if revisionID == nil {
		return nil
	}
	var revisionWorkProductID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT work_product_id FROM core.work_product_revisions WHERE id = $1`, *revisionID).Scan(&revisionWorkProductID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrTraceEndpointInvalid
		}
		return fmt.Errorf("перевірка ревізії кінцевої точки: %w", err)
	}
	if revisionWorkProductID != workProductID {
		return ErrTraceEndpointInvalid
	}
	return nil
}

func (s *Store) TraverseTraceability(ctx context.Context, projectID, sourceID uuid.UUID) ([]TracePathEntry, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM core.work_products WHERE id = $1 AND project_id = $2)`, sourceID, projectID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("перевірка кореня графа: %w", err)
	}
	if !exists {
		return nil, ErrTraceEndpointInvalid
	}

	rows, err := s.pool.Query(ctx, `
		WITH RECURSIVE trace_tree AS (
			SELECT tl.source_id, tl.target_id, tl.source_revision_id, tl.target_revision_id,
			       tl.relation_kind, tl.is_suspect, 1 AS depth, ARRAY[tl.source_id, tl.target_id] AS path,
			       false AS is_cycle
			FROM core.trace_links tl
			WHERE tl.project_id = $1 AND tl.source_id = $2
			UNION ALL
			SELECT child.source_id, child.target_id, child.source_revision_id, child.target_revision_id,
			       child.relation_kind, child.is_suspect, parent.depth + 1,
			       parent.path || child.target_id, child.target_id = ANY(parent.path)
			FROM core.trace_links child
			JOIN trace_tree parent ON child.source_id = parent.target_id
			WHERE child.project_id = $1 AND parent.depth < 100 AND NOT parent.is_cycle
		)
		SELECT tt.depth, tt.source_id, source_wp.code, tt.target_id, target_wp.code, target_wp.type,
		       tt.source_revision_id, tt.target_revision_id, tt.relation_kind, tt.is_suspect, tt.is_cycle
		FROM trace_tree tt
		JOIN core.work_products source_wp ON source_wp.id = tt.source_id
		JOIN core.work_products target_wp ON target_wp.id = tt.target_id
		WHERE source_wp.project_id = $1 AND target_wp.project_id = $1
		ORDER BY tt.depth, source_wp.code, target_wp.code`, projectID, sourceID)
	if err != nil {
		return nil, fmt.Errorf("обхід графа простежуваності: %w", err)
	}
	defer rows.Close()

	var entries []TracePathEntry
	for rows.Next() {
		var entry TracePathEntry
		if err := rows.Scan(&entry.Depth, &entry.SourceID, &entry.SourceCode, &entry.TargetID, &entry.TargetCode,
			&entry.TargetType, &entry.SourceRevisionID, &entry.TargetRevisionID, &entry.RelationKind,
			&entry.IsSuspect, &entry.IsCycle); err != nil {
			return nil, fmt.Errorf("розбір графа простежуваності: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}
