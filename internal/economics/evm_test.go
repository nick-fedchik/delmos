package economics

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"delmos/internal/migrate"
	"delmos/internal/testsupport"
)

// Дати фіксовані: показники здобутої цінності залежать від календаря, тому
// тест не має права спиратися на "сьогодні".
var (
	phaseStart  = date(2026, 1, 1)
	phaseFinish = date(2026, 1, 31)
	controlDate = date(2026, 1, 16) // рівно половина 30-денного інтервалу
)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

type fixture struct {
	pool    *pgxpool.Pool
	store   *Store
	project uuid.UUID
	author  uuid.UUID
	checker uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	dsn := testsupport.NewDatabase(t)
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("пул підключень: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := migrate.Apply(ctx, pool, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatalf("міграції: %v", err)
	}

	f := &fixture{pool: pool, store: New(pool)}
	f.author = f.createUser(t, "author")
	f.checker = f.createUser(t, "checker")
	f.project = f.createProject(t)
	f.createPhase(t, "design", "Проєктування", "active")
	return f
}

func (f *fixture) createUser(t *testing.T, login string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := f.pool.QueryRow(context.Background(),
		`INSERT INTO core.users (login, display_name, password_hash, is_active)
		 VALUES ($1::citext, $2::text, 'x', true) RETURNING id`, login, login).Scan(&id)
	if err != nil {
		t.Fatalf("створення користувача %s: %v", login, err)
	}
	return id
}

func (f *fixture) createProject(t *testing.T) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := f.pool.QueryRow(context.Background(),
		`INSERT INTO core.projects (code, name, description, status, created_by)
		 VALUES ('ECO-1', 'Економіка', '', 'active', $1) RETURNING id`, f.author).Scan(&id)
	if err != nil {
		t.Fatalf("створення проєкту: %v", err)
	}
	return id
}

func (f *fixture) createPhase(t *testing.T, key, name, status string) {
	t.Helper()
	_, err := f.pool.Exec(context.Background(),
		`INSERT INTO core.project_phases
		   (project_id, phase_key, name, planned_start, planned_finish, status, config_generation)
		 VALUES ($1, $2, $3, $4, $5, $6, 1)`,
		f.project, key, name, phaseStart, phaseFinish, status)
	if err != nil {
		t.Fatalf("створення фази %s: %v", key, err)
	}
}

func (f *fixture) createWorkProduct(t *testing.T, code, status string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := f.pool.QueryRow(context.Background(),
		`INSERT INTO core.work_products (project_id, code, type, profile, title, status)
		 VALUES ($1, $2, 'requirement', 'core:requirement', $3, $4) RETURNING id`,
		f.project, code, code, status).Scan(&id)
	if err != nil {
		t.Fatalf("створення артефакту %s: %v", code, err)
	}
	return id
}

// approvedBaseline створює й затверджує кошторис на 10000 для фази design.
func (f *fixture) approvedBaseline(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := f.store.CreateCostBaseline(context.Background(), f.author, f.project, "Базовий", "EUR",
		[]BudgetLine{{PhaseKey: "design", CostCategory: "labor", PlannedAmount: "10000.00", FundingLimit: "12000.00"}})
	if err != nil {
		t.Fatalf("створення кошторису: %v", err)
	}
	if err := f.store.ApproveCostBaseline(context.Background(), id, f.checker); err != nil {
		t.Fatalf("затвердження кошторису: %v", err)
	}
	return id
}

// Планова вартість рознесена лінійно за календарем фази: на 16 січня минуло
// 15 з 30 днів, отже PV = 50% від 10000.
func TestPlannedValueIsTimePhasedLinearly(t *testing.T) {
	f := newFixture(t)
	f.approvedBaseline(t)

	got, err := f.store.ComputeEarnedValue(context.Background(), f.project, controlDate)
	if err != nil {
		t.Fatalf("розрахунок: %v", err)
	}
	if got.PlannedValue != "5000.00" {
		t.Errorf("PV = %s, очікувано 5000.00", got.PlannedValue)
	}
	if got.BudgetAtCompletion != "10000.00" {
		t.Errorf("BAC = %s, очікувано 10000.00", got.BudgetAtCompletion)
	}
	// Робіт не виконано, тому індексів не існує — але не нуль: нуль означав би
	// "показник дорівнює нулю", а насправді даних для ділення немає.
	if got.CostPerformanceIndex != nil {
		t.Errorf("CPI = %v, очікувано відсутній за нульових витрат", *got.CostPerformanceIndex)
	}
	if got.SchedulePerformanceIndex == nil || *got.SchedulePerformanceIndex != "0.0000" {
		t.Errorf("SPI = %v, очікувано 0.0000 за ненульового PV", got.SchedulePerformanceIndex)
	}
}

// Фактична вартість = години × ставка, чинна на день виконання роботи.
func TestActualCostUsesRateValidOnWorkDate(t *testing.T) {
	f := newFixture(t)
	f.approvedBaseline(t)
	ctx := context.Background()

	until := date(2026, 1, 10)
	if _, err := f.store.SetLaborRate(ctx, &f.project, "engineer", "100.00", "EUR", phaseStart, &until); err != nil {
		t.Fatalf("рання ставка: %v", err)
	}
	if _, err := f.store.SetLaborRate(ctx, &f.project, "engineer", "200.00", "EUR", until, nil); err != nil {
		t.Fatalf("пізня ставка: %v", err)
	}

	// 10 год за старою ставкою (100) + 10 год за новою (200) = 3000.
	mustLog(t, f, date(2026, 1, 5), "10.00")
	mustLog(t, f, date(2026, 1, 15), "10.00")

	got, err := f.store.ComputeEarnedValue(ctx, f.project, controlDate)
	if err != nil {
		t.Fatalf("розрахунок: %v", err)
	}
	if got.ActualCost != "3000.00" {
		t.Errorf("AC = %s, очікувано 3000.00 (10×100 + 10×200)", got.ActualCost)
	}
}

// Зміна ставки заднім числом не повинна переписувати вже пораховану історію.
func TestRateVersionsMayNotOverlap(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	until := date(2026, 6, 1)
	if _, err := f.store.SetLaborRate(ctx, &f.project, "engineer", "100.00", "EUR", phaseStart, &until); err != nil {
		t.Fatalf("перша ставка: %v", err)
	}

	overlapStart := date(2026, 3, 1)
	_, err := f.store.SetLaborRate(ctx, &f.project, "engineer", "150.00", "EUR", overlapStart, nil)
	if err == nil {
		t.Fatal("очікувалась відмова: період ставки перекривається з наявним")
	}
	if !isRateOverlap(err) {
		t.Errorf("очікувано ErrRatePeriodOverlap, отримано %v", err)
	}
}

// Здобута цінність за правилом 0/100: лише затверджені результати.
func TestEarnedValueCountsOnlyApprovedDeliverables(t *testing.T) {
	f := newFixture(t)
	f.approvedBaseline(t)
	ctx := context.Background()

	approved := f.createWorkProduct(t, "REQ-1", "approved")
	draft := f.createWorkProduct(t, "REQ-2", "draft")
	for _, wp := range []uuid.UUID{approved, draft} {
		if err := f.store.LinkPhaseDeliverable(ctx, f.project, wp, "design", "1.000"); err != nil {
			t.Fatalf("привʼязка результату: %v", err)
		}
	}

	got, err := f.store.ComputeEarnedValue(ctx, f.project, controlDate)
	if err != nil {
		t.Fatalf("розрахунок: %v", err)
	}
	// Затверджено 1 з 2 результатів рівної ваги => EV = 50% від 10000.
	if got.EarnedValue != "5000.00" {
		t.Errorf("EV = %s, очікувано 5000.00 (1 з 2 результатів затверджено)", got.EarnedValue)
	}
}

// Перевищення ліміту фінансування має бути видимим у показниках фази —
// це вхідні дані для правила rule.economics.funding_limit_gate.
func TestPhaseOverFundingLimitIsFlagged(t *testing.T) {
	f := newFixture(t)
	f.approvedBaseline(t)
	ctx := context.Background()

	// Ліміт фази — 12000. Витрата 13000 його перевищує.
	if _, err := f.store.RecordExpense(ctx, f.project, f.author, "design",
		"hardware_prototypes", "capex", "13000.00", "EUR", "INV-1", date(2026, 1, 10)); err != nil {
		t.Fatalf("запис витрати: %v", err)
	}

	got, err := f.store.ComputeEarnedValue(ctx, f.project, controlDate)
	if err != nil {
		t.Fatalf("розрахунок: %v", err)
	}
	if len(got.Phases) != 1 {
		t.Fatalf("очікувано 1 фазу, отримано %d", len(got.Phases))
	}
	if !got.Phases[0].OverLimit {
		t.Errorf("фазу не позначено як таку, що перевищила ліміт: AC=%s, ліміт=%s",
			got.Phases[0].ActualCost, got.Phases[0].FundingLimit)
	}
}

// Списання годин у завершену фазу змінило б уже подану звітність.
func TestWorkRecordRejectedForClosedPhase(t *testing.T) {
	f := newFixture(t)
	f.createPhase(t, "closed", "Завершена", "completed")

	_, err := f.store.LogWorkRecord(context.Background(), WorkRecord{
		ProjectID: f.project, UserID: f.author, PhaseKey: "closed", RoleKey: "engineer",
		WorkDate: date(2026, 1, 5), DurationHours: "8.00", WorkCategory: "design",
	})
	if err == nil {
		t.Fatal("очікувалась відмова списання у завершену фазу")
	}
}

// У проєкті може бути лише один затверджений кошторис: інакше BAC неоднозначний.
func TestApprovingSecondBaselineSupersedesFirst(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	first := f.approvedBaseline(t)

	second, err := f.store.CreateCostBaseline(ctx, f.author, f.project, "Переглянутий", "EUR",
		[]BudgetLine{{PhaseKey: "design", CostCategory: "labor", PlannedAmount: "20000.00", FundingLimit: "20000.00"}})
	if err != nil {
		t.Fatalf("друга версія: %v", err)
	}
	if err := f.store.ApproveCostBaseline(ctx, second, f.checker); err != nil {
		t.Fatalf("затвердження другої версії: %v", err)
	}

	var firstStatus string
	if err := f.pool.QueryRow(ctx, `SELECT status FROM core.cost_baselines WHERE id = $1`, first).
		Scan(&firstStatus); err != nil {
		t.Fatalf("читання першої версії: %v", err)
	}
	if firstStatus != "superseded" {
		t.Errorf("статус попередньої версії = %q, очікувано superseded", firstStatus)
	}

	got, err := f.store.ComputeEarnedValue(ctx, f.project, controlDate)
	if err != nil {
		t.Fatalf("розрахунок: %v", err)
	}
	if got.BudgetAtCompletion != "20000.00" {
		t.Errorf("BAC = %s, очікувано 20000.00 з нової версії", got.BudgetAtCompletion)
	}
}

// Розподіл обовʼязків: складач кошторису не затверджує його сам. Перевірку
// дублює констрейнт СУБД (міграція 0016), тому вона діє й в обхід коду.
func TestCostBaselineSelfApprovalRejected(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	id, err := f.store.CreateCostBaseline(ctx, f.author, f.project, "Базовий", "EUR",
		[]BudgetLine{{PhaseKey: "design", CostCategory: "labor", PlannedAmount: "10000.00", FundingLimit: "12000.00"}})
	if err != nil {
		t.Fatalf("створення кошторису: %v", err)
	}

	if err := f.store.ApproveCostBaseline(ctx, id, f.author); err == nil {
		t.Fatal("складач кошторису не має права його затверджувати")
	}

	var status string
	if err := f.pool.QueryRow(ctx, `SELECT status FROM core.cost_baselines WHERE id = $1`, id).
		Scan(&status); err != nil {
		t.Fatalf("читання статусу: %v", err)
	}
	if status != "draft" {
		t.Errorf("статус = %q, очікувано draft — затвердження мало відкотитися", status)
	}

	// Інша особа затверджує без перешкод.
	if err := f.store.ApproveCostBaseline(ctx, id, f.checker); err != nil {
		t.Fatalf("незалежний затверджувач має проходити: %v", err)
	}
}

// Констрейнт СУБД тримає інваріант навіть при помилці в застосунку.
func TestCostBaselineSelfApprovalRejectedByDatabase(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	id, err := f.store.CreateCostBaseline(ctx, f.author, f.project, "Базовий", "EUR",
		[]BudgetLine{{PhaseKey: "design", CostCategory: "labor", PlannedAmount: "1.00", FundingLimit: "1.00"}})
	if err != nil {
		t.Fatalf("створення кошторису: %v", err)
	}

	_, err = f.pool.Exec(ctx,
		`UPDATE core.cost_baselines
		 SET status = 'approved', approved_by = created_by, approved_at = now()
		 WHERE id = $1`, id)
	if err == nil {
		t.Fatal("СУБД мала відхилити самозатвердження в обхід коду")
	}
}

func mustLog(t *testing.T, f *fixture, day time.Time, hours string) {
	t.Helper()
	_, err := f.store.LogWorkRecord(context.Background(), WorkRecord{
		ProjectID: f.project, UserID: f.author, PhaseKey: "design", RoleKey: "engineer",
		WorkDate: day, DurationHours: hours, WorkCategory: "design",
	})
	if err != nil {
		t.Fatalf("списання годин за %s: %v", day.Format(time.DateOnly), err)
	}
}

func isRateOverlap(err error) bool {
	return err != nil && err.Error() == ErrRatePeriodOverlap.Error()
}
