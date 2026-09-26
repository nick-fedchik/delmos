package project_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"delmos/internal/project"
)

// planFixture працює з обов'язковим PLAN-001, який створюється разом із
// проєктом: план — теж артефакт і погоджується тією самою процедурою.
type planFixture struct {
	*phaseFixture
	planWorkProductID uuid.UUID
	planRowVersion    int64
	reviewer          uuid.UUID
	approver          uuid.UUID
}

func newPlanFixture(t *testing.T) *planFixture {
	t.Helper()
	base := newPhaseFixture(t)
	f := &planFixture{phaseFixture: base}
	f.reviewer = base.user(t, "plan-reviewer")
	f.approver = base.user(t, "plan-approver")

	// Проєкт створюється штатним шляхом: лише він заводить обов'язковий
	// PLAN-001 разом із першою ревізією та маніфестом.
	detail, err := f.projects.CreateWithPlan(context.Background(), f.actor, "PLAN-GATE", "Шлюз плану", "")
	if err != nil {
		t.Fatalf("створення проєкту з планом: %v", err)
	}
	f.projectID = detail.Project.ID

	if err := f.pool.QueryRow(context.Background(),
		`SELECT id, row_version FROM core.work_products
		 WHERE project_id = $1 AND code = 'PLAN-001'`, f.projectID).
		Scan(&f.planWorkProductID, &f.planRowVersion); err != nil {
		t.Fatalf("читання PLAN-001: %v", err)
	}
	return f
}

func (f *planFixture) latestPlanRevision(t *testing.T) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := f.pool.QueryRow(context.Background(),
		`SELECT id FROM core.work_product_revisions
		 WHERE work_product_id = $1 ORDER BY revision_number DESC LIMIT 1`, f.planWorkProductID).
		Scan(&id); err != nil {
		t.Fatalf("читання ревізії плану: %v", err)
	}
	return id
}

// approveLatestPlanRevision проводить план через увесь робочий процес.
func (f *planFixture) approveLatestPlanRevision(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	if _, err := f.projects.SubmitWorkProduct(ctx, f.actor, f.projectID, f.planWorkProductID,
		[]project.ReviewAssignment{
			{UserID: f.reviewer, Role: "reviewer"},
			{UserID: f.approver, Role: "approver"},
		}); err != nil {
		t.Fatalf("подання плану: %v", err)
	}
	if _, err := f.projects.RecordDecision(ctx, f.reviewer, f.projectID, f.planWorkProductID,
		project.DecisionReview, "", ""); err != nil {
		t.Fatalf("рецензія плану: %v", err)
	}
	if _, err := f.projects.RecordDecision(ctx, f.approver, f.projectID, f.planWorkProductID,
		project.DecisionApproval, "", ""); err != nil {
		t.Fatalf("погодження плану: %v", err)
	}
	// Подання та погодження змінюють row_version артефакту.
	if err := f.pool.QueryRow(ctx,
		`SELECT row_version FROM core.work_products WHERE id = $1`, f.planWorkProductID).
		Scan(&f.planRowVersion); err != nil {
		t.Fatalf("оновлення row_version плану: %v", err)
	}
}

// ADR-009 §5: непогоджений план не вводиться в дію.
func TestApplyPlanRejectedWithoutApproval(t *testing.T) {
	f := newPlanFixture(t)

	_, err := f.projects.ApplyPlan(context.Background(),
		f.actor, f.projectID, nil, f.planRowVersion)
	if !errors.Is(err, project.ErrPlanNotApproved) {
		t.Fatalf("очікувано ErrPlanNotApproved, отримано %v", err)
	}

	// Стан проєкту не змінився: покоління конфігурації лишилося колишнім.
	var applications int
	if err := f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM core.project_plan_applications WHERE project_id = $1`, f.projectID).
		Scan(&applications); err != nil {
		t.Fatalf("підрахунок застосувань: %v", err)
	}
	if applications != 0 {
		t.Errorf("застосувань = %d, очікувано 0 — транзакція мала відкотитися", applications)
	}
}

// Дзеркальний випадок: після погодження план вводиться в дію.
func TestApplyPlanSucceedsAfterApproval(t *testing.T) {
	f := newPlanFixture(t)
	f.approveLatestPlanRevision(t)

	applied, err := f.projects.ApplyPlan(context.Background(),
		f.actor, f.projectID, nil, f.planRowVersion)
	if err != nil {
		t.Fatalf("застосування погодженого плану: %v", err)
	}
	if applied.ConfigGeneration == 0 {
		t.Error("покоління конфігурації не підвищено")
	}

	var applications int
	if err := f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM core.project_plan_applications WHERE project_id = $1`, f.projectID).
		Scan(&applications); err != nil {
		t.Fatalf("підрахунок застосувань: %v", err)
	}
	if applications != 1 {
		t.Errorf("застосувань = %d, очікувано 1", applications)
	}
}

// Найтонший випадок: після першого затвердження артефакт плану лишається
// approved назавжди. Якби перевірка спиралася на статус артефакту, наступна
// непогоджена ревізія пройшла б у дію непоміченою.
func TestApplyPlanRejectsUnapprovedNewerRevision(t *testing.T) {
	f := newPlanFixture(t)
	f.approveLatestPlanRevision(t)
	approvedRevision := f.latestPlanRevision(t)

	if _, err := f.projects.ApplyPlan(context.Background(),
		f.actor, f.projectID, nil, f.planRowVersion); err != nil {
		t.Fatalf("застосування погодженої ревізії: %v", err)
	}
	f.refreshPlanRowVersion(t)

	// Артефакт плану після затвердження лишається approved — переконуємось,
	// що саме це й відбувається, інакше тест перевіряв би не те.
	if got := f.wpStatusByID(t, f.planWorkProductID); got != "approved" {
		t.Fatalf("статус плану = %q, очікувано approved", got)
	}

	newRevision := f.revisePlan(t, "змінений план")
	if newRevision == approvedRevision {
		t.Fatal("нова ревізія не створилася")
	}

	_, err := f.projects.ApplyPlan(context.Background(),
		f.actor, f.projectID, &newRevision, f.planRowVersion)
	if !errors.Is(err, project.ErrPlanNotApproved) {
		t.Errorf("очікувано ErrPlanNotApproved для нової ревізії, отримано %v", err)
	}
}

func (f *planFixture) refreshPlanRowVersion(t *testing.T) {
	t.Helper()
	if err := f.pool.QueryRow(context.Background(),
		`SELECT row_version FROM core.work_products WHERE id = $1`, f.planWorkProductID).
		Scan(&f.planRowVersion); err != nil {
		t.Fatalf("оновлення row_version плану: %v", err)
	}
}

func (f *planFixture) wpStatusByID(t *testing.T, id uuid.UUID) string {
	t.Helper()
	var status string
	if err := f.pool.QueryRow(context.Background(),
		`SELECT status FROM core.work_products WHERE id = $1`, id).Scan(&status); err != nil {
		t.Fatalf("читання статусу: %v", err)
	}
	return status
}

// revisePlan створює нову ревізію плану з тим самим маніфестом, змінюючи лише
// тіло: мета — отримати новий payload_hash, а не перевірити валідацію.
func (f *planFixture) revisePlan(t *testing.T, body string) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var revisionID uuid.UUID
	if err := f.pool.QueryRow(ctx,
		`INSERT INTO core.work_product_revisions
		   (work_product_id, revision_number, body, metadata, payload_hash, content_hash, created_by)
		 VALUES ($1,
		         COALESCE((SELECT max(revision_number) FROM core.work_product_revisions WHERE work_product_id = $1), 0) + 1,
		         $2::text, '{}'::jsonb,
		         sha256(convert_to($2::text, 'UTF8')), sha256(convert_to($2::text, 'UTF8')), $3)
		 RETURNING id`, f.planWorkProductID, body, f.actor).Scan(&revisionID); err != nil {
		t.Fatalf("створення ревізії плану: %v", err)
	}

	// Маніфест копіюється з попередньої ревізії цілком: мета — новий
	// payload_hash, а не інший зміст плану.
	if _, err := f.pool.Exec(ctx,
		`INSERT INTO core.project_plan_manifests
		   (revision_id, template_key, template_version, manifest, manifest_hash)
		 SELECT $1, pm.template_key, pm.template_version, pm.manifest, pm.manifest_hash
		 FROM core.project_plan_manifests pm
		 JOIN core.work_product_revisions r ON r.id = pm.revision_id
		 WHERE r.work_product_id = $2 AND r.id <> $1
		 ORDER BY r.revision_number DESC LIMIT 1`,
		revisionID, f.planWorkProductID); err != nil {
		t.Fatalf("маніфест нової ревізії: %v", err)
	}
	return revisionID
}
