package project

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"delmos/internal/automation"
)

var (
	ErrNotInReview      = errors.New("артефакт не перебуває на рецензії")
	ErrNotAssigned      = errors.New("особу не призначено на цей запит у потрібній ролі")
	ErrSelfDecision     = errors.New("автор ревізії не може ухвалювати щодо неї рішення")
	ErrStaleRevision    = errors.New("ревізія змінилася після створення запиту")
	ErrNoPositiveReview = errors.New("погодження потребує попередньої позитивної рецензії")
	ErrReasonRequired   = errors.New("рішення «потрібні зміни» потребує причини")
)

const (
	DecisionReview         = "review"
	DecisionApproval       = "approval"
	DecisionRequestChanges = "request_changes"
)

type ReviewDecision struct {
	ID               uuid.UUID `json:"id"`
	ReviewRequestID  uuid.UUID `json:"review_request_id"`
	RevisionID       uuid.UUID `json:"revision_id"`
	DecisionKind     string    `json:"decision_kind"`
	Outcome          string    `json:"outcome"`
	DecidedBy        uuid.UUID `json:"decided_by"`
	Reason           string    `json:"reason,omitempty"`
	DecidedAt        time.Time `json:"decided_at"`
	WorkProductState string    `json:"work_product_status"`
}

// requiredAssignment визначає, яке призначення потрібне для роду рішення.
// «Потрібні зміни» може дати і рецензент, і погоджувач (ADR-009 §3).
var requiredAssignment = map[string][]string{
	DecisionReview:         {"reviewer"},
	DecisionApproval:       {"approver"},
	DecisionRequestChanges: {"reviewer", "approver"},
}

// RecordDecision фіксує рішення рецензента або погоджувача щодо точної
// ревізії (ADR-009 §2-4).
//
// Перевірки виконуються атомарно в одній транзакції саме в тому порядку, що
// й у ADR-009 §4: ідемпотентність -> стан -> призначення -> SoD -> актуальність
// хеша -> кворум. Порушення будь-якої з них відкочує все.
func (s *Store) RecordDecision(ctx context.Context, actorID, projectID, workProductID uuid.UUID, kind, reason, operationKey string) (ReviewDecision, error) {
	if kind == DecisionRequestChanges && reason == "" {
		return ReviewDecision{}, ErrReasonRequired
	}
	if _, ok := requiredAssignment[kind]; !ok {
		return ReviewDecision{}, fmt.Errorf("невідомий рід рішення %q", kind)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ReviewDecision{}, fmt.Errorf("відкриття транзакції рішення: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Повтор із тим самим ключем операції повертає попередній результат, а не
	// другий підпис (ADR-009 §4).
	if operationKey != "" {
		existing, found, err := loadDecisionByOperationKey(ctx, tx, operationKey)
		if err != nil {
			return ReviewDecision{}, err
		}
		if found {
			return existing, nil
		}
	}

	var wpStatus string
	err = tx.QueryRow(ctx,
		`SELECT status FROM core.work_products
		 WHERE id = $1 AND project_id = $2 FOR UPDATE`, workProductID, projectID).Scan(&wpStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return ReviewDecision{}, ErrWorkProductNotFound
	}
	if err != nil {
		return ReviewDecision{}, fmt.Errorf("читання артефакту: %w", err)
	}
	if wpStatus != "in_review" {
		return ReviewDecision{}, fmt.Errorf("%w: поточний статус %q", ErrNotInReview, wpStatus)
	}

	var requestID, revisionID uuid.UUID
	var requestHash []byte
	err = tx.QueryRow(ctx,
		`SELECT id, revision_id, payload_hash FROM core.review_requests
		 WHERE work_product_id = $1 AND status = 'open' FOR UPDATE`, workProductID).
		Scan(&requestID, &revisionID, &requestHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return ReviewDecision{}, ErrReviewRequestMissing
	}
	if err != nil {
		return ReviewDecision{}, fmt.Errorf("читання запиту на рецензію: %w", err)
	}

	if err := ensureAssigned(ctx, tx, requestID, actorID, requiredAssignment[kind]); err != nil {
		return ReviewDecision{}, err
	}

	// Актуальність хеша та незалежність від автора беруться з тієї самої
	// ревізії, на яку створено запит.
	var revisionAuthor uuid.UUID
	var currentHash []byte
	if err := tx.QueryRow(ctx,
		`SELECT created_by, payload_hash FROM core.work_product_revisions WHERE id = $1`, revisionID).
		Scan(&revisionAuthor, &currentHash); err != nil {
		return ReviewDecision{}, fmt.Errorf("читання ревізії запиту: %w", err)
	}
	if actorID == revisionAuthor {
		return ReviewDecision{}, ErrSelfDecision
	}
	if !bytes.Equal(requestHash, currentHash) {
		return ReviewDecision{}, ErrStaleRevision
	}

	// Кворум: погодження неможливе без попередньої позитивної рецензії
	// незалежної особи (ADR-009 §2).
	if kind == DecisionApproval {
		var positiveReviews int
		if err := tx.QueryRow(ctx,
			`SELECT count(*) FROM core.review_decisions
			 WHERE review_request_id = $1 AND decision_kind = 'review' AND outcome = 'positive'`,
			requestID).Scan(&positiveReviews); err != nil {
			return ReviewDecision{}, fmt.Errorf("перевірка кворуму: %w", err)
		}
		if positiveReviews == 0 {
			return ReviewDecision{}, ErrNoPositiveReview
		}
	}

	outcome := "positive"
	if kind == DecisionRequestChanges {
		outcome = "changes_requested"
	}

	decision, err := insertDecision(ctx, tx, requestID, revisionID, currentHash, kind, outcome, actorID, reason, operationKey)
	if err != nil {
		return ReviewDecision{}, err
	}

	newStatus, err := applyDecisionEffect(ctx, tx, kind, requestID, workProductID, wpStatus)
	if err != nil {
		return ReviewDecision{}, err
	}
	decision.WorkProductState = newStatus

	correlationID := uuid.New()
	if _, err := tx.Exec(ctx,
		`INSERT INTO core.audit_events (actor_user_id, action, scope_type, scope_id, outcome, detail, correlation_id)
		 VALUES ($1, $2, 'project', $3, 'success', $4, $5)`,
		actorID, "wp."+kind, projectID, map[string]any{
			"work_product_id":   workProductID.String(),
			"review_request_id": requestID.String(),
			"revision_id":       revisionID.String(),
			"decision_id":       decision.ID.String(),
		}, correlationID); err != nil {
		return ReviewDecision{}, fmt.Errorf("аудит рішення: %w", err)
	}

	if eventKey := decisionEventKey(kind); eventKey != "" {
		if err := automation.EmitEvent(ctx, tx, eventKey, &projectID, &actorID, correlationID, map[string]any{
			"work_product_id": workProductID.String(),
			"revision_id":     revisionID.String(),
		}); err != nil {
			return ReviewDecision{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return ReviewDecision{}, fmt.Errorf("фіксація рішення: %w", err)
	}
	return decision, nil
}

func ensureAssigned(ctx context.Context, tx pgx.Tx, requestID, actorID uuid.UUID, roles []string) error {
	var assigned bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (
		    SELECT 1 FROM core.review_assignments
		    WHERE review_request_id = $1 AND user_id = $2 AND assignment_role = ANY($3)
		 )`, requestID, actorID, roles).Scan(&assigned); err != nil {
		return fmt.Errorf("перевірка призначення: %w", err)
	}
	if !assigned {
		return fmt.Errorf("%w: потрібна роль %v", ErrNotAssigned, roles)
	}
	return nil
}

func insertDecision(ctx context.Context, tx pgx.Tx, requestID, revisionID uuid.UUID, hash []byte,
	kind, outcome string, actorID uuid.UUID, reason, operationKey string) (ReviewDecision, error) {
	var d ReviewDecision
	var storedReason *string
	err := tx.QueryRow(ctx,
		`INSERT INTO core.review_decisions
		   (review_request_id, revision_id, payload_hash, decision_kind, outcome, decided_by, reason, operation_key)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, review_request_id, revision_id, decision_kind, outcome, decided_by, reason, decided_at`,
		requestID, revisionID, hash, kind, outcome, actorID, nullIfBlank(reason), nullIfBlank(operationKey)).
		Scan(&d.ID, &d.ReviewRequestID, &d.RevisionID, &d.DecisionKind, &d.Outcome, &d.DecidedBy, &storedReason, &d.DecidedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ReviewDecision{}, fmt.Errorf("рішення цього роду вже ухвалено цією особою: %w", err)
		}
		return ReviewDecision{}, fmt.Errorf("запис рішення: %w", err)
	}
	if storedReason != nil {
		d.Reason = *storedReason
	}
	return d, nil
}

// applyDecisionEffect змінює стан запиту й артефакту.
//
// Позитивна рецензія свідомо НЕ змінює стан артефакту (ADR-009 §2): вона лише
// фіксує власний незмінний запис і є передумовою погодження.
func applyDecisionEffect(ctx context.Context, tx pgx.Tx, kind string, requestID, workProductID uuid.UUID, currentStatus string) (string, error) {
	switch kind {
	case DecisionReview:
		return currentStatus, nil

	case DecisionApproval:
		if _, err := tx.Exec(ctx,
			`UPDATE core.review_requests SET status = 'approved', closed_at = now() WHERE id = $1`, requestID); err != nil {
			return "", fmt.Errorf("закриття запиту як approved: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE core.work_products SET status = 'approved', row_version = row_version + 1 WHERE id = $1`,
			workProductID); err != nil {
			return "", fmt.Errorf("перехід артефакту в approved: %w", err)
		}
		return "approved", nil

	case DecisionRequestChanges:
		if _, err := tx.Exec(ctx,
			`UPDATE core.review_requests SET status = 'changes_requested', closed_at = now() WHERE id = $1`,
			requestID); err != nil {
			return "", fmt.Errorf("закриття запиту як changes_requested: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE core.work_products SET status = 'draft', row_version = row_version + 1 WHERE id = $1`,
			workProductID); err != nil {
			return "", fmt.Errorf("повернення артефакту в draft: %w", err)
		}
		return "draft", nil
	}
	return "", fmt.Errorf("невідомий рід рішення %q", kind)
}

func loadDecisionByOperationKey(ctx context.Context, tx pgx.Tx, operationKey string) (ReviewDecision, bool, error) {
	var d ReviewDecision
	var storedReason *string
	err := tx.QueryRow(ctx,
		`SELECT d.id, d.review_request_id, d.revision_id, d.decision_kind, d.outcome,
		        d.decided_by, d.reason, d.decided_at, wp.status
		 FROM core.review_decisions d
		 JOIN core.review_requests rr ON rr.id = d.review_request_id
		 JOIN core.work_products wp ON wp.id = rr.work_product_id
		 WHERE d.operation_key = $1`, operationKey).
		Scan(&d.ID, &d.ReviewRequestID, &d.RevisionID, &d.DecisionKind, &d.Outcome,
			&d.DecidedBy, &storedReason, &d.DecidedAt, &d.WorkProductState)
	if errors.Is(err, pgx.ErrNoRows) {
		return ReviewDecision{}, false, nil
	}
	if err != nil {
		return ReviewDecision{}, false, fmt.Errorf("пошук рішення за ключем операції: %w", err)
	}
	if storedReason != nil {
		d.Reason = *storedReason
	}
	return d, true, nil
}

func decisionEventKey(kind string) string {
	switch kind {
	case DecisionApproval:
		return "wp.approved"
	case DecisionRequestChanges:
		return "wp.changes_requested"
	}
	return ""
}

func nullIfBlank(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// planRevisionApproved повідомляє, чи має вказана ревізія позитивне
// погодження через робочий процес рецензування (ADR-009 §5).
func planRevisionApproved(ctx context.Context, tx pgx.Tx, revisionID uuid.UUID) (bool, error) {
	var approved bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (
		    SELECT 1
		    FROM core.review_decisions d
		    JOIN core.review_requests rr ON rr.id = d.review_request_id
		    WHERE rr.revision_id = $1
		      AND d.decision_kind = 'approval'
		      AND d.outcome = 'positive'
		 )`, revisionID).Scan(&approved); err != nil {
		return false, fmt.Errorf("перевірка погодження ревізії плану: %w", err)
	}
	return approved, nil
}
