package automation_test

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"delmos/internal/automation"
	"delmos/internal/migrate"
	"delmos/internal/testsupport"
)

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testsupport.NewDatabase(t))
	if err != nil {
		t.Fatalf("підключення до тимчасової бази: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := migrate.Apply(context.Background(), pool, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatalf("застосування міграцій: %v", err)
	}
	return pool
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestEmitEventIsAtomicWithBusinessChange перевіряє транзакційний outbox
// (ADR-007): подія стає видимою лише після commit тієї самої транзакції.
func TestEmitEventIsAtomicWithBusinessChange(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)

	// Відкат транзакції не повинен лишати запис у outbox.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("початок транзакції: %v", err)
	}
	if err := automation.EmitEvent(ctx, tx, "wp.revision_committed", nil, nil, uuid.New(), map[string]any{"x": "1"}); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("відкат: %v", err)
	}

	var countAfterRollback int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM core.event_outbox`).Scan(&countAfterRollback); err != nil {
		t.Fatalf("підрахунок outbox після відкату: %v", err)
	}
	if countAfterRollback != 0 {
		t.Fatalf("outbox не повинен містити рядків після відкату транзакції, отримано %d", countAfterRollback)
	}

	// Commit — подія видима.
	tx2, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("початок транзакції 2: %v", err)
	}
	correlationID := uuid.New()
	if err := automation.EmitEvent(ctx, tx2, "wp.revision_committed", nil, nil, correlationID, map[string]any{"x": "2"}); err != nil {
		t.Fatalf("emit 2: %v", err)
	}
	if err := tx2.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var countAfterCommit int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM core.event_outbox`).Scan(&countAfterCommit); err != nil {
		t.Fatalf("підрахунок outbox після commit: %v", err)
	}
	if countAfterCommit != 1 {
		t.Fatalf("outbox мав містити рівно 1 рядок після commit, отримано %d", countAfterCommit)
	}
}

// TestDispatchAndProcessDeliversToRegisteredHandler перевіряє наскрізний шлях
// outbox -> event_deliveries -> Lease Fencing -> обробник -> delivered.
func TestDispatchAndProcessDeliversToRegisteredHandler(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	engine := automation.NewEngine(pool, newTestLogger())

	processed := make(chan string, 1)
	engine.RegisterHandler("trigger.core.after_revision_committed", func(ctx context.Context, tx pgx.Tx, env automation.Envelope) error {
		processed <- env.EventKey
		return nil
	})

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("початок транзакції: %v", err)
	}
	if err := automation.EmitEvent(ctx, tx, "wp.revision_committed", nil, nil, uuid.New(), map[string]any{"work_product_id": uuid.New().String(), "revision_id": uuid.New().String()}); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if n, err := engine.DispatchPending(ctx); err != nil || n != 1 {
		t.Fatalf("диспетчеризація: n=%d err=%v", n, err)
	}
	if n, err := engine.ClaimAndProcess(ctx); err != nil || n != 1 {
		t.Fatalf("обробка: n=%d err=%v", n, err)
	}

	select {
	case key := <-processed:
		if key != "wp.revision_committed" {
			t.Fatalf("неочікуваний event_key: %s", key)
		}
	default:
		t.Fatal("обробник не був викликаний")
	}

	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM core.event_deliveries LIMIT 1`).Scan(&status); err != nil {
		t.Fatalf("читання статусу доставки: %v", err)
	}
	if status != "delivered" {
		t.Fatalf("статус доставки мав бути delivered, отримано %s", status)
	}
}

// TestDeadLetterAfterMaxAttempts перевіряє SPEC-04 dead-letter handling:
// доставка, що постійно провалюється, після max_attempts переходить у
// dead_letter і потрапляє у знімок спостереження.
func TestDeadLetterAfterMaxAttempts(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	engine := automation.NewEngine(pool, newTestLogger())

	failing := automation.HandlerFunc(func(ctx context.Context, tx pgx.Tx, env automation.Envelope) error {
		return context.DeadlineExceeded
	})
	engine.RegisterHandler("trigger.core.after_revision_committed", failing)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("початок транзакції: %v", err)
	}
	if err := automation.EmitEvent(ctx, tx, "wp.revision_committed", nil, nil, uuid.New(), map[string]any{"work_product_id": uuid.New().String(), "revision_id": uuid.New().String()}); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if _, err := engine.DispatchPending(ctx); err != nil {
		t.Fatalf("диспетчеризація: %v", err)
	}

	// max_attempts=5 за замовчуванням — 5 невдалих спроб мають перевести
	// доставку в dead_letter.
	for i := 0; i < 5; i++ {
		if _, err := engine.ClaimAndProcess(ctx); err != nil {
			t.Fatalf("обробка спроби %d: %v", i, err)
		}
	}

	snapshot, err := engine.Snapshot(ctx)
	if err != nil {
		t.Fatalf("знімок спостереження: %v", err)
	}
	if snapshot.Counts["dead_letter"] != 1 {
		t.Fatalf("очікувалася 1 dead-letter доставка, отримано лічильники %+v", snapshot.Counts)
	}
	if len(snapshot.DeadLetters) != 1 || snapshot.DeadLetters[0].TriggerKey != "trigger.core.after_revision_committed" {
		t.Fatalf("несподіваний перелік dead-letter: %+v", snapshot.DeadLetters)
	}
}

// TestLeaseFencingPreventsDoubleDelivery перевіряє SPEC-04 §3: конкурентні
// воркери, що одночасно опитують чергу (FOR UPDATE SKIP LOCKED), обробляють
// кожну доставку рівно один раз — жодного подвійного виконання чи втрати.
func TestLeaseFencingPreventsDoubleDelivery(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	engine := automation.NewEngine(pool, newTestLogger())

	var mu sync.Mutex
	seen := make(map[string]int)
	engine.RegisterHandler("trigger.core.after_revision_committed", func(ctx context.Context, tx pgx.Tx, env automation.Envelope) error {
		time.Sleep(5 * time.Millisecond) // імітація роботи обробника для ширшого вікна перегонів
		mu.Lock()
		seen[env.EventKey+env.CorrelationID.String()]++
		mu.Unlock()
		return nil
	})

	const total = 20
	for i := 0; i < total; i++ {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("початок транзакції %d: %v", i, err)
		}
		correlationID := uuid.New()
		if err := automation.EmitEvent(ctx, tx, "wp.revision_committed", nil, nil, correlationID, map[string]any{
			"work_product_id": uuid.New().String(), "revision_id": uuid.New().String(),
		}); err != nil {
			t.Fatalf("emit %d: %v", i, err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatalf("commit %d: %v", i, err)
		}
	}
	if n, err := engine.DispatchPending(ctx); err != nil || n != total {
		t.Fatalf("диспетчеризація: n=%d err=%v", n, err)
	}

	var wg sync.WaitGroup
	var totalProcessed atomic.Int64
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				n, err := engine.ClaimAndProcess(ctx)
				if err != nil {
					t.Errorf("конкурентна обробка: %v", err)
					return
				}
				totalProcessed.Add(int64(n))
				if n == 0 {
					return
				}
			}
		}()
	}
	wg.Wait()

	if totalProcessed.Load() != total {
		t.Fatalf("очікувалося опрацювати %d доставок, оброблено %d", total, totalProcessed.Load())
	}
	mu.Lock()
	defer mu.Unlock()
	for key, count := range seen {
		if count != 1 {
			t.Fatalf("доставка %s оброблена %d разів замість 1 — подвійна доставка", key, count)
		}
	}
	if len(seen) != total {
		t.Fatalf("очікувалося %d унікальних доставок, отримано %d", total, len(seen))
	}
}
