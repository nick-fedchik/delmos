package project_test

import (
	"context"
	"errors"
	"testing"

	"delmos/internal/automation"
	"delmos/internal/project"
)

func TestCreateWorkProductRejectsUnknownType(t *testing.T) {
	ctx := context.Background()
	store, authStore := newTestStore(t)
	userID := newTestUser(t, authStore, "creator")
	detail, err := store.CreateWithPlan(ctx, userID, "WP-001", "Project", "")
	if err != nil {
		t.Fatalf("створення проєкту: %v", err)
	}

	_, _, err = store.CreateWorkProduct(ctx, userID, detail.Project.ID, "REQ-001", "not_a_type", "T", "", nil)
	if !errors.Is(err, project.ErrInvalidWorkProductType) {
		t.Errorf("очікувалася ErrInvalidWorkProductType, отримано %v", err)
	}
}

func TestCreateWorkProductRejectsAdditionalProjectPlan(t *testing.T) {
	ctx := context.Background()
	store, authStore := newTestStore(t)
	userID := newTestUser(t, authStore, "creator")
	detail, err := store.CreateWithPlan(ctx, userID, "WP-PLAN-001", "Project", "")
	if err != nil {
		t.Fatalf("створення проєкту: %v", err)
	}

	_, _, err = store.CreateWorkProduct(ctx, userID, detail.Project.ID, "PLAN-002", "plan", "Other plan", "", nil)
	if !errors.Is(err, project.ErrProjectPlanReserved) {
		t.Errorf("очікувалася ErrProjectPlanReserved, отримано %v", err)
	}
}

func TestCreateWorkProductAndRevise(t *testing.T) {
	ctx := context.Background()
	store, authStore := newTestStore(t)
	userID := newTestUser(t, authStore, "creator")
	detail, err := store.CreateWithPlan(ctx, userID, "WP-002", "Project", "")
	if err != nil {
		t.Fatalf("створення проєкту: %v", err)
	}

	wp, r1, err := store.CreateWorkProduct(ctx, userID, detail.Project.ID, "REQ-001", "requirement", "Живлення", "перша версія", nil)
	if err != nil {
		t.Fatalf("створення work product: %v", err)
	}
	if wp.Status != "draft" || r1.RevisionNumber != 1 {
		t.Errorf("новий WP має бути draft/r1, отримано %+v / %+v", wp, r1)
	}

	r2, err := store.ReviseWorkProduct(ctx, userID, detail.Project.ID, wp.ID, wp.RowVersion, "друга версія", map[string]any{"k": "v"})
	if err != nil {
		t.Fatalf("створення другої ревізії: %v", err)
	}
	if r2.RevisionNumber != 2 {
		t.Errorf("очікувався revision_number=2, отримано %d", r2.RevisionNumber)
	}
	if string(r1.PayloadHash) == string(r2.PayloadHash) {
		t.Error("зміна тіла має змінювати payload_hash")
	}

	fetched, err := store.GetWorkProduct(ctx, detail.Project.ID, wp.ID)
	if err != nil {
		t.Fatalf("читання work product: %v", err)
	}
	if fetched.Latest.RevisionNumber != 2 || fetched.Latest.Body != "друга версія" {
		t.Errorf("читання має повертати останню ревізію, отримано %+v", fetched.Latest)
	}
	if fetched.WorkProduct.RowVersion != 2 {
		t.Errorf("row_version має зрости до 2, отримано %d", fetched.WorkProduct.RowVersion)
	}
}

func TestReviseWorkProductDetectsVersionConflict(t *testing.T) {
	ctx := context.Background()
	store, authStore := newTestStore(t)
	userID := newTestUser(t, authStore, "creator")
	detail, _ := store.CreateWithPlan(ctx, userID, "WP-003", "Project", "")
	wp, _, err := store.CreateWorkProduct(ctx, userID, detail.Project.ID, "REQ-001", "requirement", "T", "v1", nil)
	if err != nil {
		t.Fatalf("створення work product: %v", err)
	}

	if _, err := store.ReviseWorkProduct(ctx, userID, detail.Project.ID, wp.ID, wp.RowVersion, "v2", nil); err != nil {
		t.Fatalf("перша ревізія: %v", err)
	}

	// Повторний запит зі старим row_version імітує конкурентний конфлікт.
	if _, err := store.ReviseWorkProduct(ctx, userID, detail.Project.ID, wp.ID, wp.RowVersion, "v3-stale", nil); !errors.Is(err, project.ErrVersionConflict) {
		t.Errorf("очікувалася ErrVersionConflict, отримано %v", err)
	}
}

func TestRetireWorkProductIsIdempotentAndBlocksFurtherRevision(t *testing.T) {
	ctx := context.Background()
	store, authStore := newTestStore(t)
	userID := newTestUser(t, authStore, "creator")
	detail, _ := store.CreateWithPlan(ctx, userID, "WP-004", "Project", "")
	wp, _, err := store.CreateWorkProduct(ctx, userID, detail.Project.ID, "REQ-001", "requirement", "T", "v1", nil)
	if err != nil {
		t.Fatalf("створення work product: %v", err)
	}
	var versionBefore int64
	if err := authStore.Pool().QueryRow(ctx,
		`SELECT version FROM core.metric_input_versions WHERE project_id = $1`, detail.Project.ID).Scan(&versionBefore); err != nil {
		t.Fatalf("читання версії входів метрик: %v", err)
	}

	if err := store.RetireWorkProduct(ctx, userID, detail.Project.ID, wp.ID, wp.RowVersion); err != nil {
		t.Fatalf("перше виведення з експлуатації: %v", err)
	}
	// Повторний виклик з тим самим (застарілим) row_version — безпечний no-op, не помилка.
	if err := store.RetireWorkProduct(ctx, userID, detail.Project.ID, wp.ID, wp.RowVersion); err != nil {
		t.Errorf("повторне виведення з експлуатації має бути no-op: %v", err)
	}
	var emitted int
	if err := authStore.Pool().QueryRow(ctx,
		`SELECT count(*) FROM core.event_outbox WHERE event_key = 'wp.retired' AND project_id = $1`,
		detail.Project.ID).Scan(&emitted); err != nil {
		t.Fatalf("читання подій retirement: %v", err)
	}
	if emitted != 1 {
		t.Fatalf("очікується одна подія wp.retired після повтору, отримано %d", emitted)
	}
	var versionAfter int64
	if err := authStore.Pool().QueryRow(ctx,
		`SELECT version FROM core.metric_input_versions WHERE project_id = $1`, detail.Project.ID).Scan(&versionAfter); err != nil {
		t.Fatalf("читання версії входів після виведення: %v", err)
	}
	if versionAfter != versionBefore+1 {
		t.Fatalf("виведення має інвалідувати метрики рівно раз: %d -> %d", versionBefore, versionAfter)
	}

	if _, err := store.ReviseWorkProduct(ctx, userID, detail.Project.ID, wp.ID, wp.RowVersion+1, "v2", nil); !errors.Is(err, project.ErrWorkProductObsolete) {
		t.Errorf("ревізія obsolete work product має відхилятися, отримано %v", err)
	}
}

func TestRetireWorkProductRejectsPrimaryProjectPlan(t *testing.T) {
	ctx := context.Background()
	store, authStore := newTestStore(t)
	actorID := newTestUser(t, authStore, "plan-retirement")
	projectDetail, err := store.CreateWithPlan(ctx, actorID, "WP-PLAN-RETIRE", "Project", "")
	if err != nil {
		t.Fatal(err)
	}
	plan := projectDetail.Plan
	if err := store.RetireWorkProduct(ctx, actorID, projectDetail.Project.ID, plan.ID, plan.RowVersion); !errors.Is(err, project.ErrProjectPlanReserved) {
		t.Fatalf("основний PLAN-001 не можна вивести з експлуатації: %v", err)
	}
	readBack, err := store.GetWorkProduct(ctx, projectDetail.Project.ID, plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if readBack.WorkProduct.Status != "draft" {
		t.Fatalf("PLAN-001 змінив статус: %s", readBack.WorkProduct.Status)
	}
}

func TestRetireWorkProductHonorsBeforeRule(t *testing.T) {
	f := newPhaseFixture(t)
	wp, _, err := f.projects.CreateWorkProduct(context.Background(), f.actor, f.projectID,
		"REQ-RETIRE", "requirement", "Retirement", "initial", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.pool.Exec(context.Background(), `INSERT INTO core.rule_definitions
		(rule_key, trigger_key, enforcement_level, when_condition, assert_condition)
		VALUES ('rule.test.veto_retire', 'trigger.core.before_wp_transition', 'MANDATORY_VETO',
		'{"predicate_key":"field_equals","params":{"field":"action","value":"retire"}}',
		'{"predicate_key":"always_false"}')`)
	if err != nil {
		t.Fatal(err)
	}
	err = f.projects.RetireWorkProduct(context.Background(), f.actor, f.projectID, wp.ID, wp.RowVersion)
	var violation *automation.RuleViolationError
	if !errors.As(err, &violation) || violation.RuleKey != "rule.test.veto_retire" {
		t.Fatalf("expected retire rule veto, got %v", err)
	}
	detail, err := f.projects.GetWorkProduct(context.Background(), f.projectID, wp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.WorkProduct.Status != "draft" || detail.WorkProduct.RowVersion != wp.RowVersion {
		t.Fatalf("veto must leave work product unchanged: %+v", detail.WorkProduct)
	}
}

func TestWorkProductIsScopedToItsProject(t *testing.T) {
	ctx := context.Background()
	store, authStore := newTestStore(t)
	userID := newTestUser(t, authStore, "creator")
	first, _ := store.CreateWithPlan(ctx, userID, "WP-005", "P1", "")
	second, _ := store.CreateWithPlan(ctx, userID, "WP-006", "P2", "")

	wp, _, err := store.CreateWorkProduct(ctx, userID, first.Project.ID, "REQ-001", "requirement", "T", "v1", nil)
	if err != nil {
		t.Fatalf("створення work product: %v", err)
	}

	if _, err := store.GetWorkProduct(ctx, second.Project.ID, wp.ID); !errors.Is(err, project.ErrWorkProductNotFound) {
		t.Errorf("WP іншого проєкту має бути невидимим, отримано %v", err)
	}
}
