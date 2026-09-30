package project_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"delmos/internal/automation"
	"delmos/internal/project"
)

// submitted готує артефакт у стані in_review з призначеними рецензентом і
// погоджувачем, незалежними від автора.
func (f *submitFixture) submitted(t *testing.T) project.ReviewRequest {
	t.Helper()
	req, err := f.projects.SubmitWorkProduct(context.Background(),
		f.actor, f.projectID, f.workProductID, f.defaultAssignments())
	if err != nil {
		t.Fatalf("подання: %v", err)
	}
	return req
}

func (f *submitFixture) decide(t *testing.T, actor uuid.UUID, kind, reason, opKey string) (project.ReviewDecision, error) {
	t.Helper()
	return f.projects.RecordDecision(context.Background(),
		actor, f.projectID, f.workProductID, kind, reason, opKey)
}

// Позитивна рецензія фіксує запис, але свідомо НЕ переводить артефакт у
// approved (ADR-009 §2) — інакше рецензування підмінило б погодження.
func TestPositiveReviewDoesNotApprove(t *testing.T) {
	f := newSubmitFixture(t)
	f.submitted(t)

	decision, err := f.decide(t, f.reviewer, project.DecisionReview, "", "")
	if err != nil {
		t.Fatalf("рецензія: %v", err)
	}
	if decision.Outcome != "positive" {
		t.Errorf("результат = %q, очікувано positive", decision.Outcome)
	}
	if got := f.wpStatus(t, f.workProductID); got != "in_review" {
		t.Errorf("статус артефакту = %q, очікувано in_review — рецензія не затверджує", got)
	}
}

// Кворум ADR-009 §2: погодження неможливе без попередньої позитивної рецензії.
func TestApprovalRequiresPositiveReviewFirst(t *testing.T) {
	f := newSubmitFixture(t)
	f.submitted(t)

	_, err := f.decide(t, f.approver2, project.DecisionApproval, "", "")
	if !errors.Is(err, project.ErrNoPositiveReview) {
		t.Errorf("очікувано ErrNoPositiveReview, отримано %v", err)
	}
	if got := f.wpStatus(t, f.workProductID); got != "in_review" {
		t.Errorf("статус артефакту = %q, очікувано in_review", got)
	}
}

// Повний позитивний шлях: рецензія -> погодження -> approved.
func TestReviewThenApprovalApprovesWorkProduct(t *testing.T) {
	f := newSubmitFixture(t)
	req := f.submitted(t)

	if _, err := f.decide(t, f.reviewer, project.DecisionReview, "", ""); err != nil {
		t.Fatalf("рецензія: %v", err)
	}
	decision, err := f.decide(t, f.approver2, project.DecisionApproval, "", "")
	if err != nil {
		t.Fatalf("погодження: %v", err)
	}
	if decision.WorkProductState != "approved" {
		t.Errorf("стан артефакту у відповіді = %q, очікувано approved", decision.WorkProductState)
	}
	if got := f.wpStatus(t, f.workProductID); got != "approved" {
		t.Errorf("статус артефакту = %q, очікувано approved", got)
	}
	if got := f.requestStatus(t, req.ID); got != "approved" {
		t.Errorf("статус запиту = %q, очікувано approved", got)
	}
}

func TestApprovalHonorsBeforeWPTransitionRule(t *testing.T) {
	f := newSubmitFixture(t)
	req := f.submitted(t)
	if _, err := f.decide(t, f.reviewer, project.DecisionReview, "", ""); err != nil {
		t.Fatal(err)
	}
	_, err := f.pool.Exec(context.Background(), `INSERT INTO core.rule_definitions
		(rule_key, trigger_key, enforcement_level, when_condition, assert_condition)
		VALUES ('rule.test.no_approval', 'trigger.core.before_wp_transition', 'MANDATORY_VETO',
		'{"predicate_key":"field_equals","params":{"field":"target_status","value":"approved"}}',
		'{"predicate_key":"always_false"}')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.decide(t, f.approver2, project.DecisionApproval, "", "")
	var violation *automation.RuleViolationError
	if !errors.As(err, &violation) || violation.RuleKey != "rule.test.no_approval" {
		t.Fatalf("очікувано вето погодження, отримано %v", err)
	}
	if status := f.wpStatus(t, f.workProductID); status != "in_review" {
		t.Fatalf("після вето WP має лишитись in_review, отримано %s", status)
	}
	if status := f.requestStatus(t, req.ID); status != "open" {
		t.Fatalf("після вето запит має лишитись open, отримано %s", status)
	}
	var approvals int
	if err := f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM core.review_decisions WHERE review_request_id = $1 AND decision_kind = 'approval'`, req.ID).Scan(&approvals); err != nil {
		t.Fatal(err)
	}
	if approvals != 0 {
		t.Fatalf("після вето не повинно бути рішення погоджувача: %d", approvals)
	}
}

// Самопогодження: автор ревізії не ухвалює щодо неї рішень навіть якщо
// призначення якось з'явилося. Перевірка не покладається лише на подання.
func TestAuthorCannotDecideEvenIfAssigned(t *testing.T) {
	f := newSubmitFixture(t)
	req := f.submitted(t)

	// Призначення автора в обхід SubmitWorkProduct — імітуємо помилку
	// адміністрування, від якої має захистити сама команда рішення.
	if _, err := f.pool.Exec(context.Background(),
		`INSERT INTO core.review_assignments (review_request_id, user_id, assignment_role, assigned_by)
		 VALUES ($1, $2, 'reviewer', $2)`, req.ID, f.actor); err != nil {
		t.Fatalf("призначення автора: %v", err)
	}

	_, err := f.decide(t, f.actor, project.DecisionReview, "", "")
	if !errors.Is(err, project.ErrSelfDecision) {
		t.Errorf("очікувано ErrSelfDecision, отримано %v", err)
	}
}

// Наявність дозволу без призначення недостатня (ADR-009 §1).
func TestUnassignedPersonCannotDecide(t *testing.T) {
	f := newSubmitFixture(t)
	f.submitted(t)
	outsider := f.user(t, "outsider")

	_, err := f.decide(t, outsider, project.DecisionReview, "", "")
	if !errors.Is(err, project.ErrNotAssigned) {
		t.Errorf("очікувано ErrNotAssigned, отримано %v", err)
	}
}

// Рецензент не може погоджувати без призначення погоджувачем: одна особа
// може бути обома лише за наявності ОБОХ призначень (ADR-009 §2).
func TestReviewerCannotApproveWithoutApproverAssignment(t *testing.T) {
	f := newSubmitFixture(t)
	f.submitted(t)

	if _, err := f.decide(t, f.reviewer, project.DecisionReview, "", ""); err != nil {
		t.Fatalf("рецензія: %v", err)
	}
	_, err := f.decide(t, f.reviewer, project.DecisionApproval, "", "")
	if !errors.Is(err, project.ErrNotAssigned) {
		t.Errorf("очікувано ErrNotAssigned, отримано %v", err)
	}
}

// Дзеркальний випадок: та сама особа з ОБОМА призначеннями може дати обидва
// рішення. Кожне фіксується окремо (ADR-009 §2).
func TestSinglePersonWithBothAssignmentsMayDecideTwice(t *testing.T) {
	f := newSubmitFixture(t)
	dual := f.user(t, "dual")
	f.grantReviewRole(t, dual, "project.reviewer")
	f.grantReviewRole(t, dual, "project.approver")

	req, err := f.projects.SubmitWorkProduct(context.Background(),
		f.actor, f.projectID, f.workProductID, []project.ReviewAssignment{
			{UserID: dual, Role: "reviewer"},
			{UserID: dual, Role: "approver"},
		})
	if err != nil {
		t.Fatalf("подання: %v", err)
	}

	if _, err := f.decide(t, dual, project.DecisionReview, "", ""); err != nil {
		t.Fatalf("рецензія: %v", err)
	}
	if _, err := f.decide(t, dual, project.DecisionApproval, "", ""); err != nil {
		t.Fatalf("погодження: %v", err)
	}

	var decisions int
	if err := f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM core.review_decisions WHERE review_request_id = $1`, req.ID).Scan(&decisions); err != nil {
		t.Fatalf("підрахунок рішень: %v", err)
	}
	if decisions != 2 {
		t.Errorf("рішень = %d, очікувано 2 окремі записи", decisions)
	}
}

// «Потрібні зміни» повертають артефакт у draft із мотивованим записом,
// а не створюють окремий стан rejected (ADR-009 §3).
func TestRequestChangesReturnsToDraft(t *testing.T) {
	f := newSubmitFixture(t)
	req := f.submitted(t)

	decision, err := f.decide(t, f.reviewer, project.DecisionRequestChanges, "бракує критеріїв приймання", "")
	if err != nil {
		t.Fatalf("запит змін: %v", err)
	}
	if decision.Outcome != "changes_requested" {
		t.Errorf("результат = %q, очікувано changes_requested", decision.Outcome)
	}
	if got := f.wpStatus(t, f.workProductID); got != "draft" {
		t.Errorf("статус артефакту = %q, очікувано draft", got)
	}
	if got := f.requestStatus(t, req.ID); got != "changes_requested" {
		t.Errorf("статус запиту = %q, очікувано changes_requested", got)
	}
}

func TestRequestChangesWithoutReasonRejected(t *testing.T) {
	f := newSubmitFixture(t)
	f.submitted(t)

	_, err := f.decide(t, f.reviewer, project.DecisionRequestChanges, "", "")
	if !errors.Is(err, project.ErrReasonRequired) {
		t.Errorf("очікувано ErrReasonRequired, отримано %v", err)
	}
}

// Повтор із тим самим ключем операції повертає попередній результат, а не
// другий підпис (ADR-009 §4).
func TestDecisionIsIdempotentByOperationKey(t *testing.T) {
	f := newSubmitFixture(t)
	req := f.submitted(t)
	const opKey = "op-12345"

	first, err := f.decide(t, f.reviewer, project.DecisionReview, "", opKey)
	if err != nil {
		t.Fatalf("перше рішення: %v", err)
	}
	second, err := f.decide(t, f.reviewer, project.DecisionReview, "", opKey)
	if err != nil {
		t.Fatalf("повтор має повертати попередній результат, а не помилку: %v", err)
	}
	if first.ID != second.ID {
		t.Errorf("повтор створив нове рішення %s замість повернення %s", second.ID, first.ID)
	}

	var decisions int
	if err := f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM core.review_decisions WHERE review_request_id = $1`, req.ID).Scan(&decisions); err != nil {
		t.Fatalf("підрахунок рішень: %v", err)
	}
	if decisions != 1 {
		t.Errorf("рішень = %d, очікувано 1 — другий підпис не мав з'явитися", decisions)
	}
}

func TestDecisionOperationKeyCannotReplayAnotherActorOrProject(t *testing.T) {
	f := newSubmitFixture(t)
	f.submitted(t)
	const opKey = "shared-operation-key"
	first, err := f.decide(t, f.reviewer, project.DecisionReview, "", opKey)
	if err != nil {
		t.Fatal(err)
	}
	for _, attempt := range []struct {
		actorID   uuid.UUID
		projectID uuid.UUID
		kind      string
	}{
		{actorID: f.approver2, projectID: f.projectID, kind: project.DecisionApproval},
		{actorID: f.reviewer, projectID: uuid.New(), kind: project.DecisionReview},
	} {
		got, err := f.projects.RecordDecision(context.Background(), attempt.actorID, attempt.projectID,
			f.workProductID, attempt.kind, "", opKey)
		if !errors.Is(err, project.ErrDecisionOperationConflict) || got.ID == first.ID {
			t.Fatalf("повтор чужого ключа не повинен розкривати рішення: got=%+v err=%v", got, err)
		}
	}
}

func TestDecisionOperationKeyCannotReplayStaleRevision(t *testing.T) {
	f := newSubmitFixture(t)
	f.submitted(t)
	const opKey = "revision-operation-key"
	if _, err := f.decide(t, f.reviewer, project.DecisionReview, "", opKey); err != nil {
		t.Fatal(err)
	}
	f.addRevision(t, f.workProductID, "нова ревізія", f.actor)
	if _, err := f.decide(t, f.reviewer, project.DecisionReview, "", opKey); !errors.Is(err, project.ErrDecisionOperationConflict) {
		t.Fatalf("ключ попередньої ревізії не повинен повертати старе рішення: %v", err)
	}
}

func TestDecisionOperationKeyCannotReplayDifferentReason(t *testing.T) {
	f := newSubmitFixture(t)
	f.submitted(t)
	const opKey = "reason-operation-key"
	if _, err := f.decide(t, f.reviewer, project.DecisionRequestChanges, "Потрібні критерії", opKey); err != nil {
		t.Fatal(err)
	}
	if _, err := f.decide(t, f.reviewer, project.DecisionRequestChanges, "Потрібен інший доказ", opKey); !errors.Is(err, project.ErrDecisionOperationConflict) {
		t.Fatalf("ключ рішення з іншою причиною має конфліктувати: %v", err)
	}
}

// Рішення щодо артефакту, який уже не на рецензії, неможливе.
func TestDecisionRejectedWhenNotInReview(t *testing.T) {
	f := newSubmitFixture(t)

	_, err := f.decide(t, f.reviewer, project.DecisionReview, "", "")
	if !errors.Is(err, project.ErrNotInReview) {
		t.Errorf("очікувано ErrNotInReview, отримано %v", err)
	}
}

// Після погодження повторне рішення неможливе: запит закрито, артефакт
// вийшов зі стану in_review.
func TestNoDecisionsAfterApproval(t *testing.T) {
	f := newSubmitFixture(t)
	f.submitted(t)

	if _, err := f.decide(t, f.reviewer, project.DecisionReview, "", ""); err != nil {
		t.Fatalf("рецензія: %v", err)
	}
	if _, err := f.decide(t, f.approver2, project.DecisionApproval, "", ""); err != nil {
		t.Fatalf("погодження: %v", err)
	}

	_, err := f.decide(t, f.reviewer, project.DecisionRequestChanges, "запізніла думка", "")
	if !errors.Is(err, project.ErrNotInReview) {
		t.Errorf("очікувано ErrNotInReview після затвердження, отримано %v", err)
	}
}
