package project_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"delmos/internal/economics"
	"delmos/internal/migrate"
	"delmos/internal/project"
	"delmos/internal/testsupport"
)

type phaseFixture struct {
	pool      *pgxpool.Pool
	projects  *project.Store
	econ      *economics.Store
	projectID uuid.UUID
	actor     uuid.UUID
	approver  uuid.UUID
}

const (
	gatePhase  = "design"
	gateBudget = "10000.00"
	gateLimit  = "12000.00"
)

var gateAsOf = time.Date(2026, 1, 16, 0, 0, 0, 0, time.UTC)

func newPhaseFixture(t *testing.T) *phaseFixture {
	t.Helper()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, testsupport.NewDatabase(t))
	if err != nil {
		t.Fatalf("підключення до тимчасової бази: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := migrate.Apply(ctx, pool, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatalf("застосування міграцій: %v", err)
	}

	f := &phaseFixture{pool: pool, projects: project.NewStore(pool), econ: economics.New(pool)}
	f.actor = f.user(t, "author")
	f.approver = f.user(t, "controller")
	f.projectID = f.project(t)
	f.phase(t, gatePhase, "not_started")
	return f
}

func (f *phaseFixture) user(t *testing.T, login string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := f.pool.QueryRow(context.Background(),
		`INSERT INTO core.users (login, display_name, password_hash, is_active)
		 VALUES ($1::citext, $2::text, 'x', true) RETURNING id`, login, login).Scan(&id); err != nil {
		t.Fatalf("створення користувача %s: %v", login, err)
	}
	return id
}

func (f *phaseFixture) project(t *testing.T) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := f.pool.QueryRow(context.Background(),
		`INSERT INTO core.projects (code, name, description, status, created_by)
		 VALUES ('GATE-1', 'Шлюз', '', 'active', $1) RETURNING id`, f.actor).Scan(&id); err != nil {
		t.Fatalf("створення проєкту: %v", err)
	}
	return id
}

func (f *phaseFixture) phase(t *testing.T, key, status string) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(),
		`INSERT INTO core.project_phases
		   (project_id, phase_key, name, planned_start, planned_finish, status, config_generation)
		 VALUES ($1, $2, $3, DATE '2026-01-01', DATE '2026-01-31', $4, 1)`,
		f.projectID, key, key, status); err != nil {
		t.Fatalf("створення фази %s: %v", key, err)
	}
}

func (f *phaseFixture) approvedBaseline(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	id, err := f.econ.CreateCostBaseline(ctx, f.projectID, "Базовий", "EUR",
		[]economics.BudgetLine{{
			PhaseKey: gatePhase, CostCategory: "labor",
			PlannedAmount: gateBudget, FundingLimit: gateLimit,
		}})
	if err != nil {
		t.Fatalf("створення кошторису: %v", err)
	}
	if err := f.econ.ApproveCostBaseline(ctx, id, f.approver); err != nil {
		t.Fatalf("затвердження кошторису: %v", err)
	}
}

func (f *phaseFixture) activate(t *testing.T) {
	t.Helper()
	if _, err := f.projects.TransitionPhase(context.Background(),
		f.actor, f.projectID, gatePhase, "active", gateAsOf); err != nil {
		t.Fatalf("активація фази: %v", err)
	}
}

func (f *phaseFixture) spend(t *testing.T, amount string) {
	t.Helper()
	if _, err := f.econ.RecordExpense(context.Background(), f.projectID, f.actor, gatePhase,
		"hardware_prototypes", "capex", amount, "EUR", "INV-1",
		time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("запис витрати %s: %v", amount, err)
	}
}

func (f *phaseFixture) status(t *testing.T) string {
	t.Helper()
	var status string
	if err := f.pool.QueryRow(context.Background(),
		`SELECT status FROM core.project_phases WHERE project_id = $1 AND phase_key = $2`,
		f.projectID, gatePhase).Scan(&status); err != nil {
		t.Fatalf("читання статусу фази: %v", err)
	}
	return status
}

// Головна перевірка результативності шлюзу: перевищення ліміту фінансування
// має не лише повернути помилку, а й ЗАЛИШИТИ фазу відкритою. Якби транзакція
// не відкотилася, фаза закрилася б попри вето.
func TestFundingLimitGateBlocksPhaseClosure(t *testing.T) {
	f := newPhaseFixture(t)
	f.approvedBaseline(t)
	f.activate(t)
	f.spend(t, "13000.00") // ліміт 12000

	_, err := f.projects.TransitionPhase(context.Background(),
		f.actor, f.projectID, gatePhase, "completed", gateAsOf)
	if err == nil {
		t.Fatal("очікувалось вето: витрати перевищують ліміт фінансування")
	}
	if !errors.Is(err, project.ErrPhaseGateRejected) {
		t.Errorf("очікувано ErrPhaseGateRejected, отримано %v", err)
	}
	if got := f.status(t); got != "active" {
		t.Errorf("статус фази = %q, очікувано active — транзакція мала відкотитися", got)
	}
}

// Дзеркальний випадок: у межах ліміту шлюз не заважає. Без цього тесту
// попередній доводив би лише те, що перехід зламаний узагалі.
func TestPhaseClosesWhenWithinFundingLimit(t *testing.T) {
	f := newPhaseFixture(t)
	f.approvedBaseline(t)
	f.activate(t)
	f.spend(t, "9000.00") // ліміт 12000

	if _, err := f.projects.TransitionPhase(context.Background(),
		f.actor, f.projectID, gatePhase, "completed", gateAsOf); err != nil {
		t.Fatalf("перехід у межах ліміту має проходити: %v", err)
	}
	if got := f.status(t); got != "completed" {
		t.Errorf("статус фази = %q, очікувано completed", got)
	}
}

// Межа рівно на ліміті не є перевищенням.
func TestPhaseClosesExactlyAtFundingLimit(t *testing.T) {
	f := newPhaseFixture(t)
	f.approvedBaseline(t)
	f.activate(t)
	f.spend(t, gateLimit)

	if _, err := f.projects.TransitionPhase(context.Background(),
		f.actor, f.projectID, gatePhase, "completed", gateAsOf); err != nil {
		t.Fatalf("витрата рівно на ліміті не є перевищенням: %v", err)
	}
	if got := f.status(t); got != "completed" {
		t.Errorf("статус фази = %q, очікувано completed", got)
	}
}

// Без затвердженого кошторису ліміту не існує, тож перевіряти нічого —
// шлюз не має перетворюватися на глухий блокувальник.
func TestPhaseClosesWithoutApprovedBaseline(t *testing.T) {
	f := newPhaseFixture(t)
	f.activate(t)

	if _, err := f.projects.TransitionPhase(context.Background(),
		f.actor, f.projectID, gatePhase, "completed", gateAsOf); err != nil {
		t.Fatalf("без кошторису перехід має проходити: %v", err)
	}
}

// Зворотні та стрибкові переходи заборонені: завершена фаза вже врахована
// в показниках, її повторне відкриття переписало б підсумки.
func TestInvalidPhaseTransitionsRejected(t *testing.T) {
	cases := []struct {
		name    string
		initial string
		target  string
	}{
		{"стрибок через active", "not_started", "completed"},
		{"повторне відкриття", "completed", "active"},
		{"відкат у not_started", "active", "not_started"},
		{"перехід у самого себе", "active", "active"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newPhaseFixture(t)
			if _, err := f.pool.Exec(context.Background(),
				`UPDATE core.project_phases SET status = $3
				 WHERE project_id = $1 AND phase_key = $2`,
				f.projectID, gatePhase, tc.initial); err != nil {
				t.Fatalf("підготовка статусу: %v", err)
			}

			_, err := f.projects.TransitionPhase(context.Background(),
				f.actor, f.projectID, gatePhase, tc.target, gateAsOf)
			if !errors.Is(err, project.ErrPhaseTransitionInvalid) {
				t.Errorf("%s -> %s: очікувано ErrPhaseTransitionInvalid, отримано %v",
					tc.initial, tc.target, err)
			}
			if got := f.status(t); got != tc.initial {
				t.Errorf("статус змінився на %q попри відхилення переходу", got)
			}
		})
	}
}

// Попередження про відхилення вартості не має блокувати перехід — воно лише
// фіксується в аудиті (ADVISORY_WARNING).
func TestCostVarianceAlertWarnsButDoesNotBlock(t *testing.T) {
	f := newPhaseFixture(t)
	f.approvedBaseline(t)
	f.activate(t)
	// Витрати є, затверджених результатів немає => EV = 0 => CPI = 0 < 0.85.
	f.spend(t, "1000.00")

	if _, err := f.projects.TransitionPhase(context.Background(),
		f.actor, f.projectID, gatePhase, "completed", gateAsOf); err != nil {
		t.Fatalf("ADVISORY_WARNING не має блокувати перехід: %v", err)
	}

	var warnings int
	if err := f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM core.audit_events
		 WHERE action = 'rule.violated' AND detail->>'rule_key' = 'rule.economics.cost_variance_alert'`).
		Scan(&warnings); err != nil {
		t.Fatalf("читання аудиту: %v", err)
	}
	if warnings == 0 {
		t.Error("зауваження про відхилення вартості не зафіксовано в аудиті")
	}
}

func TestPhaseTransitionUnknownPhase(t *testing.T) {
	f := newPhaseFixture(t)
	_, err := f.projects.TransitionPhase(context.Background(),
		f.actor, f.projectID, "no-such-phase", "active", gateAsOf)
	if !errors.Is(err, project.ErrPhaseNotFound) {
		t.Errorf("очікувано ErrPhaseNotFound, отримано %v", err)
	}
}
