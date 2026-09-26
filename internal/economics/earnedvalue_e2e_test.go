package economics_test

import (
	"context"
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

// Наскрізна перевірка: до появи команд погодження здобута цінність завжди
// дорівнювала нулю, бо статус approved був недосяжний через API. Цей тест
// доводить, що ланцюг submit -> review -> approve -> EV тепер замкнений.
func TestEarnedValueBecomesNonZeroAfterApproval(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testsupport.NewDatabase(t))
	if err != nil {
		t.Fatalf("пул підключень: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := migrate.Apply(ctx, pool, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatalf("міграції: %v", err)
	}

	econ := economics.New(pool)
	projects := project.NewStore(pool)

	author := mustUser(t, pool, "author")
	reviewer := mustUser(t, pool, "reviewer")
	approver := mustUser(t, pool, "approver")
	projectID := mustProject(t, pool, author)
	mustPhase(t, pool, projectID, "design")

	baselineID, err := econ.CreateCostBaseline(ctx, author, projectID, "Базовий", "EUR",
		[]economics.BudgetLine{{
			PhaseKey: "design", CostCategory: "labor",
			PlannedAmount: "10000.00", FundingLimit: "12000.00",
		}})
	if err != nil {
		t.Fatalf("кошторис: %v", err)
	}
	if err := econ.ApproveCostBaseline(ctx, baselineID, approver); err != nil {
		t.Fatalf("затвердження кошторису: %v", err)
	}

	wpID := mustDraftWithRevision(t, pool, projectID, author, "REQ-1")
	if err := econ.LinkPhaseDeliverable(ctx, projectID, wpID, "design", "1.000"); err != nil {
		t.Fatalf("привʼязка результату до фази: %v", err)
	}

	asOf := time.Date(2026, 1, 16, 0, 0, 0, 0, time.UTC)

	before, err := econ.ComputeEarnedValue(ctx, projectID, asOf)
	if err != nil {
		t.Fatalf("розрахунок до погодження: %v", err)
	}
	if before.EarnedValue != "0.00" {
		t.Fatalf("EV до погодження = %s, очікувано 0.00 (чернетка не зараховується)", before.EarnedValue)
	}

	if _, err := projects.SubmitWorkProduct(ctx, author, projectID, wpID, []project.ReviewAssignment{
		{UserID: reviewer, Role: "reviewer"},
		{UserID: approver, Role: "approver"},
	}); err != nil {
		t.Fatalf("подання: %v", err)
	}
	if _, err := projects.RecordDecision(ctx, reviewer, projectID, wpID, project.DecisionReview, "", ""); err != nil {
		t.Fatalf("рецензія: %v", err)
	}
	if _, err := projects.RecordDecision(ctx, approver, projectID, wpID, project.DecisionApproval, "", ""); err != nil {
		t.Fatalf("погодження: %v", err)
	}

	after, err := econ.ComputeEarnedValue(ctx, projectID, asOf)
	if err != nil {
		t.Fatalf("розрахунок після погодження: %v", err)
	}
	// Єдиний результат фази затверджено => здобуто всю планову вартість фази.
	if after.EarnedValue != "10000.00" {
		t.Errorf("EV після погодження = %s, очікувано 10000.00", after.EarnedValue)
	}
	// SPI = EV/PV; PV на середину фази = 5000 => SPI = 2.
	if after.SchedulePerformanceIndex == nil || *after.SchedulePerformanceIndex != "2.0000" {
		t.Errorf("SPI = %v, очікувано 2.0000 (EV 10000 / PV 5000)", after.SchedulePerformanceIndex)
	}
}

func mustUser(t *testing.T, pool *pgxpool.Pool, login string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO core.users (login, display_name, password_hash, is_active)
		 VALUES ($1::citext, $2::text, 'x', true) RETURNING id`, login, login).Scan(&id); err != nil {
		t.Fatalf("створення користувача %s: %v", login, err)
	}
	return id
}

func mustProject(t *testing.T, pool *pgxpool.Pool, author uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO core.projects (code, name, description, status, created_by)
		 VALUES ('EV-1', 'Здобута цінність', '', 'active', $1) RETURNING id`, author).Scan(&id); err != nil {
		t.Fatalf("створення проєкту: %v", err)
	}
	return id
}

func mustPhase(t *testing.T, pool *pgxpool.Pool, projectID uuid.UUID, key string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO core.project_phases
		   (project_id, phase_key, name, planned_start, planned_finish, status, config_generation)
		 VALUES ($1, $2, $2, DATE '2026-01-01', DATE '2026-01-31', 'active', 1)`,
		projectID, key); err != nil {
		t.Fatalf("створення фази: %v", err)
	}
}

func mustDraftWithRevision(t *testing.T, pool *pgxpool.Pool, projectID, author uuid.UUID, code string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var wpID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO core.work_products (project_id, code, type, profile, title, status)
		 VALUES ($1, $2, 'requirement', 'core:requirement', $3, 'draft') RETURNING id`,
		projectID, code, code).Scan(&wpID); err != nil {
		t.Fatalf("створення артефакту: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO core.work_product_revisions
		   (work_product_id, revision_number, body, metadata, payload_hash, content_hash, created_by)
		 VALUES ($1, 1, 'текст', '{}'::jsonb,
		         sha256(convert_to('текст', 'UTF8')), sha256(convert_to('текст', 'UTF8')), $2)`,
		wpID, author); err != nil {
		t.Fatalf("створення ревізії: %v", err)
	}
	return wpID
}
