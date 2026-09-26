package project

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrWorkProductNotFound    = errors.New("work product не знайдено")
	ErrInvalidWorkProductType = errors.New("невідомий тип work product")
	ErrProjectPlanReserved    = errors.New("тип plan зарезервовано для системного Generic Project Plan")
	ErrWorkProductCodeTaken   = errors.New("код work product вже використовується в цьому проєкті")
	ErrVersionConflict        = errors.New("work product змінено іншим запитом (конфлікт версії)")
	ErrWorkProductObsolete    = errors.New("work product виведено з експлуатації (obsolete)")
)

// coreWorkProductTypes — шість базових типів артефактів ядра (WORK_PRODUCTS.md §2).
var coreWorkProductTypes = map[string]bool{
	"plan": true, "requirement": true, "architecture": true,
	"test_spec": true, "report": true, "record": true,
}

// CreateWorkProduct створює чернетку базового типу з першою незмінною ревізією
// (wp.create). Композитні специфікації та профілі модулів — поза межами v0.4.0.
func (s *Store) CreateWorkProduct(ctx context.Context, actorID, projectID uuid.UUID, code, wpType, title, body string, metadata map[string]any) (WorkProduct, WorkProductRevision, error) {
	if wpType == planType {
		return WorkProduct{}, WorkProductRevision{}, ErrProjectPlanReserved
	}
	if !coreWorkProductTypes[wpType] {
		return WorkProduct{}, WorkProductRevision{}, ErrInvalidWorkProductType
	}
	if code == "" || title == "" {
		return WorkProduct{}, WorkProductRevision{}, ErrInvalidCode
	}
	if metadata == nil {
		metadata = map[string]any{}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return WorkProduct{}, WorkProductRevision{}, fmt.Errorf("початок транзакції створення work product: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	profile := "core:" + wpType
	var wp WorkProduct
	err = tx.QueryRow(ctx,
		`INSERT INTO core.work_products (project_id, code, type, profile, title)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id, project_id, code, type, profile, title, status, classification, row_version`,
		projectID, code, wpType, profile, title,
	).Scan(&wp.ID, &wp.ProjectID, &wp.Code, &wp.Type, &wp.Profile, &wp.Title, &wp.Status, &wp.Classification, &wp.RowVersion)
	if err != nil {
		if isUniqueViolation(err) {
			return WorkProduct{}, WorkProductRevision{}, ErrWorkProductCodeTaken
		}
		return WorkProduct{}, WorkProductRevision{}, fmt.Errorf("створення work product: %w", err)
	}

	payloadHash, contentHash := canonicalHashes(code, wpType, profile, title, body, metadata)

	var revision WorkProductRevision
	err = tx.QueryRow(ctx,
		`INSERT INTO core.work_product_revisions
		    (work_product_id, revision_number, body, metadata, payload_hash, content_hash, created_by)
		 VALUES ($1, 1, $2, $3, $4, $5, $6)
		 RETURNING id, work_product_id, revision_number, body, metadata, payload_hash, content_hash, created_by, created_at`,
		wp.ID, body, metadata, payloadHash, contentHash, actorID,
	).Scan(&revision.ID, &revision.WorkProductID, &revision.RevisionNumber, &revision.Body, &revision.Metadata,
		&revision.PayloadHash, &revision.ContentHash, &revision.CreatedBy, &revision.CreatedAt)
	if err != nil {
		return WorkProduct{}, WorkProductRevision{}, fmt.Errorf("створення першої ревізії work product: %w", err)
	}

	correlationID := uuid.New()
	_, err = tx.Exec(ctx,
		`INSERT INTO core.audit_events (actor_user_id, action, scope_type, scope_id, outcome, detail, correlation_id)
		 VALUES ($1, 'wp.create', 'project', $2, 'success', $3, $4)`,
		actorID, projectID, map[string]any{"work_product_id": wp.ID.String(), "code": code}, correlationID)
	if err != nil {
		return WorkProduct{}, WorkProductRevision{}, fmt.Errorf("запис аудиторської події створення work product: %w", err)
	}

	if err := upsertEmbedding(ctx, tx, wpType, wp.ID, revision.ID, title, body); err != nil {
		return WorkProduct{}, WorkProductRevision{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return WorkProduct{}, WorkProductRevision{}, fmt.Errorf("фіксація створення work product: %w", err)
	}

	return wp, revision, nil
}

// ReviseWorkProduct додає нову незмінну ревізію (wp.edit) з оптимістичною
// перевіркою row_version; попередні ревізії залишаються незмінними.
func (s *Store) ReviseWorkProduct(ctx context.Context, actorID, projectID, workProductID uuid.UUID, expectedRowVersion int64, body string, metadata map[string]any) (WorkProductRevision, error) {
	if metadata == nil {
		metadata = map[string]any{}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return WorkProductRevision{}, fmt.Errorf("початок транзакції ревізії: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var wp WorkProduct
	err = tx.QueryRow(ctx,
		`SELECT id, project_id, code, type, profile, title, status, classification, row_version
		 FROM core.work_products WHERE id = $1 AND project_id = $2 FOR UPDATE`,
		workProductID, projectID,
	).Scan(&wp.ID, &wp.ProjectID, &wp.Code, &wp.Type, &wp.Profile, &wp.Title, &wp.Status, &wp.Classification, &wp.RowVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkProductRevision{}, ErrWorkProductNotFound
	}
	if err != nil {
		return WorkProductRevision{}, fmt.Errorf("читання work product для ревізії: %w", err)
	}
	if wp.Status == "obsolete" {
		return WorkProductRevision{}, ErrWorkProductObsolete
	}
	if wp.RowVersion != expectedRowVersion {
		return WorkProductRevision{}, ErrVersionConflict
	}

	var lastRevisionNumber int
	err = tx.QueryRow(ctx,
		`SELECT max(revision_number) FROM core.work_product_revisions WHERE work_product_id = $1`, workProductID,
	).Scan(&lastRevisionNumber)
	if err != nil {
		return WorkProductRevision{}, fmt.Errorf("читання останньої ревізії: %w", err)
	}

	payloadHash, contentHash := canonicalHashes(wp.Code, wp.Type, wp.Profile, wp.Title, body, metadata)

	var revision WorkProductRevision
	err = tx.QueryRow(ctx,
		`INSERT INTO core.work_product_revisions
		    (work_product_id, revision_number, body, metadata, payload_hash, content_hash, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, work_product_id, revision_number, body, metadata, payload_hash, content_hash, created_by, created_at`,
		workProductID, lastRevisionNumber+1, body, metadata, payloadHash, contentHash, actorID,
	).Scan(&revision.ID, &revision.WorkProductID, &revision.RevisionNumber, &revision.Body, &revision.Metadata,
		&revision.PayloadHash, &revision.ContentHash, &revision.CreatedBy, &revision.CreatedAt)
	if err != nil {
		return WorkProductRevision{}, fmt.Errorf("створення ревізії: %w", err)
	}

	tag, err := tx.Exec(ctx,
		`UPDATE core.work_products SET row_version = row_version + 1 WHERE id = $1 AND row_version = $2`,
		workProductID, expectedRowVersion)
	if err != nil {
		return WorkProductRevision{}, fmt.Errorf("оновлення row_version work product: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return WorkProductRevision{}, ErrVersionConflict
	}

	// GRP-03 Change Impact Analysis: нова ревізія позначає підозрілими всі
	// ребра графа, що прямо чи опосередковано спираються на цей артефакт.
	suspectCount, err := propagateSuspect(ctx, tx, projectID, workProductID)
	if err != nil {
		return WorkProductRevision{}, err
	}

	correlationID := uuid.New()
	_, err = tx.Exec(ctx,
		`INSERT INTO core.audit_events (actor_user_id, action, scope_type, scope_id, outcome, detail, correlation_id)
		 VALUES ($1, 'wp.revise', 'project', $2, 'success', $3, $4)`,
		actorID, projectID, map[string]any{"work_product_id": workProductID.String(), "revision_number": revision.RevisionNumber, "suspect_links_marked": suspectCount}, correlationID)
	if err != nil {
		return WorkProductRevision{}, fmt.Errorf("запис аудиторської події ревізії: %w", err)
	}

	if err := upsertEmbedding(ctx, tx, wp.Type, workProductID, revision.ID, wp.Title, body); err != nil {
		return WorkProductRevision{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return WorkProductRevision{}, fmt.Errorf("фіксація ревізії: %w", err)
	}

	return revision, nil
}

// RetireWorkProduct переводить чернетку в obsolete (wp.retire, WORK_PRODUCTS.md §4:
// Draft -> Obsolete). Уже виведений з експлуатації WP — безпечний no-op, не помилка.
func (s *Store) RetireWorkProduct(ctx context.Context, actorID, projectID, workProductID uuid.UUID, expectedRowVersion int64) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE core.work_products SET status = 'obsolete', row_version = row_version + 1
		 WHERE id = $1 AND project_id = $2 AND row_version = $3 AND status != 'obsolete'`,
		workProductID, projectID, expectedRowVersion)
	if err != nil {
		return fmt.Errorf("виведення work product з експлуатації: %w", err)
	}
	if tag.RowsAffected() == 0 {
		var currentVersion int64
		var status string
		checkErr := s.pool.QueryRow(ctx,
			`SELECT row_version, status FROM core.work_products WHERE id = $1 AND project_id = $2`,
			workProductID, projectID).Scan(&currentVersion, &status)
		if errors.Is(checkErr, pgx.ErrNoRows) {
			return ErrWorkProductNotFound
		}
		if checkErr != nil {
			return fmt.Errorf("перевірка стану work product: %w", checkErr)
		}
		if status == "obsolete" {
			return nil // вже виведено з експлуатації — безпечний no-op
		}
		return ErrVersionConflict
	}

	correlationID := uuid.New()
	if err := s.recordProjectAudit(ctx, actorID, projectID, "wp.retire", correlationID,
		map[string]any{"work_product_id": workProductID.String()}); err != nil {
		return err
	}

	return nil
}

// WorkProductDetail — WP разом з останньою ревізією.
type WorkProductDetail struct {
	WorkProduct WorkProduct
	Latest      WorkProductRevision
}

// GetWorkProduct повертає WP лише в межах вказаного проєкту (захист від
// вгадування чужого work_product_id через інший проєкт).
func (s *Store) GetWorkProduct(ctx context.Context, projectID, workProductID uuid.UUID) (WorkProductDetail, error) {
	var detail WorkProductDetail

	err := s.pool.QueryRow(ctx,
		`SELECT wp.id, wp.project_id, wp.code, wp.type, wp.profile, wp.title, wp.status, wp.classification, wp.row_version,
		        r.id, r.revision_number, r.body, r.metadata, r.payload_hash, r.content_hash, r.created_by, r.created_at
		 FROM core.work_products wp
		 JOIN core.work_product_revisions r ON r.work_product_id = wp.id
		 WHERE wp.id = $1 AND wp.project_id = $2
		 ORDER BY r.revision_number DESC LIMIT 1`,
		workProductID, projectID,
	).Scan(&detail.WorkProduct.ID, &detail.WorkProduct.ProjectID, &detail.WorkProduct.Code, &detail.WorkProduct.Type,
		&detail.WorkProduct.Profile, &detail.WorkProduct.Title, &detail.WorkProduct.Status, &detail.WorkProduct.Classification,
		&detail.WorkProduct.RowVersion, &detail.Latest.ID, &detail.Latest.RevisionNumber, &detail.Latest.Body,
		&detail.Latest.Metadata, &detail.Latest.PayloadHash, &detail.Latest.ContentHash, &detail.Latest.CreatedBy, &detail.Latest.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkProductDetail{}, ErrWorkProductNotFound
	}
	if err != nil {
		return WorkProductDetail{}, fmt.Errorf("читання work product: %w", err)
	}
	detail.Latest.WorkProductID = workProductID

	return detail, nil
}

// WorkProductSummary — рядок списку WP проєкту (без тіла останньої ревізії).
type WorkProductSummary struct {
	ID     uuid.UUID
	Code   string
	Type   string
	Title  string
	Status string
}

func (s *Store) ListWorkProducts(ctx context.Context, projectID uuid.UUID) ([]WorkProductSummary, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, code, type, title, status FROM core.work_products WHERE project_id = $1 ORDER BY code`,
		projectID)
	if err != nil {
		return nil, fmt.Errorf("читання списку work products: %w", err)
	}
	defer rows.Close()

	var summaries []WorkProductSummary
	for rows.Next() {
		var summary WorkProductSummary
		if err := rows.Scan(&summary.ID, &summary.Code, &summary.Type, &summary.Title, &summary.Status); err != nil {
			return nil, fmt.Errorf("розбір рядка списку work products: %w", err)
		}
		summaries = append(summaries, summary)
	}

	return summaries, rows.Err()
}

func (s *Store) recordProjectAudit(ctx context.Context, actorID, projectID uuid.UUID, action string, correlationID uuid.UUID, detail map[string]any) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO core.audit_events (actor_user_id, action, scope_type, scope_id, outcome, detail, correlation_id)
		 VALUES ($1, $2, 'project', $3, 'success', $4, $5)`,
		actorID, action, projectID, detail, correlationID)
	if err != nil {
		return fmt.Errorf("запис аудиторської події %s: %w", action, err)
	}

	return nil
}

func (s *Store) recordProjectAuditTx(ctx context.Context, tx pgx.Tx, actorID, projectID uuid.UUID, action string, detail map[string]any) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO core.audit_events (actor_user_id, action, scope_type, scope_id, outcome, detail, correlation_id)
		 VALUES ($1, $2, 'project', $3, 'success', $4, $5)`,
		actorID, action, projectID, detail, uuid.New())
	if err != nil {
		return fmt.Errorf("запис аудиторської події %s: %w", action, err)
	}

	return nil
}
