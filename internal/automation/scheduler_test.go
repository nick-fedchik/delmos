package automation_test

import (
	"context"
	"testing"

	"delmos/internal/automation"
)

// TestSchedulerFixedRateDeduplicatesAndAdvances перевіряє SWR-24: атомарний
// запис ScheduleOccurrence разом з подією та захист від дублювання через
// унікальний (schedule_id, occurrence_time_utc) — повторний Tick без
// настання нового терміну не створює другого запису.
func TestSchedulerFixedRateDeduplicatesAndAdvances(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	engine := automation.NewEngine(pool, newTestLogger())

	scheduleID, err := engine.CreateSchedule(ctx, "test.fixed-rate-demo", "fixed_rate", automation.ScheduleSpec{IntervalSeconds: 3600})
	if err != nil {
		t.Fatalf("створення розкладу: %v", err)
	}

	processed, err := engine.TickScheduler(ctx)
	if err != nil {
		t.Fatalf("перший tick: %v", err)
	}
	if processed != 1 {
		t.Fatalf("очікувалося 1 спрацювання розкладу, отримано %d", processed)
	}

	// Наступний tick одразу після першого не повинен нічого зробити:
	// next_run_at посунуто на годину вперед.
	processed, err = engine.TickScheduler(ctx)
	if err != nil {
		t.Fatalf("другий tick: %v", err)
	}
	if processed != 0 {
		t.Fatalf("очікувалося 0 спрацювань одразу після попереднього, отримано %d", processed)
	}

	var occurrences int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM core.schedule_occurrences WHERE schedule_id = $1`, scheduleID).Scan(&occurrences); err != nil {
		t.Fatalf("підрахунок настань: %v", err)
	}
	if occurrences != 1 {
		t.Fatalf("очікувалося 1 настання, отримано %d", occurrences)
	}
}

// TestSchedulerOnceDeactivatesAfterFiring перевіряє режим "once": після
// одноразового спрацювання розклад стає неактивним і більше не тікає.
func TestSchedulerOnceDeactivatesAfterFiring(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	engine := automation.NewEngine(pool, newTestLogger())

	if _, err := engine.CreateSchedule(ctx, "test.once-demo", "once", automation.ScheduleSpec{}); err != nil {
		t.Fatalf("створення розкладу: %v", err)
	}

	if processed, err := engine.TickScheduler(ctx); err != nil || processed != 1 {
		t.Fatalf("перший tick: processed=%d err=%v", processed, err)
	}

	var active bool
	if err := pool.QueryRow(ctx, `SELECT active FROM core.schedule_definitions WHERE key = 'test.once-demo'`).Scan(&active); err != nil {
		t.Fatalf("читання стану розкладу: %v", err)
	}
	if active {
		t.Fatal("розклад once мав деактивуватися після спрацювання")
	}

	if processed, err := engine.TickScheduler(ctx); err != nil || processed != 0 {
		t.Fatalf("другий tick не мав нічого обробити: processed=%d err=%v", processed, err)
	}
}
