// Секундний планувальник (SWR-24, EVENTS_TRIGGERS_RULES.md §3): атомарний
// запис ScheduleOccurrence разом з публікацією події в outbox у транзакції.
// Захист від дублювання — унікальний (schedule_id, occurrence_time_utc).
package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	modeOnce       = "once"
	modeFixedRate  = "fixed_rate"
	modeFixedDelay = "fixed_delay"
	modeCalendar   = "calendar"
)

// scheduleSpec — вміст jsonb-колонки spec; поля залежать від mode.
//   - once:        {"run_at": RFC3339}
//   - fixed_rate:  {"interval_seconds": N}
//   - fixed_delay: {"interval_seconds": N} (MVP: наступний запуск рахується
//     від часу спрацювання, а не від завершення роботи — справжній
//     delay-від-завершення потребує зворотного виклику про завершення,
//     що виходить за межі поточного зрізу двигуна).
//   - calendar:    {"cron": "хвилина година день_місяця місяць день_тижня"}
//     (мінімальний 5-польовий cron, НЕ повний RFC 5545 — свідоме звуження
//     обсягу; документовано в CHANGELOG).
type ScheduleSpec struct {
	RunAt           string `json:"run_at,omitempty"`
	IntervalSeconds int64  `json:"interval_seconds,omitempty"`
	Cron            string `json:"cron,omitempty"`
}

type dueSchedule struct {
	ID   uuid.UUID
	Key  string
	Mode string
	Spec ScheduleSpec
}

// TickScheduler обробляє всі активні розклади з next_run_at <= now(),
// генеруючи по одному ScheduleOccurrence + events.schedule.occurrence_due на
// кожен розклад за виклик (наступний due-момент обробиться наступним tick).
// Повертає кількість оброблених розкладів.
func (e *Engine) TickScheduler(ctx context.Context) (int, error) {
	due, err := e.loadDueSchedules(ctx)
	if err != nil {
		return 0, err
	}

	processed := 0
	for _, s := range due {
		if err := e.fireSchedule(ctx, s); err != nil {
			e.logger.Error("обробка розкладу завершилася помилкою", "schedule_key", s.Key, "error", err)
			continue
		}
		processed++
	}
	return processed, nil
}

func (e *Engine) loadDueSchedules(ctx context.Context) ([]dueSchedule, error) {
	rows, err := e.pool.Query(ctx, `
		SELECT id, key, mode, spec FROM core.schedule_definitions
		WHERE active AND (next_run_at IS NULL OR next_run_at <= now())
		ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("читання активних розкладів: %w", err)
	}
	defer rows.Close()

	var result []dueSchedule
	for rows.Next() {
		var s dueSchedule
		var specJSON []byte
		if err := rows.Scan(&s.ID, &s.Key, &s.Mode, &specJSON); err != nil {
			return nil, fmt.Errorf("розбір розкладу: %w", err)
		}
		if err := json.Unmarshal(specJSON, &s.Spec); err != nil {
			return nil, fmt.Errorf("розбір специфікації розкладу %s: %w", s.Key, err)
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// fireSchedule атомарно фіксує ScheduleOccurrence, публікує подію та
// обчислює наступний next_run_at (чи деактивує розклад для "once").
func (e *Engine) fireSchedule(ctx context.Context, s dueSchedule) error {
	now := time.Now().UTC().Truncate(time.Second)

	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("початок транзакції розкладу %s: %w", s.Key, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx,
		`INSERT INTO core.schedule_occurrences (schedule_id, occurrence_time_utc)
		 VALUES ($1, $2) ON CONFLICT (schedule_id, occurrence_time_utc) DO NOTHING`,
		s.ID, now)
	if err != nil {
		return fmt.Errorf("запис настання розкладу %s: %w", s.Key, err)
	}
	if tag.RowsAffected() > 0 {
		if err := EmitEvent(ctx, tx, "schedule.occurrence_due", nil, nil, uuid.New(), map[string]any{
			"schedule_id": s.ID.String(), "schedule_key": s.Key, "occurrence_time_utc": now,
		}); err != nil {
			return err
		}
	}

	nextRunAt, active, err := computeNext(s, now)
	if err != nil {
		return fmt.Errorf("обчислення наступного запуску розкладу %s: %w", s.Key, err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE core.schedule_definitions SET next_run_at = $2, active = $3 WHERE id = $1`,
		s.ID, nextRunAt, active); err != nil {
		return fmt.Errorf("оновлення розкладу %s: %w", s.Key, err)
	}

	return tx.Commit(ctx)
}

// computeNext повертає наступний момент запуску та чи лишається розклад
// активним після цього спрацювання.
func computeNext(s dueSchedule, firedAt time.Time) (*time.Time, bool, error) {
	switch s.Mode {
	case modeOnce:
		return nil, false, nil
	case modeFixedRate, modeFixedDelay:
		if s.Spec.IntervalSeconds <= 0 {
			return nil, false, fmt.Errorf("interval_seconds має бути додатним для режиму %s", s.Mode)
		}
		next := firedAt.Add(time.Duration(s.Spec.IntervalSeconds) * time.Second)
		return &next, true, nil
	case modeCalendar:
		next, err := nextCronMatch(s.Spec.Cron, firedAt)
		if err != nil {
			return nil, false, err
		}
		return &next, true, nil
	default:
		return nil, false, fmt.Errorf("невідомий режим розкладу: %s", s.Mode)
	}
}

// EnsureInitialNextRun обчислює next_run_at при першому вмиканні розкладу
// (once з відкладеним run_at, або перший matching момент календаря).
func EnsureInitialNextRun(mode string, spec ScheduleSpec, from time.Time) (*time.Time, error) {
	switch mode {
	case modeOnce:
		if spec.RunAt == "" {
			t := from
			return &t, nil
		}
		t, err := time.Parse(time.RFC3339, spec.RunAt)
		if err != nil {
			return nil, fmt.Errorf("некоректний run_at: %w", err)
		}
		t = t.UTC()
		return &t, nil
	case modeFixedRate, modeFixedDelay:
		t := from
		return &t, nil
	case modeCalendar:
		t, err := nextCronMatch(spec.Cron, from)
		if err != nil {
			return nil, err
		}
		return &t, nil
	default:
		return nil, fmt.Errorf("невідомий режим розкладу: %s", mode)
	}
}

// CreateSchedule реєструє новий активний розклад і обчислює його перший
// next_run_at (SWR-24). Ключ має бути унікальним.
func (e *Engine) CreateSchedule(ctx context.Context, key, mode string, spec ScheduleSpec) (uuid.UUID, error) {
	nextRunAt, err := EnsureInitialNextRun(mode, spec, time.Now().UTC())
	if err != nil {
		return uuid.Nil, err
	}
	specJSON, err := json.Marshal(spec)
	if err != nil {
		return uuid.Nil, fmt.Errorf("серіалізація специфікації розкладу %s: %w", key, err)
	}
	var id uuid.UUID
	err = e.pool.QueryRow(ctx,
		`INSERT INTO core.schedule_definitions (key, mode, spec, next_run_at) VALUES ($1, $2, $3, $4) RETURNING id`,
		key, mode, specJSON, nextRunAt).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("створення розкладу %s: %w", key, err)
	}
	return id, nil
}
