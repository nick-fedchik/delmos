// Диспетчер доставки подій: генерація event_deliveries з активних підписок
// та Lease Fencing воркерів (SPEC-04 §2-3, ADR-007). Обробники прив'язуються
// ззовні через RegisterHandler — рушій не знає про предметні домени.
package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HandlerFunc — обробник "after"-тригера: виконується у власній транзакції
// разом з підтвердженням доставки (атомарність бізнес-ефекту й ack).
type HandlerFunc func(ctx context.Context, tx pgx.Tx, env Envelope) error

const (
	defaultLeaseDuration = 30 * time.Second
	defaultBatchSize     = 10
)

// Engine — фоновий диспетчер + реєстр обробників "after"-тригерів.
type Engine struct {
	pool          *pgxpool.Pool
	logger        *slog.Logger
	handlers      map[string]HandlerFunc
	leaseDuration time.Duration
	batchSize     int
}

// NewEngine створює диспетчер над спільним пулом з'єднань застосунку.
func NewEngine(pool *pgxpool.Pool, logger *slog.Logger) *Engine {
	return &Engine{
		pool:          pool,
		logger:        logger,
		handlers:      make(map[string]HandlerFunc),
		leaseDuration: defaultLeaseDuration,
		batchSize:     defaultBatchSize,
	}
}

// RegisterHandler прив'язує обробник до trigger_key (наприклад,
// "trigger.core.after_revision_committed"). Виклик до старту Run.
func (e *Engine) RegisterHandler(triggerKey string, handler HandlerFunc) {
	e.handlers[triggerKey] = handler
}

// SetLeaseDuration перевизначає тривалість оренди доставки (за замовчуванням
// 30с) — конфігурується через automation.lease_seconds.
func (e *Engine) SetLeaseDuration(d time.Duration) {
	if d > 0 {
		e.leaseDuration = d
	}
}

// SetBatchSize перевизначає розмір пачки захоплення доставок за один tick.
func (e *Engine) SetBatchSize(n int) {
	if n > 0 {
		e.batchSize = n
	}
}

// DispatchPending створює event_deliveries для ще не розподілених рядків
// outbox згідно з активними підписками та позначає їх dispatched_at.
// Повертає кількість оброблених рядків outbox.
func (e *Engine) DispatchPending(ctx context.Context) (int, error) {
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("початок транзакції диспетчеризації: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
		SELECT id, event_key FROM core.event_outbox
		WHERE dispatched_at IS NULL
		ORDER BY created_at ASC
		LIMIT 100
		FOR UPDATE SKIP LOCKED`)
	if err != nil {
		return 0, fmt.Errorf("вибірка нерозподілених подій: %w", err)
	}
	type outboxRow struct {
		ID       uuid.UUID
		EventKey string
	}
	var pending []outboxRow
	for rows.Next() {
		var r outboxRow
		if err := rows.Scan(&r.ID, &r.EventKey); err != nil {
			rows.Close()
			return 0, fmt.Errorf("розбір нерозподіленої події: %w", err)
		}
		pending = append(pending, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	for _, r := range pending {
		if _, err := tx.Exec(ctx, `
			INSERT INTO core.event_deliveries (outbox_event_id, subscription_id)
			SELECT $1, ts.id FROM core.trigger_subscriptions ts
			WHERE ts.event_key = $2 AND ts.active`,
			r.ID, r.EventKey); err != nil {
			return 0, fmt.Errorf("створення доставок для події %s: %w", r.EventKey, err)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE core.event_outbox SET dispatched_at = now() WHERE id = $1`, r.ID); err != nil {
			return 0, fmt.Errorf("позначення події %s розподіленою: %w", r.EventKey, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("фіксація диспетчеризації: %w", err)
	}
	return len(pending), nil
}

type claimedDelivery struct {
	DeliveryID uuid.UUID
	EventKey   string
	TriggerKey string
	Envelope   Envelope
}

// ClaimAndProcess захоплює пачку готових доставок (Lease Fencing, SPEC-04 §3)
// і обробляє кожну у власній транзакції разом з ack. Повертає кількість
// успішно оброблених доставок.
func (e *Engine) ClaimAndProcess(ctx context.Context) (int, error) {
	workerToken := uuid.New()

	rows, err := e.pool.Query(ctx, `
		UPDATE core.event_deliveries
		SET status = 'leased',
		    lease_token = $1,
		    lease_until = now() + make_interval(secs => $2),
		    delivery_attempts = delivery_attempts + 1
		WHERE id IN (
		    SELECT id FROM core.event_deliveries
		    WHERE status = 'pending' OR (status = 'leased' AND lease_until < now())
		    ORDER BY created_at ASC
		    LIMIT $3
		    FOR UPDATE SKIP LOCKED
		)
		RETURNING id`,
		workerToken, e.leaseDuration.Seconds(), e.batchSize)
	if err != nil {
		return 0, fmt.Errorf("захоплення оренди доставок: %w", err)
	}
	var claimedIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, fmt.Errorf("розбір захопленої доставки: %w", err)
		}
		claimedIDs = append(claimedIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(claimedIDs) == 0 {
		return 0, nil
	}

	details, err := e.loadDeliveryDetails(ctx, claimedIDs)
	if err != nil {
		return 0, err
	}

	processed := 0
	for _, d := range details {
		if err := e.processOne(ctx, d, workerToken); err != nil {
			e.logger.Error("обробка доставки завершилася помилкою", "delivery_id", d.DeliveryID, "trigger_key", d.TriggerKey, "error", err)
			continue
		}
		processed++
	}
	return processed, nil
}

func (e *Engine) loadDeliveryDetails(ctx context.Context, ids []uuid.UUID) ([]claimedDelivery, error) {
	rows, err := e.pool.Query(ctx, `
		SELECT ed.id, eo.event_key, ts.trigger_key, eo.envelope
		FROM core.event_deliveries ed
		JOIN core.event_outbox eo ON eo.id = ed.outbox_event_id
		JOIN core.trigger_subscriptions ts ON ts.id = ed.subscription_id
		WHERE ed.id = ANY($1)`, ids)
	if err != nil {
		return nil, fmt.Errorf("читання деталей доставок: %w", err)
	}
	defer rows.Close()

	var details []claimedDelivery
	for rows.Next() {
		var d claimedDelivery
		var envelopeJSON []byte
		if err := rows.Scan(&d.DeliveryID, &d.EventKey, &d.TriggerKey, &envelopeJSON); err != nil {
			return nil, fmt.Errorf("розбір деталей доставки: %w", err)
		}
		if err := json.Unmarshal(envelopeJSON, &d.Envelope); err != nil {
			return nil, fmt.Errorf("розбір конверта доставки %s: %w", d.DeliveryID, err)
		}
		details = append(details, d)
	}
	return details, rows.Err()
}

// processOne виконує зареєстрований обробник і підтвердження доставки
// (ack) в одній транзакції: бізнес-ефект і статус "delivered" атомарні.
// При помилці обробника — окремим autocommit-запитом переводить доставку в
// "pending" (негайний повторний retry) або "dead_letter" після max_attempts.
func (e *Engine) processOne(ctx context.Context, d claimedDelivery, workerToken uuid.UUID) error {
	handler, exists := e.handlers[d.TriggerKey]
	if !exists {
		e.markFailed(ctx, d.DeliveryID, fmt.Sprintf("немає зареєстрованого обробника для %s", d.TriggerKey))
		return fmt.Errorf("немає обробника для тригера %s", d.TriggerKey)
	}

	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("початок транзакції обробки доставки: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := handler(ctx, tx, d.Envelope); err != nil {
		e.markFailed(ctx, d.DeliveryID, err.Error())
		return fmt.Errorf("обробник %s: %w", d.TriggerKey, err)
	}

	tag, err := tx.Exec(ctx,
		`UPDATE core.event_deliveries SET status = 'delivered', completed_at = now(), lease_token = NULL
		 WHERE id = $1 AND lease_token = $2`,
		d.DeliveryID, workerToken)
	if err != nil {
		return fmt.Errorf("підтвердження доставки %s: %w", d.DeliveryID, err)
	}
	if tag.RowsAffected() == 0 {
		// Оренду перехоплено іншим воркером (закінчився lease_until) —
		// ефект обробника відкочується разом з транзакцією, дублювання немає.
		return fmt.Errorf("оренду доставки %s втрачено до підтвердження", d.DeliveryID)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("фіксація доставки %s: %w", d.DeliveryID, err)
	}
	return nil
}

// markFailed переводить доставку в pending (негайний retry) або dead_letter
// після вичерпання max_attempts (SPEC-04 §3, dead-letter handling).
func (e *Engine) markFailed(ctx context.Context, deliveryID uuid.UUID, reason string) {
	if _, err := e.pool.Exec(ctx, `
		UPDATE core.event_deliveries
		SET status = CASE WHEN delivery_attempts >= max_attempts THEN 'dead_letter' ELSE 'pending' END,
		    last_error = $2,
		    lease_token = NULL
		WHERE id = $1`, deliveryID, reason); err != nil {
		e.logger.Error("не вдалося зафіксувати збій доставки", "delivery_id", deliveryID, "error", err)
	}
}

// DeliveryStatusCounts — кількість доставок за кожним статусом (спостереження).
type DeliveryStatusCounts map[string]int

// DeadLetterEntry — доставка, що вичерпала max_attempts (SPEC-04 §3).
type DeadLetterEntry struct {
	DeliveryID       uuid.UUID `json:"delivery_id"`
	EventKey         string    `json:"event_key"`
	TriggerKey       string    `json:"trigger_key"`
	DeliveryAttempts int       `json:"delivery_attempts"`
	LastError        string    `json:"last_error"`
	CreatedAt        time.Time `json:"created_at"`
}

// ObservabilitySnapshot — стан черги для адміністративного ендпоінта
// спостереження (RUNBOOK.md, ADM-007): кількості за статусом і перелік
// dead-letter доставок, що потребують розслідування.
type ObservabilitySnapshot struct {
	Counts      DeliveryStatusCounts `json:"counts"`
	DeadLetters []DeadLetterEntry    `json:"dead_letters"`
}

// Snapshot читає поточний стан черги доставок для спостереження адміністратором.
func (e *Engine) Snapshot(ctx context.Context) (ObservabilitySnapshot, error) {
	snapshot := ObservabilitySnapshot{Counts: make(DeliveryStatusCounts)}

	rows, err := e.pool.Query(ctx, `SELECT status, count(*) FROM core.event_deliveries GROUP BY status`)
	if err != nil {
		return snapshot, fmt.Errorf("підрахунок доставок за статусом: %w", err)
	}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			rows.Close()
			return snapshot, fmt.Errorf("розбір лічильника доставок: %w", err)
		}
		snapshot.Counts[status] = count
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return snapshot, err
	}

	deadRows, err := e.pool.Query(ctx, `
		SELECT ed.id, eo.event_key, ts.trigger_key, ed.delivery_attempts, coalesce(ed.last_error, ''), ed.created_at
		FROM core.event_deliveries ed
		JOIN core.event_outbox eo ON eo.id = ed.outbox_event_id
		JOIN core.trigger_subscriptions ts ON ts.id = ed.subscription_id
		WHERE ed.status = 'dead_letter'
		ORDER BY ed.created_at DESC
		LIMIT 100`)
	if err != nil {
		return snapshot, fmt.Errorf("читання dead-letter доставок: %w", err)
	}
	defer deadRows.Close()
	for deadRows.Next() {
		var entry DeadLetterEntry
		if err := deadRows.Scan(&entry.DeliveryID, &entry.EventKey, &entry.TriggerKey, &entry.DeliveryAttempts, &entry.LastError, &entry.CreatedAt); err != nil {
			return snapshot, fmt.Errorf("розбір dead-letter доставки: %w", err)
		}
		snapshot.DeadLetters = append(snapshot.DeadLetters, entry)
	}
	return snapshot, deadRows.Err()
}

// Run запускає нескінченний цикл диспетчеризації й обробки з опитуванням
// заданого інтервалу, поки контекст не буде скасовано (graceful shutdown).
func (e *Engine) Run(ctx context.Context, pollInterval time.Duration) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := e.DispatchPending(ctx); err != nil {
				e.logger.Error("диспетчеризація подій завершилася помилкою", "error", err)
			}
			if _, err := e.ClaimAndProcess(ctx); err != nil {
				e.logger.Error("обробка доставок завершилася помилкою", "error", err)
			}
		}
	}
}
