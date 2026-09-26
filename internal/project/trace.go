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

// propagateSuspectOnRevision позначає is_suspect=true на всіх ребрах графа, що
// прямо чи опосередковано посилаються на workProductID як на ціль (target) —
// саме так у trace_links кодується залежність "джерело спирається на ціль"
// (verifies/satisfies/refines/derives_from). Виконується в тій самій
// транзакції, що й фіксація нової ревізії (GRP-03, Change Impact Analysis):
// зміна артефакту не робить залежні артефакти невалідними автоматично, лише
// формує список для перегляду інженером; зняття прапорця — окрема явна дія
// рецензента (AcknowledgeTraceLink).
func propagateSuspect(ctx context.Context, tx pgx.Tx, projectID, workProductID uuid.UUID) (int, error) {
	tag, err := tx.Exec(ctx, `
		WITH RECURSIVE impact AS (
			SELECT tl.id, tl.source_id, ARRAY[tl.target_id] AS path, false AS is_cycle
			FROM core.trace_links tl
			WHERE tl.project_id = $1 AND tl.target_id = $2
			UNION ALL
			SELECT child.id, child.source_id, parent.path || child.target_id,
			       child.target_id = ANY(parent.path)
			FROM core.trace_links child
			JOIN impact parent ON child.target_id = parent.source_id
			WHERE child.project_id = $1 AND array_length(parent.path, 1) < 100 AND NOT parent.is_cycle
		)
		UPDATE core.trace_links
		SET is_suspect = true
		WHERE project_id = $1 AND id IN (SELECT id FROM impact)`,
		projectID, workProductID)
	if err != nil {
		return 0, fmt.Errorf("поширення прапорця is_suspect: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// ListTraceLinks повертає всі зв'язки простежуваності проєкту (найновіші
// перші), опційно лише підозрілі — аудиторський список перегляду з GRP-03.
func (s *Store) ListTraceLinks(ctx context.Context, projectID uuid.UUID, onlySuspect bool) ([]TraceLink, error) {
	query := `SELECT id, project_id, source_id, source_revision_id, target_id, target_revision_id, relation_kind, is_suspect
	          FROM core.trace_links WHERE project_id = $1`
	if onlySuspect {
		query += ` AND is_suspect`
	}
	query += ` ORDER BY created_at DESC`

	rows, err := s.pool.Query(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("читання зв'язків простежуваності: %w", err)
	}
	defer rows.Close()

	var links []TraceLink
	for rows.Next() {
		var link TraceLink
		if err := rows.Scan(&link.ID, &link.ProjectID, &link.SourceID, &link.SourceRevisionID,
			&link.TargetID, &link.TargetRevisionID, &link.RelationKind, &link.IsSuspect); err != nil {
			return nil, fmt.Errorf("розбір зв'язку простежуваності: %w", err)
		}
		links = append(links, link)
	}
	return links, rows.Err()
}

// AcknowledgeTraceLink знімає прапорець is_suspect за явною дією рецензента
// після підтвердження відповідності (VECTOR_AND_GRAPH_DATA.md §3.3).
func (s *Store) AcknowledgeTraceLink(ctx context.Context, actorID, projectID, linkID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE core.trace_links SET is_suspect = false WHERE id = $1 AND project_id = $2 AND is_suspect`,
		linkID, projectID)
	if err != nil {
		return fmt.Errorf("зняття прапорця is_suspect: %w", err)
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM core.trace_links WHERE id = $1 AND project_id = $2)`,
			linkID, projectID).Scan(&exists); err != nil {
			return fmt.Errorf("перевірка існування зв'язку: %w", err)
		}
		if !exists {
			return ErrTraceLinkNotFound
		}
		return nil // вже не підозрілий — безпечний no-op
	}
	_, err = s.pool.Exec(ctx,
		`INSERT INTO core.audit_events (actor_user_id, action, scope_type, scope_id, outcome, detail, correlation_id)
		 VALUES ($1, 'trace.acknowledge', 'project', $2, 'success', $3, $4)`,
		actorID, projectID, map[string]any{"trace_link_id": linkID.String()}, uuid.New())
	if err != nil {
		return fmt.Errorf("запис аудиторської події зняття підозрілості: %w", err)
	}
	return nil
}
