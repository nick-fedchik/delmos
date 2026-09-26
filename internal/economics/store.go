package economics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"delmos/internal/automation"
)

var (
	ErrRatePeriodOverlap  = errors.New("для цієї ролі вже є ставка на вказаний період")
	ErrBaselineApproved   = errors.New("затверджений кошторис незмінний")
	ErrSelfApproval       = errors.New("складач кошторису не може його затверджувати")
	ErrBaselineNotFound   = errors.New("версію кошторису не знайдено")
	ErrPhaseNotOpen       = errors.New("списання дозволене лише у відкритій фазі")
	ErrDuplicateApproved  = errors.New("у проєкті вже є затверджений кошторис")
	ErrUnknownCostategory = errors.New("невідома категорія витрат")
	// ErrCurrencyMismatch: AC підсумовує трудовитрати й витрати без конвертації
	// (evm.go, phaseActualCostSQL). Витрата в іншій валюті, ніж кошторис, тихо
	// змішала б дві різні валюти як одне число.
	ErrCurrencyMismatch = errors.New("валюта витрати не відповідає валюті затвердженого кошторису")
)

type WorkRecord struct {
	ID            uuid.UUID  `json:"id"`
	ProjectID     uuid.UUID  `json:"project_id"`
	UserID        uuid.UUID  `json:"user_id"`
	WorkProductID *uuid.UUID `json:"work_product_id,omitempty"`
	PhaseKey      string     `json:"phase_key"`
	RoleKey       string     `json:"role_key"`
	WorkDate      time.Time  `json:"work_date"`
	DurationHours string     `json:"duration_hours"`
	WorkCategory  string     `json:"work_category"`
	Comment       string     `json:"comment,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// emitInputsChanged сповіщає про зміну вхідних даних здобутої цінності.
// Подія записується в тій самій транзакції, що й сама зміна: інакше збій
// після фіксації запису лишив би показники назавжди розбіжними з даними.
//
// actorID може бути uuid.Nil (наприклад, для загальносистемної ставки без
// одного проєкту): тоді в конверт потрапляє відсутній актор, а не
// вигаданий нульовий UUID.
func emitInputsChanged(ctx context.Context, tx pgx.Tx, projectID, actorID uuid.UUID, reason string) error {
	var actor *uuid.UUID
	if actorID != uuid.Nil {
		actor = &actorID
	}
	return automation.EmitEvent(ctx, tx, "economics.inputs_changed", &projectID, actor, uuid.New(),
		map[string]any{"reason": reason})
}

// LogWorkRecord фіксує відпрацьовані години. Списання дозволене лише у фазі
// зі статусом active (SWR-39.1): завершена фаза вже врахована в показниках,
// і дозапис до неї заднім числом змінив би вже подані звіти.
func (s *Store) LogWorkRecord(ctx context.Context, rec WorkRecord) (WorkRecord, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return WorkRecord{}, fmt.Errorf("відкриття транзакції трудовитрат: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var phaseStatus string
	err = tx.QueryRow(ctx,
		`SELECT status FROM core.project_phases WHERE project_id = $1 AND phase_key = $2`,
		rec.ProjectID, rec.PhaseKey).Scan(&phaseStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkRecord{}, ErrPhaseNotFound
	}
	if err != nil {
		return WorkRecord{}, fmt.Errorf("читання фази: %w", err)
	}
	if phaseStatus != "active" {
		return WorkRecord{}, fmt.Errorf("%w: фаза %q має статус %q", ErrPhaseNotOpen, rec.PhaseKey, phaseStatus)
	}

	var out WorkRecord
	err = tx.QueryRow(ctx,
		`INSERT INTO core.work_records
		   (project_id, user_id, work_product_id, phase_key, role_key,
		    work_date, duration_hours, work_category, comment)
		 VALUES ($1, $2, $3, $4, $5, $6, $7::numeric, $8, $9)
		 RETURNING id, project_id, user_id, work_product_id, phase_key, role_key,
		           work_date, duration_hours::text, work_category, COALESCE(comment, ''), created_at`,
		rec.ProjectID, rec.UserID, rec.WorkProductID, rec.PhaseKey, rec.RoleKey,
		rec.WorkDate, rec.DurationHours, rec.WorkCategory, nullIfEmpty(rec.Comment)).
		Scan(&out.ID, &out.ProjectID, &out.UserID, &out.WorkProductID, &out.PhaseKey, &out.RoleKey,
			&out.WorkDate, &out.DurationHours, &out.WorkCategory, &out.Comment, &out.CreatedAt)
	if err != nil {
		return WorkRecord{}, fmt.Errorf("запис трудовитрат: %w", err)
	}

	if err := emitInputsChanged(ctx, tx, rec.ProjectID, rec.UserID, "work_record"); err != nil {
		return WorkRecord{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return WorkRecord{}, fmt.Errorf("фіксація трудовитрат: %w", err)
	}
	return out, nil
}

// SetLaborRate додає версію ставки. Перекриття періодів відхиляє СУБД
// (EXCLUDE-констрейнт), а не перевірка в коді — інакше дві конкурентні
// транзакції створили б дві чинні ставки на ту саму дату.
func (s *Store) SetLaborRate(ctx context.Context, projectID *uuid.UUID, roleKey, hourlyRate, currency string, validFrom time.Time, validTo *time.Time) (uuid.UUID, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("відкриття транзакції ставки: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO core.labor_rates (project_id, role_key, hourly_rate, currency, valid_from, valid_to)
		 VALUES ($1, $2, $3::numeric, $4, $5, $6) RETURNING id`,
		projectID, roleKey, hourlyRate, currency, validFrom, validTo).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23P01" {
			return uuid.Nil, ErrRatePeriodOverlap
		}
		return uuid.Nil, fmt.Errorf("запис ставки: %w", err)
	}

	// Ставка проєктного рівня впливає на AC цього проєкту й має відразу
	// запустити перерахунок: без цього вимірювання AC/CPI лишалося б valid
	// з уже чинними вхідними даними. Загальносистемна ставка (projectID == nil)
	// не має одного проєкту для точкового перерахунку тут.
	if projectID != nil {
		if err := emitInputsChanged(ctx, tx, *projectID, uuid.Nil, "labor_rate"); err != nil {
			return uuid.Nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("фіксація ставки: %w", err)
	}
	return id, nil
}

// CreateCostBaseline створює чергову чернетку кошторису з рядками за фазами.
// Версія призначається в тій самій транзакції під блокуванням проєкту, щоб
// два паралельні виклики не отримали однаковий номер.
func (s *Store) CreateCostBaseline(ctx context.Context, actorID, projectID uuid.UUID, name, currency string, lines []BudgetLine) (uuid.UUID, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("відкриття транзакції кошторису: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT id FROM core.projects WHERE id = $1 FOR UPDATE`, projectID); err != nil {
		return uuid.Nil, fmt.Errorf("блокування проєкту: %w", err)
	}

	var baselineID uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO core.cost_baselines (project_id, version, name, currency, status, created_by)
		 VALUES ($1, COALESCE((SELECT MAX(version) FROM core.cost_baselines WHERE project_id = $1), 0) + 1,
		         $2, $3, 'draft', $4)
		 RETURNING id`, projectID, name, currency, actorID).Scan(&baselineID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("створення кошторису: %w", err)
	}

	for _, line := range lines {
		_, err := tx.Exec(ctx,
			`INSERT INTO core.budget_lines
			   (cost_baseline_id, project_id, phase_key, cost_category, planned_amount, funding_limit)
			 VALUES ($1, $2, $3, $4, $5::numeric, $6::numeric)`,
			baselineID, projectID, line.PhaseKey, line.CostCategory, line.PlannedAmount, line.FundingLimit)
		if err != nil {
			return uuid.Nil, fmt.Errorf("стаття кошторису %s/%s: %w", line.PhaseKey, line.CostCategory, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("фіксація кошторису: %w", err)
	}
	return baselineID, nil
}

type BudgetLine struct {
	PhaseKey      string `json:"phase_key"`
	CostCategory  string `json:"cost_category"`
	PlannedAmount string `json:"planned_amount"`
	FundingLimit  string `json:"funding_limit"`
}

// ApproveCostBaseline затверджує версію кошторису.
//
// Розподіл обовʼязків (ADR-006): затверджувач не може бути автором проєкту
// кошторису. Попередня затверджена версія переводиться в superseded — це
// забезпечує частковий унікальний індекс на одну затверджену версію.
func (s *Store) ApproveCostBaseline(ctx context.Context, baselineID, approverID uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("відкриття транзакції затвердження: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var projectID uuid.UUID
	var status string
	var createdBy uuid.UUID
	err = tx.QueryRow(ctx,
		`SELECT project_id, status, created_by FROM core.cost_baselines WHERE id = $1 FOR UPDATE`, baselineID).
		Scan(&projectID, &status, &createdBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrBaselineNotFound
	}
	if err != nil {
		return fmt.Errorf("читання кошторису: %w", err)
	}
	if status == "approved" {
		return ErrBaselineApproved
	}
	// Розподіл обов'язків: складач кошторису не затверджує його сам.
	// Дублюється констрейнтом СУБД (міграція 0016) — тут лише заради
	// зрозумілої помилки замість порушення обмеження.
	if approverID == createdBy {
		return ErrSelfApproval
	}

	if _, err := tx.Exec(ctx,
		`UPDATE core.cost_baselines SET status = 'superseded', approved_by = NULL, approved_at = NULL
		 WHERE project_id = $1 AND status = 'approved'`, projectID); err != nil {
		return fmt.Errorf("перевід попередньої версії в superseded: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE core.cost_baselines
		 SET status = 'approved', approved_by = $2, approved_at = now()
		 WHERE id = $1`, baselineID, approverID); err != nil {
		return fmt.Errorf("затвердження кошторису: %w", err)
	}

	if err := emitInputsChanged(ctx, tx, projectID, approverID, "cost_baseline_approved"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RecordExpense фіксує нетрудову витрату (SWR-40.4). Списання дозволене лише
// у відкритій фазі й лише у валюті затвердженого кошторису — інакше AC склав би
// суми в різних валютах як єдине число (evm.go, phaseActualCostSQL не конвертує).
func (s *Store) RecordExpense(ctx context.Context, projectID, actorID uuid.UUID, phaseKey, category, expenseType, amount, currency, invoiceRef string, expenseDate time.Time) (uuid.UUID, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("відкриття транзакції витрати: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var phaseStatus string
	err = tx.QueryRow(ctx,
		`SELECT status FROM core.project_phases WHERE project_id = $1 AND phase_key = $2`,
		projectID, phaseKey).Scan(&phaseStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrPhaseNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("читання фази: %w", err)
	}
	if phaseStatus != "active" {
		return uuid.Nil, fmt.Errorf("%w: фаза %q має статус %q", ErrPhaseNotOpen, phaseKey, phaseStatus)
	}

	var baselineCurrency string
	err = tx.QueryRow(ctx,
		`SELECT currency FROM core.cost_baselines WHERE project_id = $1 AND status = 'approved'`,
		projectID).Scan(&baselineCurrency)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("читання валюти кошторису: %w", err)
	}
	if err == nil && baselineCurrency != currency {
		return uuid.Nil, fmt.Errorf("%w: витрата в %q, кошторис у %q", ErrCurrencyMismatch, currency, baselineCurrency)
	}

	var id uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO core.expense_records
		   (project_id, phase_key, cost_category, expense_type, amount, currency,
		    invoice_reference, expense_date, recorded_by)
		 VALUES ($1, $2, $3, $4, $5::numeric, $6, $7, $8, $9) RETURNING id`,
		projectID, phaseKey, category, expenseType, amount, currency,
		nullIfEmpty(invoiceRef), expenseDate, actorID).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("запис витрати: %w", err)
	}

	if err := emitInputsChanged(ctx, tx, projectID, actorID, "expense"); err != nil {
		return uuid.Nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("фіксація витрати: %w", err)
	}
	return id, nil
}

// LinkPhaseDeliverable приписує результат до фази з ваговим коефіцієнтом.
// Без цього звʼязку здобута цінність фази не обчислюється (ISO 21511).
func (s *Store) LinkPhaseDeliverable(ctx context.Context, projectID, workProductID, actorID uuid.UUID, phaseKey, weight string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("відкриття транзакції привʼязки результату: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`INSERT INTO core.phase_deliverables (project_id, phase_key, work_product_id, weight)
		 VALUES ($1, $2, $3, $4::numeric)
		 ON CONFLICT (project_id, work_product_id)
		 DO UPDATE SET phase_key = EXCLUDED.phase_key, weight = EXCLUDED.weight`,
		projectID, phaseKey, workProductID, weight); err != nil {
		return fmt.Errorf("привʼязка результату до фази: %w", err)
	}

	if err := emitInputsChanged(ctx, tx, projectID, actorID, "phase_deliverable"); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("фіксація привʼязки результату: %w", err)
	}
	return nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
