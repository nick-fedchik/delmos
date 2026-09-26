package automation_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"delmos/internal/auth"
	"delmos/internal/automation"
)

// TestEnforceRulesMandatoryVetoBlocks перевіряє посіяне правило
// rule.core.no_revision_when_obsolete (0013_automation_engine.sql):
// MANDATORY_VETO-порушення повертає RuleViolationError.
func TestEnforceRulesMandatoryVetoBlocks(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("початок транзакції: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	actorID, projectID := uuid.New(), uuid.New()

	err = automation.EnforceRules(ctx, tx, "trigger.core.before_wp_transition",
		automation.EvalContext{Fields: map[string]any{"status": "obsolete"}}, actorID, projectID, uuid.New())
	var violation *automation.RuleViolationError
	if !errors.As(err, &violation) {
		t.Fatalf("очікувалася *RuleViolationError, отримано %v", err)
	}
	if violation.RuleKey != "rule.core.no_revision_when_obsolete" || violation.Outcome != automation.OutcomeViolated {
		t.Fatalf("несподіване правило порушення: %+v", violation)
	}
}

// TestEnforceRulesPassesWhenNotApplicableOrSatisfied перевіряє, що чинний
// статус не блокується (assert виконано — passed).
func TestEnforceRulesPassesWhenNotApplicableOrSatisfied(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("початок транзакції: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	err = automation.EnforceRules(ctx, tx, "trigger.core.before_wp_transition",
		automation.EvalContext{Fields: map[string]any{"status": "draft"}}, uuid.New(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("статус draft не мав порушувати правило: %v", err)
	}
}

// TestEnforceRulesAdvisoryWarningLogsAuditAndContinues перевіряє
// ADVISORY_WARNING: порушення не блокує операцію, але пишеться в audit_events
// з action='rule.violated'.
func TestEnforceRulesAdvisoryWarningLogsAuditAndContinues(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)

	const triggerKey = "trigger.test.advisory_demo"
	if _, err := pool.Exec(ctx,
		`INSERT INTO core.trigger_definitions (trigger_key, phase, input_kind, execution_mode) VALUES ($1, 'before', 'command', 'in_transaction')`,
		triggerKey); err != nil {
		t.Fatalf("реєстрація тестового тригера: %v", err)
	}
	assertCondition, _ := json.Marshal(automation.ConditionNode{PredicateKey: "always_false"})
	if _, err := pool.Exec(ctx,
		`INSERT INTO core.rule_definitions (rule_key, trigger_key, enforcement_level, assert_condition) VALUES ($1, $2, 'ADVISORY_WARNING', $3)`,
		"rule.test.advisory", triggerKey, assertCondition); err != nil {
		t.Fatalf("реєстрація тестового правила: %v", err)
	}

	// audit_events.actor_user_id має FK на core.users — потрібен реальний користувач.
	authStore := auth.NewStore(pool)
	hash, err := auth.HashPassword("irrelevant-password")
	if err != nil {
		t.Fatalf("хешування пароля: %v", err)
	}
	actorID, err := authStore.BootstrapAdministrator(ctx, "advisory-actor", "advisory-actor", hash)
	if err != nil {
		t.Fatalf("створення тестового користувача: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("початок транзакції: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	err = automation.EnforceRules(ctx, tx, triggerKey, automation.EvalContext{}, actorID, uuid.Nil, uuid.New())
	if err != nil {
		t.Fatalf("ADVISORY_WARNING не мав блокувати операцію: %v", err)
	}

	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM core.audit_events WHERE action = 'rule.violated'`).Scan(&count); err != nil {
		t.Fatalf("підрахунок аудиторських подій: %v", err)
	}
	if count != 1 {
		t.Fatalf("очікувався 1 запис аудиту rule.violated, отримано %d", count)
	}
}
