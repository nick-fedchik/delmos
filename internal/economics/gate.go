package economics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Ключі полів, які шлюз фази передає предикатам правил. Виведені окремими
// константами, бо збігатися мають три місця: цей розрахунок, засів правил у
// міграції 0014 і реєстр предикатів.
const (
	FieldTargetStatus = "target_status"
	FieldPhaseKey     = "phase_key"
	FieldActualCost   = "actual_cost"
	FieldFundingLimit = "funding_limit"
	FieldCPI          = "cpi"
)

// PhaseGateFacts рахує фактичні витрати, ліміт фінансування та CPI однієї
// фази в межах переданої транзакції.
//
// Розрахунок навмисно виконується в тій самій транзакції, що й сам перехід
// фази: інакше між перевіркою ліміту та зміною статусу могло б з'явитися нове
// списання, і шлюз пропустив би фазу, яка вже вийшла за бюджет.
//
// Якщо затвердженого кошторису немає, повертається порожній набір фактів:
// перевіряти дотримання ліміту, якого не існує, беззмістовно.
func PhaseGateFacts(ctx context.Context, tx pgx.Tx, projectID uuid.UUID, phaseKey string, asOf time.Time) (map[string]any, error) {
	facts := map[string]any{FieldPhaseKey: phaseKey}

	var baselineID uuid.UUID
	err := tx.QueryRow(ctx,
		`SELECT id FROM core.cost_baselines
		 WHERE project_id = $1 AND status = 'approved'`, projectID).Scan(&baselineID)
	if errors.Is(err, pgx.ErrNoRows) {
		return facts, nil
	}
	if err != nil {
		return nil, fmt.Errorf("читання затвердженого кошторису: %w", err)
	}

	var actualCost, fundingLimit, earnedValue string
	err = tx.QueryRow(ctx,
		`WITH `+phaseActualCostSQL+`,
		 phase_budget AS (
		     SELECT COALESCE(SUM(planned_amount), 0) AS planned_total,
		            COALESCE(SUM(funding_limit), 0) AS funding_limit
		     FROM core.budget_lines
		     WHERE cost_baseline_id = $3 AND phase_key = $4
		 ),
		 phase_progress AS (
		     SELECT COALESCE(SUM(pd.weight) FILTER (WHERE wp.status = 'approved'), 0) AS earned_weight,
		            COALESCE(SUM(pd.weight), 0) AS total_weight
		     FROM core.phase_deliverables pd
		     JOIN core.work_products wp ON wp.id = pd.work_product_id
		     WHERE pd.project_id = $1 AND pd.phase_key = $4
		 )
		 SELECT COALESCE((SELECT amount FROM actual_cost WHERE phase_key = $4), 0)::numeric(14, 2)::text,
		        pb.funding_limit::numeric(14, 2)::text,
		        (pb.planned_total * COALESCE(
		            pp.earned_weight / NULLIF(pp.total_weight, 0), 0
		        ))::numeric(14, 2)::text
		 FROM phase_budget pb, phase_progress pp`,
		projectID, asOf, baselineID, phaseKey).
		Scan(&actualCost, &fundingLimit, &earnedValue)
	if err != nil {
		return nil, fmt.Errorf("розрахунок фактів фази %s: %w", phaseKey, err)
	}

	facts[FieldActualCost] = actualCost
	facts[FieldFundingLimit] = fundingLimit

	// CPI не визначений за нульових витрат. Поле просто відсутнє — нуль тут
	// означав би «індекс дорівнює нулю», тобто повну перевитрату (METRICS.md).
	var cpi *string
	if err := tx.QueryRow(ctx,
		`SELECT CASE WHEN $1::numeric = 0 THEN NULL
		             ELSE round($2::numeric / $1::numeric, 4)::text END`,
		actualCost, earnedValue).Scan(&cpi); err != nil {
		return nil, fmt.Errorf("розрахунок CPI фази %s: %w", phaseKey, err)
	}
	if cpi != nil {
		facts[FieldCPI] = *cpi
	}
	return facts, nil
}
