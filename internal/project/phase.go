package project

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"delmos/internal/automation"
	"delmos/internal/economics"
	"delmos/internal/metrics"
)

var (
	ErrPhaseNotFound          = errors.New("фазу не знайдено в проєкті")
	ErrPhaseTransitionInvalid = errors.New("неприпустимий перехід статусу фази")
	ErrPhaseGateRejected      = errors.New("перехід заблоковано обов'язковим правилом")
)

// allowedPhaseTransitions — життєвий цикл фази (PROJECT_PLAN_ENGINE.md).
// Зворотних переходів немає свідомо: завершена фаза вже врахована в
// показниках здобутої цінності, і її повторне відкриття переписало б
// підсумки, на які спирався шлюз.
var allowedPhaseTransitions = map[string][]string{
	"not_started": {"active"},
	"active":      {"completed"},
	"completed":   {},
}

// PhaseGateWarning — зауваження ADVISORY_WARNING, зафіксоване під час переходу.
type PhaseTransitionResult struct {
	PhaseKey string `json:"phase_key"`
	From     string `json:"from_status"`
	To       string `json:"to_status"`
}

// TransitionPhase переводить фазу в новий статус, попередньо обчисливши
// економічні факти й пропустивши їх через рушій правил (SHR-13 критерій 5).
//
// Порядок навмисний: блокування рядка фази -> перевірка дозволеності переходу
// -> розрахунок фактів -> правила -> запис. Правила оцінюються в тій самій
// транзакції, тож між перевіркою ліміту фінансування та зміною статусу не
// може вклинитися нове списання.
func (s *Store) TransitionPhase(ctx context.Context, actorID, projectID uuid.UUID, phaseKey, targetStatus string, asOf time.Time) (PhaseTransitionResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PhaseTransitionResult{}, fmt.Errorf("відкриття транзакції переходу фази: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var currentStatus string
	err = tx.QueryRow(ctx,
		`SELECT status FROM core.project_phases
		 WHERE project_id = $1 AND phase_key = $2 FOR UPDATE`, projectID, phaseKey).
		Scan(&currentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return PhaseTransitionResult{}, ErrPhaseNotFound
	}
	if err != nil {
		return PhaseTransitionResult{}, fmt.Errorf("читання статусу фази: %w", err)
	}

	if !phaseTransitionAllowed(currentStatus, targetStatus) {
		return PhaseTransitionResult{}, fmt.Errorf("%w: %s -> %s", ErrPhaseTransitionInvalid, currentStatus, targetStatus)
	}

	facts, err := economics.PhaseGateFacts(ctx, tx, projectID, phaseKey, asOf)
	if err != nil {
		return PhaseTransitionResult{}, err
	}
	facts[economics.FieldTargetStatus] = targetStatus

	// Якість метрик перевіряється в тій самій транзакції: інакше між
	// перевіркою свіжості та зміною статусу могло б з'явитися нове
	// вимірювання (SWR-22.3).
	blockers, err := metrics.GateBlockers(ctx, tx, projectID, phaseKey, asOf)
	if err != nil {
		return PhaseTransitionResult{}, err
	}
	facts[automation.FieldMetricBlockers] = blockers

	evalCtx := automation.EvalContext{Fields: facts, Permissions: map[string]bool{}}
	correlationID := uuid.New()
	if err := automation.EnforceRules(ctx, tx, "trigger.core.before_phase_transition",
		evalCtx, actorID, projectID, correlationID); err != nil {
		var violation *automation.RuleViolationError
		if errors.As(err, &violation) {
			return PhaseTransitionResult{}, fmt.Errorf("%w: %s", ErrPhaseGateRejected, violation.Error())
		}
		return PhaseTransitionResult{}, err
	}

	if _, err := tx.Exec(ctx,
		`UPDATE core.project_phases SET status = $3
		 WHERE project_id = $1 AND phase_key = $2`, projectID, phaseKey, targetStatus); err != nil {
		return PhaseTransitionResult{}, fmt.Errorf("зміна статусу фази: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO core.audit_events (actor_user_id, action, scope_type, scope_id, outcome, detail, correlation_id)
		 VALUES ($1, 'phase.transitioned', 'project', $2, 'success', $3, $4)`,
		actorID, projectID, map[string]any{
			"phase_key":   phaseKey,
			"from_status": currentStatus,
			"to_status":   targetStatus,
		}, correlationID); err != nil {
		return PhaseTransitionResult{}, fmt.Errorf("аудит переходу фази: %w", err)
	}

	if err := automation.EmitEvent(ctx, tx, "phase.transitioned", &projectID, &actorID, correlationID, map[string]any{
		"phase_key":   phaseKey,
		"from_status": currentStatus,
		"to_status":   targetStatus,
	}); err != nil {
		return PhaseTransitionResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return PhaseTransitionResult{}, fmt.Errorf("фіксація переходу фази: %w", err)
	}
	return PhaseTransitionResult{PhaseKey: phaseKey, From: currentStatus, To: targetStatus}, nil
}

func phaseTransitionAllowed(from, to string) bool {
	for _, allowed := range allowedPhaseTransitions[from] {
		if allowed == to {
			return true
		}
	}
	return false
}
