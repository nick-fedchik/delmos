package project

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"delmos/internal/automation"
)

var (
	ErrReviewRequestOpen    = errors.New("артефакт уже подано на рецензію")
	ErrNotDraft             = errors.New("подати на рецензію можна лише чернетку")
	ErrNoRevision           = errors.New("артефакт не має жодної ревізії")
	ErrAssigneeIsAuthor     = errors.New("автор ревізії не може бути рецензентом або погоджувачем")
	ErrReviewRequestMissing = errors.New("відкритого запиту на рецензію не знайдено")
)

type ReviewRequest struct {
	ID            uuid.UUID `json:"id"`
	ProjectID     uuid.UUID `json:"project_id"`
	WorkProductID uuid.UUID `json:"work_product_id"`
	RevisionID    uuid.UUID `json:"revision_id"`
	Status        string    `json:"status"`
	RequestedBy   uuid.UUID `json:"requested_by"`
	CreatedAt     time.Time `json:"created_at"`
}

// ReviewAssignment — призначення особи на конкретний запит. Наявність дозволу
// без призначення недостатня (ADR-009 §1): право дає змогу діяти, призначення
// визначає, над чим саме.
type ReviewAssignment struct {
	UserID uuid.UUID `json:"user_id"`
	Role   string    `json:"assignment_role"`
}

// SubmitWorkProduct подає останню ревізію артефакту на рецензію.
//
// Запит прив'язується до точної пари (revision_id, payload_hash) — ADR-009 §1.
// Призначені особи перевіряються на незалежність від автора ревізії тут-таки,
// щоб самопризначення не дійшло до стадії рішення.
func (s *Store) SubmitWorkProduct(ctx context.Context, actorID, projectID, workProductID uuid.UUID, assignments []ReviewAssignment) (ReviewRequest, error) {
	if len(assignments) == 0 {
		return ReviewRequest{}, fmt.Errorf("%w: потрібні призначення рецензента й погоджувача", ErrAssigneeIsAuthor)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ReviewRequest{}, fmt.Errorf("відкриття транзакції подання: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status string
	err = tx.QueryRow(ctx,
		`SELECT status FROM core.work_products
		 WHERE id = $1 AND project_id = $2 FOR UPDATE`, workProductID, projectID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ReviewRequest{}, ErrWorkProductNotFound
	}
	if err != nil {
		return ReviewRequest{}, fmt.Errorf("читання артефакту: %w", err)
	}
	if status != "draft" {
		return ReviewRequest{}, fmt.Errorf("%w: поточний статус %q", ErrNotDraft, status)
	}

	var revisionID uuid.UUID
	var payloadHash []byte
	var revisionAuthor uuid.UUID
	err = tx.QueryRow(ctx,
		`SELECT id, payload_hash, created_by FROM core.work_product_revisions
		 WHERE work_product_id = $1 ORDER BY revision_number DESC LIMIT 1`, workProductID).
		Scan(&revisionID, &payloadHash, &revisionAuthor)
	if errors.Is(err, pgx.ErrNoRows) {
		return ReviewRequest{}, ErrNoRevision
	}
	if err != nil {
		return ReviewRequest{}, fmt.Errorf("читання останньої ревізії: %w", err)
	}

	for _, a := range assignments {
		if a.UserID == revisionAuthor {
			return ReviewRequest{}, fmt.Errorf("%w: %s", ErrAssigneeIsAuthor, a.Role)
		}
	}

	var request ReviewRequest
	err = tx.QueryRow(ctx,
		`INSERT INTO core.review_requests
		   (project_id, work_product_id, revision_id, payload_hash, status, requested_by)
		 VALUES ($1, $2, $3, $4, 'open', $5)
		 RETURNING id, project_id, work_product_id, revision_id, status, requested_by, created_at`,
		projectID, workProductID, revisionID, payloadHash, actorID).
		Scan(&request.ID, &request.ProjectID, &request.WorkProductID, &request.RevisionID,
			&request.Status, &request.RequestedBy, &request.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ReviewRequest{}, ErrReviewRequestOpen
		}
		return ReviewRequest{}, fmt.Errorf("створення запиту на рецензію: %w", err)
	}

	for _, a := range assignments {
		if _, err := tx.Exec(ctx,
			`INSERT INTO core.review_assignments (review_request_id, user_id, assignment_role, assigned_by)
			 VALUES ($1, $2, $3, $4)`, request.ID, a.UserID, a.Role, actorID); err != nil {
			return ReviewRequest{}, fmt.Errorf("призначення %s: %w", a.Role, err)
		}
	}

	if _, err := tx.Exec(ctx,
		`UPDATE core.work_products SET status = 'in_review', row_version = row_version + 1
		 WHERE id = $1`, workProductID); err != nil {
		return ReviewRequest{}, fmt.Errorf("перехід артефакту в in_review: %w", err)
	}

	correlationID := uuid.New()
	if _, err := tx.Exec(ctx,
		`INSERT INTO core.audit_events (actor_user_id, action, scope_type, scope_id, outcome, detail, correlation_id)
		 VALUES ($1, 'wp.submit', 'project', $2, 'success', $3, $4)`,
		actorID, projectID, map[string]any{
			"work_product_id":   workProductID.String(),
			"revision_id":       revisionID.String(),
			"review_request_id": request.ID.String(),
		}, correlationID); err != nil {
		return ReviewRequest{}, fmt.Errorf("аудит подання: %w", err)
	}

	if err := automation.EmitEvent(ctx, tx, "wp.submitted_for_review", &projectID, &actorID, correlationID, map[string]any{
		"work_product_id":   workProductID.String(),
		"revision_id":       revisionID.String(),
		"review_request_id": request.ID.String(),
	}); err != nil {
		return ReviewRequest{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return ReviewRequest{}, fmt.Errorf("фіксація подання: %w", err)
	}
	return request, nil
}

// supersedeOpenReviewRequest закриває відкритий запит при появі нової ревізії.
//
// ADR-009 §3: старі рішення ніколи не переносяться на новий хеш. Якби запит
// лишався відкритим, погоджувач підписав би ревізію, якої вже не бачив.
func supersedeOpenReviewRequest(ctx context.Context, tx pgx.Tx, workProductID uuid.UUID) error {
	_, err := tx.Exec(ctx,
		`UPDATE core.review_requests
		 SET status = 'superseded', closed_at = now()
		 WHERE work_product_id = $1 AND status = 'open'`, workProductID)
	if err != nil {
		return fmt.Errorf("скасування відкритого запиту на рецензію: %w", err)
	}
	return nil
}
