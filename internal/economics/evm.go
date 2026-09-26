// Package economics реалізує проєктну економіку та метод здобутої цінності
// (Earned Value Management) за ДСТУ ISO 21508:2022 — ADR-006, ADR-011, SWR-41.
//
// Усі грошові обчислення виконуються типом NUMERIC на боці PostgreSQL і
// повертаються рядками. Float ніде не застосовується: у кошторисі та
// показниках для аудиту втрата точності неприйнятна.
package economics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNoApprovedBaseline = errors.New("для проєкту немає затвердженого базового кошторису")
	ErrPhaseNotFound      = errors.New("фазу не знайдено в проєкті")
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// EarnedValueSnapshot — показники здобутої цінності на контрольну дату.
// Грошові величини подано рядками в десятковому записі, щоб між PostgreSQL
// та клієнтом не виникало проміжного двійкового представлення.
type EarnedValueSnapshot struct {
	ProjectID   uuid.UUID `json:"project_id"`
	AsOf        time.Time `json:"as_of"`
	Currency    string    `json:"currency"`
	BaselineID  uuid.UUID `json:"baseline_id"`
	BaseVersion int       `json:"baseline_version"`

	BudgetAtCompletion string `json:"bac"`
	PlannedValue       string `json:"pv"`
	ActualCost         string `json:"ac"`
	EarnedValue        string `json:"ev"`
	CostVariance       string `json:"cv"`
	ScheduleVariance   string `json:"sv"`

	// Індекси відсутні (nil), коли знаменник нульовий. Нуль тут означав би
	// «показник дорівнює нулю», що хибно: правильна семантика — «немає
	// даних для обчислення» (METRICS.md §6, заборона трактувати no_data як 0).
	CostPerformanceIndex     *string `json:"cpi"`
	SchedulePerformanceIndex *string `json:"spi"`
	EstimateAtCompletion     *string `json:"eac"`

	Phases []PhaseEarnedValue `json:"phases"`
}

type PhaseEarnedValue struct {
	PhaseKey     string `json:"phase_key"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	PlannedTotal string `json:"planned_total"`
	FundingLimit string `json:"funding_limit"`
	PlannedValue string `json:"pv"`
	ActualCost   string `json:"ac"`
	EarnedValue  string `json:"ev"`
	OverLimit    bool   `json:"over_limit"`
}

// Спільний вираз фактичних витрат фази: прямі трудовитрати за чинною на дату
// роботи ставкою плюс нетрудові витрати. LATERAL-підзапит обирає саме ту
// версію ставки, що діяла в день виконання роботи, а не поточну — інакше
// зміна ставки заднім числом переписала б уже пораховану історію.
const phaseActualCostSQL = `
actual_cost AS (
    SELECT phase_key, SUM(amount) AS amount FROM (
        SELECT wr.phase_key,
               wr.duration_hours * COALESCE(rate.hourly_rate, 0) AS amount
        FROM core.work_records wr
        LEFT JOIN LATERAL (
            SELECT lr.hourly_rate
            FROM core.labor_rates lr
            WHERE lr.role_key = wr.role_key
              AND (lr.project_id = wr.project_id OR lr.project_id IS NULL)
              AND lr.valid_from <= wr.work_date
              AND (lr.valid_to IS NULL OR lr.valid_to > wr.work_date)
            ORDER BY lr.project_id NULLS LAST
            LIMIT 1
        ) rate ON true
        WHERE wr.project_id = $1 AND wr.work_date <= $2

        UNION ALL

        SELECT er.phase_key, er.amount
        FROM core.expense_records er
        WHERE er.project_id = $1 AND er.expense_date <= $2
    ) costs
    GROUP BY phase_key
)`

// ComputeEarnedValue розраховує показники за ISO 21508 на дату asOf.
//
// Методи вимірювання задекларовано явно:
//   - PV: лінійне календарне рознесення планової вартості фази за
//     planned_start..planned_finish (time-phasing).
//   - EV: правило 0/100 за результатами фази — результат зараховується лише
//     після переходу в approved, часткової готовності не буває.
func (s *Store) ComputeEarnedValue(ctx context.Context, projectID uuid.UUID, asOf time.Time) (EarnedValueSnapshot, error) {
	snapshot := EarnedValueSnapshot{ProjectID: projectID, AsOf: asOf}

	err := s.pool.QueryRow(ctx,
		`SELECT id, version, currency FROM core.cost_baselines
		 WHERE project_id = $1 AND status = 'approved'`, projectID).
		Scan(&snapshot.BaselineID, &snapshot.BaseVersion, &snapshot.Currency)
	if errors.Is(err, pgx.ErrNoRows) {
		return EarnedValueSnapshot{}, ErrNoApprovedBaseline
	}
	if err != nil {
		return EarnedValueSnapshot{}, fmt.Errorf("читання затвердженого кошторису: %w", err)
	}

	query := `
WITH ` + phaseActualCostSQL + `,
phase_budget AS (
    SELECT bl.phase_key,
           SUM(bl.planned_amount) AS planned_total,
           SUM(bl.funding_limit) AS funding_limit
    FROM core.budget_lines bl
    WHERE bl.cost_baseline_id = $3
    GROUP BY bl.phase_key
),
-- Частка здобутої цінності фази за правилом 0/100: вага затверджених
-- результатів до загальної ваги результатів фази.
phase_progress AS (
    SELECT pd.phase_key,
           SUM(pd.weight) FILTER (WHERE wp.status = 'approved') AS earned_weight,
           SUM(pd.weight) AS total_weight
    FROM core.phase_deliverables pd
    JOIN core.work_products wp ON wp.id = pd.work_product_id
    WHERE pd.project_id = $1
    GROUP BY pd.phase_key
)
SELECT ph.phase_key,
       ph.name,
       ph.status,
       COALESCE(pb.planned_total, 0)::text,
       COALESCE(pb.funding_limit, 0)::text,
       -- Лінійне рознесення PV за календарем фази.
       (COALESCE(pb.planned_total, 0) * CASE
           WHEN ph.planned_start IS NULL OR ph.planned_finish IS NULL THEN 0
           WHEN $2::date < ph.planned_start THEN 0
           WHEN $2::date >= ph.planned_finish THEN 1
           WHEN ph.planned_finish = ph.planned_start THEN 1
           ELSE ($2::date - ph.planned_start)::numeric
                / NULLIF((ph.planned_finish - ph.planned_start)::numeric, 0)
       END)::numeric(14, 2)::text,
       COALESCE(ac.amount, 0)::numeric(14, 2)::text,
       (COALESCE(pb.planned_total, 0) * COALESCE(
           pp.earned_weight / NULLIF(pp.total_weight, 0), 0
       ))::numeric(14, 2)::text
FROM core.project_phases ph
LEFT JOIN phase_budget pb ON pb.phase_key = ph.phase_key
LEFT JOIN actual_cost ac ON ac.phase_key = ph.phase_key
LEFT JOIN phase_progress pp ON pp.phase_key = ph.phase_key
WHERE ph.project_id = $1
ORDER BY ph.planned_start NULLS LAST, ph.phase_key`

	rows, err := s.pool.Query(ctx, query, projectID, asOf, snapshot.BaselineID)
	if err != nil {
		return EarnedValueSnapshot{}, fmt.Errorf("розрахунок здобутої цінності: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var p PhaseEarnedValue
		if err := rows.Scan(&p.PhaseKey, &p.Name, &p.Status,
			&p.PlannedTotal, &p.FundingLimit, &p.PlannedValue, &p.ActualCost, &p.EarnedValue); err != nil {
			return EarnedValueSnapshot{}, fmt.Errorf("розбір показників фази: %w", err)
		}
		snapshot.Phases = append(snapshot.Phases, p)
	}
	if err := rows.Err(); err != nil {
		return EarnedValueSnapshot{}, fmt.Errorf("обхід показників фаз: %w", err)
	}

	if err := s.aggregate(ctx, &snapshot); err != nil {
		return EarnedValueSnapshot{}, err
	}
	return snapshot, nil
}

// aggregate підсумовує показники проєкту та похідні індекси засобами NUMERIC.
func (s *Store) aggregate(ctx context.Context, snapshot *EarnedValueSnapshot) error {
	var pv, ac, ev, bac, cv, sv string
	var cpi, spi, eac *string

	// Підсумки беруться з уже порахованих фазових значень, щоб проєктні та
	// фазові показники не розходилися через різні шляхи обчислення.
	err := s.pool.QueryRow(ctx,
		`WITH phase AS (
		    SELECT unnest($1::numeric[]) AS pv,
		           unnest($2::numeric[]) AS ac,
		           unnest($3::numeric[]) AS ev
		 ), total AS (
		    SELECT COALESCE(SUM(pv), 0) AS pv,
		           COALESCE(SUM(ac), 0) AS ac,
		           COALESCE(SUM(ev), 0) AS ev
		    FROM phase
		 ), budget AS (
		    SELECT COALESCE(SUM(planned_amount), 0) AS bac
		    FROM core.budget_lines WHERE cost_baseline_id = $4
		 )
		 SELECT t.pv::text, t.ac::text, t.ev::text, b.bac::text,
		        (t.ev - t.ac)::text,
		        (t.ev - t.pv)::text,
		        CASE WHEN t.ac = 0 THEN NULL ELSE round(t.ev / t.ac, 4)::text END,
		        CASE WHEN t.pv = 0 THEN NULL ELSE round(t.ev / t.pv, 4)::text END,
		        CASE WHEN t.ev = 0 THEN NULL
		             ELSE round(b.bac / (t.ev / NULLIF(t.ac, 0)), 2)::text END
		 FROM total t, budget b`,
		phaseField(snapshot.Phases, func(p PhaseEarnedValue) string { return p.PlannedValue }),
		phaseField(snapshot.Phases, func(p PhaseEarnedValue) string { return p.ActualCost }),
		phaseField(snapshot.Phases, func(p PhaseEarnedValue) string { return p.EarnedValue }),
		snapshot.BaselineID,
	).Scan(&pv, &ac, &ev, &bac, &cv, &sv, &cpi, &spi, &eac)
	if err != nil {
		return fmt.Errorf("підсумок показників проєкту: %w", err)
	}

	snapshot.PlannedValue = pv
	snapshot.ActualCost = ac
	snapshot.EarnedValue = ev
	snapshot.BudgetAtCompletion = bac
	snapshot.CostVariance = cv
	snapshot.ScheduleVariance = sv
	snapshot.CostPerformanceIndex = cpi
	snapshot.SchedulePerformanceIndex = spi
	snapshot.EstimateAtCompletion = eac

	for i := range snapshot.Phases {
		over, err := s.exceedsLimit(ctx, snapshot.Phases[i].ActualCost, snapshot.Phases[i].FundingLimit)
		if err != nil {
			return err
		}
		snapshot.Phases[i].OverLimit = over
	}
	return nil
}

// exceedsLimit порівнює суми в NUMERIC: переведення у float для порівняння
// грошей дало б хибний результат на межі (наприклад 0.1+0.2 > 0.3).
func (s *Store) exceedsLimit(ctx context.Context, actual, limit string) (bool, error) {
	var over bool
	if err := s.pool.QueryRow(ctx,
		`SELECT $1::numeric > $2::numeric AND $2::numeric > 0`, actual, limit).Scan(&over); err != nil {
		return false, fmt.Errorf("порівняння з лімітом фінансування: %w", err)
	}
	return over, nil
}

func phaseField(phases []PhaseEarnedValue, pick func(PhaseEarnedValue) string) []string {
	values := make([]string, 0, len(phases))
	for _, p := range phases {
		values = append(values, pick(p))
	}
	return values
}
