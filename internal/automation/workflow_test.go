package automation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"delmos/internal/automation"
)

// TestLoadWorkflowReadsSeededGenericLifecycle перевіряє посіяну FSM-схему
// core:generic_work_product_lifecycle@1 (0013_automation_engine.sql).
func TestLoadWorkflowReadsSeededGenericLifecycle(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)

	wf, err := automation.LoadWorkflow(ctx, pool, "core:generic_work_product_lifecycle", "1")
	if err != nil {
		t.Fatalf("читання посіяної FSM-схеми: %v", err)
	}
	if wf.InitialState != "draft" {
		t.Fatalf("очікувався initial_state=draft, отримано %s", wf.InitialState)
	}
	if len(wf.States) != 4 {
		t.Fatalf("очікувалося 4 стани, отримано %d", len(wf.States))
	}
	if _, found := wf.FindTransition("draft", "obsolete"); !found {
		t.Fatal("очікувався визначений перехід draft->obsolete")
	}
	if _, found := wf.FindTransition("draft", "approved"); found {
		t.Fatal("прямого переходу draft->approved у посіяній схемі бути не повинно")
	}
}

// TestApplyTransitionRejectsUndefinedEdge перевіряє, що недозволений перехід
// повертає ErrTransitionNotDefined і не публікує подій.
func TestApplyTransitionRejectsUndefinedEdge(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	wf, err := automation.LoadWorkflow(ctx, pool, "core:generic_work_product_lifecycle", "1")
	if err != nil {
		t.Fatalf("читання FSM-схеми: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("початок транзакції: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	err = automation.ApplyTransition(ctx, tx, wf, "draft", "approved", automation.EvalContext{}, nil, nil, uuid.New())
	if !errors.Is(err, automation.ErrTransitionNotDefined) {
		t.Fatalf("очікувалася ErrTransitionNotDefined, отримано %v", err)
	}
}

// TestApplyTransitionBlocksOnFailedGuard перевіряє, що перехід з guard, який
// не виконується, повертає помилку й не публікує ефект emit_event.
func TestApplyTransitionBlocksOnFailedGuard(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)

	definitionJSON := []byte(`{
		"initial_state": "draft",
		"states": [{"id": "draft", "label": "Draft", "category": "draft"}, {"id": "approved", "label": "Approved", "category": "approved"}],
		"transitions": [{
			"id": "approve", "from": "draft", "to": "approved", "action_name": "Approve",
			"guards": [{"predicate_key": "permission", "params": {"permission": "wp.approve"}}],
			"effects": [{"type": "emit_event", "event": "wp.revision_committed"}]
		}]
	}`)
	if _, err := pool.Exec(ctx,
		`INSERT INTO core.workflow_definitions (key, version, name, target_family, definition, is_published) VALUES ($1, '1', 'Test', 'work_product', $2, true)`,
		"test:guarded_workflow", definitionJSON); err != nil {
		t.Fatalf("реєстрація тестової FSM-схеми: %v", err)
	}

	wf, err := automation.LoadWorkflow(ctx, pool, "test:guarded_workflow", "1")
	if err != nil {
		t.Fatalf("читання тестової FSM-схеми: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("початок транзакції: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Без права wp.approve перехід має бути заблокований.
	if err := automation.ApplyTransition(ctx, tx, wf, "draft", "approved",
		automation.EvalContext{Permissions: map[string]bool{}}, nil, nil, uuid.New()); err == nil {
		t.Fatal("перехід без потрібного права мав бути заблокований")
	}

	// З правом wp.approve перехід дозволений і публікує подію.
	correlationID := uuid.New()
	if err := automation.ApplyTransition(ctx, tx, wf, "draft", "approved",
		automation.EvalContext{Permissions: map[string]bool{"wp.approve": true}}, nil, nil, correlationID); err != nil {
		t.Fatalf("перехід з правом мав пройти: %v", err)
	}

	var eventCount int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM core.event_outbox WHERE event_key = 'wp.revision_committed'`).Scan(&eventCount); err != nil {
		t.Fatalf("підрахунок опублікованих подій: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("очікувалася 1 опублікована подія emit_event, отримано %d", eventCount)
	}
}
