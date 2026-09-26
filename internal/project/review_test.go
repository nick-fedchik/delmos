package project_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"delmos/internal/project"
)

// submitFixture доповнює phaseFixture артефактом-чернеткою з однією ревізією.
type submitFixture struct {
	*phaseFixture
	workProductID uuid.UUID
	reviewer      uuid.UUID
	approver2     uuid.UUID
}

func newSubmitFixture(t *testing.T) *submitFixture {
	t.Helper()
	base := newPhaseFixture(t)
	f := &submitFixture{phaseFixture: base}
	f.reviewer = base.user(t, "reviewer")
	f.approver2 = base.user(t, "approver")
	f.workProductID = f.draftWithRevision(t, "REQ-1", "перший текст")
	return f
}

// draftWithRevision створює артефакт і першу ревізію напряму в БД: тест
// перевіряє подання на рецензію, а не шлях створення артефакту.
func (f *submitFixture) draftWithRevision(t *testing.T, code, body string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var wpID uuid.UUID
	if err := f.pool.QueryRow(ctx,
		`INSERT INTO core.work_products (project_id, code, type, profile, title, status)
		 VALUES ($1, $2, 'requirement', 'core:requirement', $3, 'draft') RETURNING id`,
		f.projectID, code, code).Scan(&wpID); err != nil {
		t.Fatalf("створення артефакту: %v", err)
	}
	f.addRevision(t, wpID, body, f.actor)
	return wpID
}

func (f *submitFixture) addRevision(t *testing.T, wpID uuid.UUID, body string, author uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := f.pool.QueryRow(context.Background(),
		`INSERT INTO core.work_product_revisions
		   (work_product_id, revision_number, body, metadata, payload_hash, content_hash, created_by)
		 VALUES ($1,
		         COALESCE((SELECT max(revision_number) FROM core.work_product_revisions WHERE work_product_id = $1), 0) + 1,
		         $2::text, '{}'::jsonb,
		         sha256(convert_to($2::text, 'UTF8')), sha256(convert_to($2::text, 'UTF8')), $3)
		 RETURNING id`, wpID, body, author).Scan(&id); err != nil {
		t.Fatalf("створення ревізії: %v", err)
	}
	return id
}

func (f *submitFixture) defaultAssignments() []project.ReviewAssignment {
	return []project.ReviewAssignment{
		{UserID: f.reviewer, Role: "reviewer"},
		{UserID: f.approver2, Role: "approver"},
	}
}

func (f *submitFixture) wpStatus(t *testing.T, wpID uuid.UUID) string {
	t.Helper()
	var status string
	if err := f.pool.QueryRow(context.Background(),
		`SELECT status FROM core.work_products WHERE id = $1`, wpID).Scan(&status); err != nil {
		t.Fatalf("читання статусу артефакту: %v", err)
	}
	return status
}

func (f *submitFixture) requestStatus(t *testing.T, requestID uuid.UUID) string {
	t.Helper()
	var status string
	if err := f.pool.QueryRow(context.Background(),
		`SELECT status FROM core.review_requests WHERE id = $1`, requestID).Scan(&status); err != nil {
		t.Fatalf("читання статусу запиту: %v", err)
	}
	return status
}

func TestSubmitMovesWorkProductToInReview(t *testing.T) {
	f := newSubmitFixture(t)

	req, err := f.projects.SubmitWorkProduct(context.Background(),
		f.actor, f.projectID, f.workProductID, f.defaultAssignments())
	if err != nil {
		t.Fatalf("подання на рецензію: %v", err)
	}
	if got := f.wpStatus(t, f.workProductID); got != "in_review" {
		t.Errorf("статус артефакту = %q, очікувано in_review", got)
	}
	if req.Status != "open" {
		t.Errorf("статус запиту = %q, очікувано open", req.Status)
	}

	// Запит має вказувати саме на останню ревізію (ADR-009 §1).
	var revisionID uuid.UUID
	if err := f.pool.QueryRow(context.Background(),
		`SELECT id FROM core.work_product_revisions
		 WHERE work_product_id = $1 ORDER BY revision_number DESC LIMIT 1`, f.workProductID).
		Scan(&revisionID); err != nil {
		t.Fatalf("читання ревізії: %v", err)
	}
	if req.RevisionID != revisionID {
		t.Errorf("запит прив'язано до ревізії %s, очікувано %s", req.RevisionID, revisionID)
	}
}

// Автор ревізії не може бути рецензентом чи погоджувачем — це ядро SoD
// (ADR-009 §2). Перевірка має спрацьовувати вже на подінні, а не на рішенні.
func TestSubmitRejectsAuthorAsAssignee(t *testing.T) {
	for _, role := range []string{"reviewer", "approver"} {
		t.Run(role, func(t *testing.T) {
			f := newSubmitFixture(t)
			assignments := []project.ReviewAssignment{
				{UserID: f.reviewer, Role: "reviewer"},
				{UserID: f.approver2, Role: "approver"},
				{UserID: f.actor, Role: role}, // автор ревізії
			}
			_, err := f.projects.SubmitWorkProduct(context.Background(),
				f.actor, f.projectID, f.workProductID, assignments)
			if !errors.Is(err, project.ErrAssigneeIsAuthor) {
				t.Errorf("очікувано ErrAssigneeIsAuthor, отримано %v", err)
			}
			if got := f.wpStatus(t, f.workProductID); got != "draft" {
				t.Errorf("статус артефакту = %q, очікувано draft — транзакція мала відкотитися", got)
			}
		})
	}
}

// Два паралельні подання створили б дві незалежні гілки погодження на різні
// ревізії одного артефакту. Інваріант тримає частковий унікальний індекс.
func TestSubmitTwiceRejected(t *testing.T) {
	f := newSubmitFixture(t)
	ctx := context.Background()

	if _, err := f.projects.SubmitWorkProduct(ctx, f.actor, f.projectID, f.workProductID, f.defaultAssignments()); err != nil {
		t.Fatalf("перше подання: %v", err)
	}
	_, err := f.projects.SubmitWorkProduct(ctx, f.actor, f.projectID, f.workProductID, f.defaultAssignments())
	if err == nil {
		t.Fatal("очікувалась відмова повторного подання")
	}
	// Артефакт уже in_review, тож спрацьовує перевірка стану — це теж
	// коректний захист, важливо лише що другий відкритий запит не з'явився.
	var open int
	if err := f.pool.QueryRow(ctx,
		`SELECT count(*) FROM core.review_requests WHERE work_product_id = $1 AND status = 'open'`,
		f.workProductID).Scan(&open); err != nil {
		t.Fatalf("підрахунок відкритих запитів: %v", err)
	}
	if open != 1 {
		t.Errorf("відкритих запитів = %d, очікувано рівно 1", open)
	}
}

func TestSubmitRejectedForNonDraft(t *testing.T) {
	f := newSubmitFixture(t)
	if _, err := f.pool.Exec(context.Background(),
		`UPDATE core.work_products SET status = 'approved' WHERE id = $1`, f.workProductID); err != nil {
		t.Fatalf("підготовка статусу: %v", err)
	}

	_, err := f.projects.SubmitWorkProduct(context.Background(),
		f.actor, f.projectID, f.workProductID, f.defaultAssignments())
	if !errors.Is(err, project.ErrNotDraft) {
		t.Errorf("очікувано ErrNotDraft, отримано %v", err)
	}
}

// Ключовий інваріант ADR-009 §3: нова ревізія скасовує відкритий запит, бо
// рішення ніколи не переносяться на новий хеш.
func TestNewRevisionSupersedesOpenReviewRequest(t *testing.T) {
	f := newSubmitFixture(t)
	ctx := context.Background()

	req, err := f.projects.SubmitWorkProduct(ctx, f.actor, f.projectID, f.workProductID, f.defaultAssignments())
	if err != nil {
		t.Fatalf("подання: %v", err)
	}

	var rowVersion int64
	if err := f.pool.QueryRow(ctx,
		`SELECT row_version FROM core.work_products WHERE id = $1`, f.workProductID).Scan(&rowVersion); err != nil {
		t.Fatalf("читання row_version: %v", err)
	}
	if _, err := f.projects.ReviseWorkProduct(ctx, f.actor, f.projectID, f.workProductID,
		rowVersion, "виправлений текст", nil); err != nil {
		t.Fatalf("нова ревізія: %v", err)
	}

	if got := f.requestStatus(t, req.ID); got != "superseded" {
		t.Errorf("статус запиту = %q, очікувано superseded після нової ревізії", got)
	}
}

// Рішення не може посилатися на ревізію, відмінну від тієї, на яку створено
// запит. Інваріант тримає складений зовнішній ключ, а не код.
func TestDecisionOnForeignRevisionRejectedByDatabase(t *testing.T) {
	f := newSubmitFixture(t)
	ctx := context.Background()

	req, err := f.projects.SubmitWorkProduct(ctx, f.actor, f.projectID, f.workProductID, f.defaultAssignments())
	if err != nil {
		t.Fatalf("подання: %v", err)
	}
	otherWP := f.draftWithRevision(t, "REQ-2", "інший текст")
	var foreignRevision uuid.UUID
	if err := f.pool.QueryRow(ctx,
		`SELECT id FROM core.work_product_revisions WHERE work_product_id = $1`, otherWP).
		Scan(&foreignRevision); err != nil {
		t.Fatalf("читання чужої ревізії: %v", err)
	}

	_, err = f.pool.Exec(ctx,
		`INSERT INTO core.review_decisions
		   (review_request_id, revision_id, payload_hash, decision_kind, outcome, decided_by)
		 VALUES ($1, $2, '\x00'::bytea, 'review', 'positive', $3)`,
		req.ID, foreignRevision, f.reviewer)
	if err == nil {
		t.Fatal("СУБД мала відхилити рішення на ревізію, що не належить запиту")
	}
}

// «Потрібні зміни» без причини не є вмотивованим рішенням (ADR-009 §3).
func TestRequestChangesWithoutReasonRejectedByDatabase(t *testing.T) {
	f := newSubmitFixture(t)
	ctx := context.Background()

	req, err := f.projects.SubmitWorkProduct(ctx, f.actor, f.projectID, f.workProductID, f.defaultAssignments())
	if err != nil {
		t.Fatalf("подання: %v", err)
	}

	_, err = f.pool.Exec(ctx,
		`INSERT INTO core.review_decisions
		   (review_request_id, revision_id, payload_hash, decision_kind, outcome, decided_by, reason)
		 VALUES ($1, $2, '\x00'::bytea, 'request_changes', 'changes_requested', $3, NULL)`,
		req.ID, req.RevisionID, f.reviewer)
	if err == nil {
		t.Fatal("СУБД мала відхилити «потрібні зміни» без причини")
	}
}

// Одна особа не може дати два рішення того самого роду на один запит —
// інакше повторний виклик додав би другий підпис до кворуму.
func TestDuplicateDecisionOfSameKindRejectedByDatabase(t *testing.T) {
	f := newSubmitFixture(t)
	ctx := context.Background()

	req, err := f.projects.SubmitWorkProduct(ctx, f.actor, f.projectID, f.workProductID, f.defaultAssignments())
	if err != nil {
		t.Fatalf("подання: %v", err)
	}
	insert := func() error {
		_, err := f.pool.Exec(ctx,
			`INSERT INTO core.review_decisions
			   (review_request_id, revision_id, payload_hash, decision_kind, outcome, decided_by)
			 VALUES ($1, $2, '\x00'::bytea, 'review', 'positive', $3)`,
			req.ID, req.RevisionID, f.reviewer)
		return err
	}
	if err := insert(); err != nil {
		t.Fatalf("перше рішення: %v", err)
	}
	if err := insert(); err == nil {
		t.Fatal("СУБД мала відхилити друге рішення того самого роду від тієї самої особи")
	}
}
