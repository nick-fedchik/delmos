package automation

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// WorkflowState — вузол декларативної FSM-схеми (VECTOR_AND_GRAPH_DATA.md §3.4
// приклад JSONB, ENTITY_CATALOG.md: WorkflowDefinition).
type WorkflowState struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Category string `json:"category"`
}

// WorkflowEffect — побічна дія переходу; наразі підтримується лише
// "emit_event" (публікація події в outbox після успішного переходу).
type WorkflowEffect struct {
	Type  string `json:"type"`
	Event string `json:"event,omitempty"`
}

// WorkflowTransition — дозволене ребро FSM з охоронцями (guards) та ефектами.
type WorkflowTransition struct {
	ID         string           `json:"id"`
	From       string           `json:"from"`
	To         string           `json:"to"`
	ActionName string           `json:"action_name"`
	Guards     []ConditionNode  `json:"guards"`
	Effects    []WorkflowEffect `json:"effects"`
}

// WorkflowDefinition — повна декларативна схема життєвого циклу сутності.
type WorkflowDefinition struct {
	InitialState string               `json:"initial_state"`
	States       []WorkflowState      `json:"states"`
	Transitions  []WorkflowTransition `json:"transitions"`
}

// ErrTransitionNotDefined — переходу від from до to немає у FSM-схемі.
var ErrTransitionNotDefined = fmt.Errorf("перехід не визначено у workflow-схемі")

// FindTransition шукає перше визначене ребро from -> to.
func (wf WorkflowDefinition) FindTransition(from, to string) (*WorkflowTransition, bool) {
	for i := range wf.Transitions {
		if wf.Transitions[i].From == from && wf.Transitions[i].To == to {
			return &wf.Transitions[i], true
		}
	}
	return nil, false
}

// queryRower — спільний інтерфейс *pgxpool.Pool і pgx.Tx для читання без
// прив'язки LoadWorkflow до конкретного джерела з'єднання.
type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// LoadWorkflow читає опубліковану FSM-схему за ключем і версією.
func LoadWorkflow(ctx context.Context, q queryRower, key, version string) (WorkflowDefinition, error) {
	var definitionJSON []byte
	err := q.QueryRow(ctx,
		`SELECT definition FROM core.workflow_definitions WHERE key = $1 AND version = $2 AND is_published`,
		key, version).Scan(&definitionJSON)
	if err != nil {
		return WorkflowDefinition{}, fmt.Errorf("читання workflow %s@%s: %w", key, version, err)
	}
	var def WorkflowDefinition
	if err := json.Unmarshal(definitionJSON, &def); err != nil {
		return WorkflowDefinition{}, fmt.Errorf("розбір workflow %s@%s: %w", key, version, err)
	}
	return def, nil
}

// ApplyTransition перевіряє наявність переходу й усіх його охоронців, після
// чого публікує ефекти emit_event у тій самій транзакції (SWR-14 derive/guard
// pattern). Повертає ErrTransitionNotDefined або першу помилку охоронця.
func ApplyTransition(ctx context.Context, tx pgx.Tx, wf WorkflowDefinition, from, to string, evalCtx EvalContext, projectID *uuid.UUID, actorUserID *uuid.UUID, correlationID uuid.UUID) error {
	transition, found := wf.FindTransition(from, to)
	if !found {
		return ErrTransitionNotDefined
	}
	for _, guard := range transition.Guards {
		ok, err := Evaluate(evalCtx, guard)
		if err != nil {
			return fmt.Errorf("перехід %s: помилка охоронця %s: %w", transition.ID, guard.PredicateKey, err)
		}
		if !ok {
			return fmt.Errorf("перехід %s: охоронець %s не виконано", transition.ID, guard.PredicateKey)
		}
	}
	for _, effect := range transition.Effects {
		if effect.Type != "emit_event" || effect.Event == "" {
			continue
		}
		if err := EmitEvent(ctx, tx, effect.Event, projectID, actorUserID, correlationID, map[string]any{
			"transition_id": transition.ID, "from": from, "to": to,
		}); err != nil {
			return err
		}
	}
	return nil
}
