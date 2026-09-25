package project

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrSpecificationNotFound    = errors.New("специфікацію не знайдено")
	ErrCompositionTargetInvalid = errors.New("цільова ревізія композиції недійсна")
)

type Specification struct {
	ID            uuid.UUID
	WorkProductID uuid.UUID
	RevisionID    uuid.UUID
	Manifest      CompositionManifest
	ElementsHash  []byte
}

func (s *Store) GetSpecification(ctx context.Context, projectID, specificationID uuid.UUID) (WorkProduct, WorkProductRevision, Specification, error) {
	var workProduct WorkProduct
	var revision WorkProductRevision
	var specification Specification
	var manifestJSON []byte

	err := s.pool.QueryRow(ctx,
		`SELECT s.id, s.work_product_id, s.revision_id, s.manifest, s.elements_hash,
		        wp.id, wp.project_id, wp.code, wp.type, wp.profile, wp.title, wp.status, wp.classification, wp.row_version,
		        r.id, r.work_product_id, r.revision_number, r.body, r.metadata, r.payload_hash, r.content_hash, r.created_by, r.created_at
		 FROM core.specifications s
		 JOIN core.work_products wp ON wp.id = s.work_product_id
		 JOIN core.work_product_revisions r ON r.id = s.revision_id
		 WHERE s.id = $1 AND wp.project_id = $2`,
		specificationID, projectID,
	).Scan(
		&specification.ID, &specification.WorkProductID, &specification.RevisionID, &manifestJSON, &specification.ElementsHash,
		&workProduct.ID, &workProduct.ProjectID, &workProduct.Code, &workProduct.Type, &workProduct.Profile,
		&workProduct.Title, &workProduct.Status, &workProduct.Classification, &workProduct.RowVersion,
		&revision.ID, &revision.WorkProductID, &revision.RevisionNumber, &revision.Body, &revision.Metadata,
		&revision.PayloadHash, &revision.ContentHash, &revision.CreatedBy, &revision.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkProduct{}, WorkProductRevision{}, Specification{}, ErrSpecificationNotFound
	}
	if err != nil {
		return WorkProduct{}, WorkProductRevision{}, Specification{}, fmt.Errorf("читання специфікації: %w", err)
	}
	if err := json.Unmarshal(manifestJSON, &specification.Manifest); err != nil {
		return WorkProduct{}, WorkProductRevision{}, Specification{}, fmt.Errorf("розбір маніфесту специфікації: %w", err)
	}
	return workProduct, revision, specification, nil
}

func (s *Store) CreateSpecification(ctx context.Context, actorID, projectID uuid.UUID, code, title, body string, metadata map[string]any, manifest CompositionManifest) (WorkProduct, WorkProductRevision, Specification, error) {
	if code == "" || title == "" {
		return WorkProduct{}, WorkProductRevision{}, Specification{}, ErrInvalidCode
	}
	if err := manifest.Validate(); err != nil {
		return WorkProduct{}, WorkProductRevision{}, Specification{}, err
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return WorkProduct{}, WorkProductRevision{}, Specification{}, fmt.Errorf("серіалізація маніфесту: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return WorkProduct{}, WorkProductRevision{}, Specification{}, fmt.Errorf("початок транзакції специфікації: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, occurrence := range manifest.Occurrences {
		var targetProjectID uuid.UUID
		var payloadHash []byte
		err := tx.QueryRow(ctx,
			`SELECT wp.project_id, r.payload_hash
			 FROM core.work_products wp
			 JOIN core.work_product_revisions r ON r.work_product_id = wp.id
			 WHERE wp.id = $1 AND r.id = $2`,
			occurrence.TargetWorkProductID, occurrence.TargetRevisionID,
		).Scan(&targetProjectID, &payloadHash)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return WorkProduct{}, WorkProductRevision{}, Specification{}, ErrCompositionTargetInvalid
			}
			return WorkProduct{}, WorkProductRevision{}, Specification{}, fmt.Errorf("перевірка елемента специфікації: %w", err)
		}
		if targetProjectID != projectID || hex.EncodeToString(payloadHash) != occurrence.TargetPayloadHash {
			return WorkProduct{}, WorkProductRevision{}, Specification{}, ErrCompositionTargetInvalid
		}
	}

	var workProduct WorkProduct
	err = tx.QueryRow(ctx,
		`INSERT INTO core.work_products (project_id, code, type, profile, title)
		 VALUES ($1, $2, 'requirement', 'core:specification', $3)
		 RETURNING id, project_id, code, type, profile, title, status, classification, row_version`,
		projectID, code, title,
	).Scan(&workProduct.ID, &workProduct.ProjectID, &workProduct.Code, &workProduct.Type,
		&workProduct.Profile, &workProduct.Title, &workProduct.Status, &workProduct.Classification, &workProduct.RowVersion)
	if err != nil {
		if isUniqueViolation(err) {
			return WorkProduct{}, WorkProductRevision{}, Specification{}, ErrWorkProductCodeTaken
		}
		return WorkProduct{}, WorkProductRevision{}, Specification{}, fmt.Errorf("створення work product специфікації: %w", err)
	}

	payloadHash, contentHash := canonicalHashes(code, "requirement", "core:specification", title, body, metadata)
	var revision WorkProductRevision
	err = tx.QueryRow(ctx,
		`INSERT INTO core.work_product_revisions
		    (work_product_id, revision_number, body, metadata, payload_hash, content_hash, created_by)
		 VALUES ($1, 1, $2, $3, $4, $5, $6)
		 RETURNING id, work_product_id, revision_number, body, metadata, payload_hash, content_hash, created_by, created_at`,
		workProduct.ID, body, metadata, payloadHash, contentHash, actorID,
	).Scan(&revision.ID, &revision.WorkProductID, &revision.RevisionNumber, &revision.Body, &revision.Metadata,
		&revision.PayloadHash, &revision.ContentHash, &revision.CreatedBy, &revision.CreatedAt)
	if err != nil {
		return WorkProduct{}, WorkProductRevision{}, Specification{}, fmt.Errorf("створення ревізії специфікації: %w", err)
	}

	var specification Specification
	err = tx.QueryRow(ctx,
		`INSERT INTO core.specifications (work_product_id, revision_id, manifest, elements_hash)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		workProduct.ID, revision.ID, manifestJSON, manifest.ElementsHash,
	).Scan(&specification.ID)
	if err != nil {
		return WorkProduct{}, WorkProductRevision{}, Specification{}, fmt.Errorf("збереження маніфесту специфікації: %w", err)
	}

	for _, occurrence := range manifest.Occurrences {
		if _, err := tx.Exec(ctx,
			`INSERT INTO core.specification_occurrences
			    (specification_id, occurrence_id, section_id, target_work_product_id, target_revision_id, item_order)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			specification.ID, occurrence.OccurrenceID, occurrence.SectionID,
			occurrence.TargetWorkProductID, occurrence.TargetRevisionID, occurrence.Order,
		); err != nil {
			return WorkProduct{}, WorkProductRevision{}, Specification{}, fmt.Errorf("збереження входження специфікації: %w", err)
		}
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO core.audit_events (actor_user_id, action, scope_type, scope_id, outcome, detail, correlation_id)
		 VALUES ($1, 'specification.create', 'project', $2, 'success', $3, $4)`,
		actorID, projectID, map[string]any{"work_product_id": workProduct.ID.String(), "code": code}, uuid.New(),
	); err != nil {
		return WorkProduct{}, WorkProductRevision{}, Specification{}, fmt.Errorf("запис аудиторської події створення специфікації: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return WorkProduct{}, WorkProductRevision{}, Specification{}, fmt.Errorf("фіксація специфікації: %w", err)
	}
	specification.WorkProductID = workProduct.ID
	specification.RevisionID = revision.ID
	specification.Manifest = manifest
	specification.ElementsHash = manifest.ElementsHash
	return workProduct, revision, specification, nil
}

func (s *Store) ReviseSpecification(ctx context.Context, actorID, projectID, specificationID uuid.UUID, expectedRowVersion int64, body string, metadata map[string]any, manifest CompositionManifest) (WorkProduct, WorkProductRevision, Specification, error) {
	if err := manifest.Validate(); err != nil {
		return WorkProduct{}, WorkProductRevision{}, Specification{}, err
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return WorkProduct{}, WorkProductRevision{}, Specification{}, fmt.Errorf("серіалізація маніфесту: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return WorkProduct{}, WorkProductRevision{}, Specification{}, fmt.Errorf("початок транзакції ревізії специфікації: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var workProduct WorkProduct
	var currentSpecificationID uuid.UUID
	err = tx.QueryRow(ctx,
		`SELECT s.id, wp.id, wp.project_id, wp.code, wp.type, wp.profile, wp.title, wp.status, wp.classification, wp.row_version
		 FROM core.specifications s
		 JOIN core.work_products wp ON wp.id = s.work_product_id
		 WHERE s.id = $1 AND wp.project_id = $2
		 ORDER BY s.created_at DESC LIMIT 1 FOR UPDATE OF wp`,
		specificationID, projectID,
	).Scan(&currentSpecificationID, &workProduct.ID, &workProduct.ProjectID, &workProduct.Code, &workProduct.Type,
		&workProduct.Profile, &workProduct.Title, &workProduct.Status, &workProduct.Classification, &workProduct.RowVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkProduct{}, WorkProductRevision{}, Specification{}, ErrSpecificationNotFound
	}
	if err != nil {
		return WorkProduct{}, WorkProductRevision{}, Specification{}, fmt.Errorf("читання специфікації для ревізії: %w", err)
	}
	if workProduct.Status == "obsolete" {
		return WorkProduct{}, WorkProductRevision{}, Specification{}, ErrWorkProductObsolete
	}
	if workProduct.RowVersion != expectedRowVersion {
		return WorkProduct{}, WorkProductRevision{}, Specification{}, ErrVersionConflict
	}

	for _, occurrence := range manifest.Occurrences {
		var targetProjectID uuid.UUID
		var payloadHash []byte
		err := tx.QueryRow(ctx,
			`SELECT wp.project_id, r.payload_hash
			 FROM core.work_products wp
			 JOIN core.work_product_revisions r ON r.work_product_id = wp.id
			 WHERE wp.id = $1 AND r.id = $2`, occurrence.TargetWorkProductID, occurrence.TargetRevisionID,
		).Scan(&targetProjectID, &payloadHash)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return WorkProduct{}, WorkProductRevision{}, Specification{}, ErrCompositionTargetInvalid
			}
			return WorkProduct{}, WorkProductRevision{}, Specification{}, fmt.Errorf("перевірка елемента ревізії специфікації: %w", err)
		}
		if targetProjectID != projectID || hex.EncodeToString(payloadHash) != occurrence.TargetPayloadHash {
			return WorkProduct{}, WorkProductRevision{}, Specification{}, ErrCompositionTargetInvalid
		}
	}

	var lastRevisionNumber int
	if err := tx.QueryRow(ctx, `SELECT max(revision_number) FROM core.work_product_revisions WHERE work_product_id = $1`, workProduct.ID).Scan(&lastRevisionNumber); err != nil {
		return WorkProduct{}, WorkProductRevision{}, Specification{}, fmt.Errorf("читання останньої ревізії специфікації: %w", err)
	}
	payloadHash, contentHash := canonicalHashes(workProduct.Code, workProduct.Type, workProduct.Profile, workProduct.Title, body, metadata)
	var revision WorkProductRevision
	err = tx.QueryRow(ctx,
		`INSERT INTO core.work_product_revisions
		    (work_product_id, revision_number, body, metadata, payload_hash, content_hash, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, work_product_id, revision_number, body, metadata, payload_hash, content_hash, created_by, created_at`,
		workProduct.ID, lastRevisionNumber+1, body, metadata, payloadHash, contentHash, actorID,
	).Scan(&revision.ID, &revision.WorkProductID, &revision.RevisionNumber, &revision.Body, &revision.Metadata,
		&revision.PayloadHash, &revision.ContentHash, &revision.CreatedBy, &revision.CreatedAt)
	if err != nil {
		return WorkProduct{}, WorkProductRevision{}, Specification{}, fmt.Errorf("створення ревізії специфікації: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE core.work_products SET row_version = row_version + 1 WHERE id = $1 AND row_version = $2`, workProduct.ID, expectedRowVersion); err != nil {
		return WorkProduct{}, WorkProductRevision{}, Specification{}, fmt.Errorf("оновлення версії специфікації: %w", err)
	}
	workProduct.RowVersion++

	var specification Specification
	err = tx.QueryRow(ctx,
		`INSERT INTO core.specifications (work_product_id, revision_id, manifest, elements_hash)
		 VALUES ($1, $2, $3, $4) RETURNING id`, workProduct.ID, revision.ID, manifestJSON, manifest.ElementsHash,
	).Scan(&specification.ID)
	if err != nil {
		return WorkProduct{}, WorkProductRevision{}, Specification{}, fmt.Errorf("збереження ревізії маніфесту: %w", err)
	}
	for _, occurrence := range manifest.Occurrences {
		if _, err := tx.Exec(ctx,
			`INSERT INTO core.specification_occurrences
			    (specification_id, occurrence_id, section_id, target_work_product_id, target_revision_id, item_order)
			 VALUES ($1, $2, $3, $4, $5, $6)`, specification.ID, occurrence.OccurrenceID, occurrence.SectionID,
			occurrence.TargetWorkProductID, occurrence.TargetRevisionID, occurrence.Order); err != nil {
			return WorkProduct{}, WorkProductRevision{}, Specification{}, fmt.Errorf("збереження входження ревізії: %w", err)
		}
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO core.audit_events (actor_user_id, action, scope_type, scope_id, outcome, detail, correlation_id)
		 VALUES ($1, 'specification.revise', 'project', $2, 'success', $3, $4)`, actorID, projectID,
		map[string]any{"work_product_id": workProduct.ID.String(), "previous_specification_id": currentSpecificationID.String(), "revision_number": revision.RevisionNumber}, uuid.New()); err != nil {
		return WorkProduct{}, WorkProductRevision{}, Specification{}, fmt.Errorf("запис аудиторської події ревізії специфікації: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return WorkProduct{}, WorkProductRevision{}, Specification{}, fmt.Errorf("фіксація ревізії специфікації: %w", err)
	}
	specification.WorkProductID = workProduct.ID
	specification.RevisionID = revision.ID
	specification.Manifest = manifest
	specification.ElementsHash = manifest.ElementsHash
	return workProduct, revision, specification, nil
}
