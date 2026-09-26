package metrics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"delmos/internal/economics"
)

// Collector обчислює метрики здобутої цінності і фіксує їх як незмінні
// вимірювання. Він працює у фоновому обробнику подій, а не на шляху запиту
// (SWR-22.1): браузерний віджет лише споживає вимірювання й ніколи не є
// джерелом істини.
type Collector struct {
	source *economics.Store
}

func NewCollector(source *economics.Store) *Collector {
	return &Collector{source: source}
}

// Метрики рівня фази, обов'язкові для шлюзу.
var phaseMetrics = []struct {
	key  string
	pick func(economics.PhaseEarnedValue) string
}{
	{"economics.pv", func(p economics.PhaseEarnedValue) string { return p.PlannedValue }},
	{"economics.ac", func(p economics.PhaseEarnedValue) string { return p.ActualCost }},
	{"economics.ev", func(p economics.PhaseEarnedValue) string { return p.EarnedValue }},
}

// CollectEconomics перераховує показники проєкту на момент asOf.
//
// Відсутність затвердженого кошторису не пропускається мовчки: вона
// фіксується як no_data. Мовчазний пропуск лишив би попереднє вимірювання
// найсвіжішим, і шлюз відкрився б за даними, що більше не мають підстави.
func (c *Collector) CollectEconomics(ctx context.Context, tx pgx.Tx, projectID, correlationID uuid.UUID, asOf time.Time) error {
	snapshot, err := c.source.ComputeEarnedValue(ctx, projectID, asOf)
	if errors.Is(err, economics.ErrNoApprovedBaseline) {
		return c.recordUnavailable(ctx, tx, projectID, correlationID, asOf,
			QualityNoData, "затверджений кошторис відсутній")
	}
	if err != nil {
		return c.recordUnavailable(ctx, tx, projectID, correlationID, asOf,
			QualityError, fmt.Sprintf("обчислення здобутої цінності: %v", err))
	}

	for _, phase := range snapshot.Phases {
		for _, m := range phaseMetrics {
			value := m.pick(phase)
			if _, err := Record(ctx, tx, Observation{
				MetricKey: m.key, ProjectID: projectID, PhaseKey: phase.PhaseKey,
				Quality: QualityValid, Value: &value,
				ComputedAt: asOf, CorrelationID: correlationID,
			}); err != nil {
				return err
			}
		}
	}

	// Рівень проєкту: відхилення завжди визначені, індекси — ні.
	projectLevel := []struct {
		key   string
		value *string
	}{
		{"economics.cv", &snapshot.CostVariance},
		{"economics.sv", &snapshot.ScheduleVariance},
		{"economics.cpi", snapshot.CostPerformanceIndex},
		{"economics.spi", snapshot.SchedulePerformanceIndex},
	}
	for _, m := range projectLevel {
		obs := Observation{
			MetricKey: m.key, ProjectID: projectID,
			ComputedAt: asOf, CorrelationID: correlationID,
		}
		if m.value == nil {
			// Нульовий знаменник означає «немає даних для обчислення», а не
			// «показник дорівнює нулю» (METRICS.md §6).
			obs.Quality = QualityNoData
			obs.Detail = "знаменник нульовий: здобутої цінності ще немає"
		} else {
			obs.Quality = QualityValid
			obs.Value = m.value
		}
		if _, err := Record(ctx, tx, obs); err != nil {
			return err
		}
	}
	return nil
}

// recordUnavailable фіксує неможливість обчислення для всіх фаз проєкту.
func (c *Collector) recordUnavailable(ctx context.Context, tx pgx.Tx, projectID, correlationID uuid.UUID, asOf time.Time, quality, detail string) error {
	rows, err := tx.Query(ctx,
		`SELECT phase_key FROM core.project_phases WHERE project_id = $1 ORDER BY phase_key`, projectID)
	if err != nil {
		return fmt.Errorf("читання фаз проєкту: %w", err)
	}
	var phaseKeys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			rows.Close()
			return fmt.Errorf("розбір ключа фази: %w", err)
		}
		phaseKeys = append(phaseKeys, key)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("обхід фаз проєкту: %w", err)
	}

	for _, phaseKey := range phaseKeys {
		for _, m := range phaseMetrics {
			if _, err := Record(ctx, tx, Observation{
				MetricKey: m.key, ProjectID: projectID, PhaseKey: phaseKey,
				Quality: quality, Detail: detail,
				ComputedAt: asOf, CorrelationID: correlationID,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

// HandleEvent — обробник фонового диспетчера для
// trigger.core.after_economics_changed.
func (c *Collector) HandleEvent(ctx context.Context, tx pgx.Tx, projectID, correlationID uuid.UUID) error {
	return c.CollectEconomics(ctx, tx, projectID, correlationID, time.Now().UTC())
}
