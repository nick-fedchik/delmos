package automation

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// RuleOutcome — результат оцінки правила (EVENTS_TRIGGERS_RULES.md §2.3,
// таблиця рішень).
type RuleOutcome string

const (
	OutcomeNotApplicable RuleOutcome = "not_applicable"
	OutcomePassed        RuleOutcome = "passed"
	OutcomeViolated      RuleOutcome = "violated"
	OutcomeEvaluationErr RuleOutcome = "evaluation_error"
	EnforcementVeto                  = "MANDATORY_VETO"
	EnforcementAdvisory              = "ADVISORY_WARNING"
)

// RuleViolationError — повертається EnforceRules, коли обов'язкове правило
// (MANDATORY_VETO) порушено або не обчислилося; викликач транслює це у
// відкат транзакції та HTTP 422 (SWR-13/14).
type RuleViolationError struct {
	RuleKey string
	Outcome RuleOutcome
	Reason  string
}

func (e *RuleViolationError) Error() string {
	return fmt.Sprintf("правило %s: %s (%s)", e.RuleKey, e.Outcome, e.Reason)
}

type ruleRow struct {
	RuleKey          string
	Priority         int
	EnforcementLevel string
	When             ConditionNode
	Assert           ConditionNode
}

func loadRules(ctx context.Context, tx pgx.Tx, triggerKey string) ([]ruleRow, error) {
	rows, err := tx.Query(ctx,
		`SELECT rule_key, priority, enforcement_level, when_condition, assert_condition
		 FROM core.rule_definitions WHERE trigger_key = $1 AND active ORDER BY priority ASC`,
		triggerKey)
	if err != nil {
		return nil, fmt.Errorf("читання правил тригера %s: %w", triggerKey, err)
	}
	defer rows.Close()

	var rules []ruleRow
	for rows.Next() {
		var r ruleRow
		var whenJSON, assertJSON []byte
		if err := rows.Scan(&r.RuleKey, &r.Priority, &r.EnforcementLevel, &whenJSON, &assertJSON); err != nil {
			return nil, fmt.Errorf("розбір правила тригера %s: %w", triggerKey, err)
		}
		if err := json.Unmarshal(whenJSON, &r.When); err != nil {
			return nil, fmt.Errorf("розбір when правила %s: %w", r.RuleKey, err)
		}
		if err := json.Unmarshal(assertJSON, &r.Assert); err != nil {
			return nil, fmt.Errorf("розбір assert правила %s: %w", r.RuleKey, err)
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// EnforceRules обчислює всі активні правила заданого тригера в межах
// переданої транзакції (before-фаза, in_transaction — EVENTS_TRIGGERS_RULES.md
// §2.2). ADVISORY_WARNING-порушення пишуться в audit_events і не блокують;
// перше порушене чи неоцінене MANDATORY_VETO-правило повертається як помилка,
// що має відкотити транзакцію (SWR-13/14).
func EnforceRules(ctx context.Context, tx pgx.Tx, triggerKey string, evalCtx EvalContext, actorID uuid.UUID, projectID uuid.UUID, correlationID uuid.UUID) error {
	rules, err := loadRules(ctx, tx, triggerKey)
	if err != nil {
		return err
	}

	for _, rule := range rules {
		applicable, err := Evaluate(evalCtx, rule.When)
		if err != nil {
			if rule.EnforcementLevel == EnforcementVeto {
				return &RuleViolationError{RuleKey: rule.RuleKey, Outcome: OutcomeEvaluationErr, Reason: err.Error()}
			}
			continue // ADVISORY_WARNING з помилкою when не блокує (SWR-14)
		}
		if !applicable {
			continue // not_applicable
		}

		satisfied, err := Evaluate(evalCtx, rule.Assert)
		outcome := OutcomePassed
		reason := ""
		switch {
		case err != nil:
			outcome, reason = OutcomeEvaluationErr, err.Error()
		case !satisfied:
			outcome, reason = OutcomeViolated, "assert не виконано"
		}

		if outcome == OutcomePassed {
			continue
		}

		if rule.EnforcementLevel == EnforcementVeto {
			return &RuleViolationError{RuleKey: rule.RuleKey, Outcome: outcome, Reason: reason}
		}

		// ADVISORY_WARNING: аудиторське зауваження, операція триває.
		if _, err := tx.Exec(ctx,
			`INSERT INTO core.audit_events (actor_user_id, action, scope_type, scope_id, outcome, detail, correlation_id)
			 VALUES ($1, 'rule.violated', 'project', $2, 'success', $3, $4)`,
			actorID, projectID, map[string]any{"rule_key": rule.RuleKey, "outcome": string(outcome), "reason": reason}, correlationID); err != nil {
			return fmt.Errorf("запис аудиторського зауваження правила %s: %w", rule.RuleKey, err)
		}
	}
	return nil
}
