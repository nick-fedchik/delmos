package metrics

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ListLatest повертає останнє вимірювання кожної пари (метрика, фаза) проєкту.
//
// Якість перераховується на момент asOf, тож споживач бачить stale там, де
// значення вже не має підстави. Браузерний віджет лише відображає ці дані й
// ніколи не обчислює показники сам (SWR-22.1).
func (s *Store) ListLatest(ctx context.Context, projectID uuid.UUID, asOf time.Time) ([]Observation, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT DISTINCT ON (o.metric_key, o.phase_key)
		        o.id, o.metric_key, o.value_type, o.project_id, COALESCE(o.phase_key, ''), o.quality,
		        COALESCE(o.value_integer::text, o.value_decimal::text, o.value_duration::text,
		                 o.value_boolean::text, o.value_enum, o.value_distribution::text),
		        COALESCE(o.detail, ''), o.computed_at, d.max_age_seconds
		 FROM core.metric_observations o
		 JOIN core.metric_definitions d ON d.metric_key = o.metric_key
		 WHERE o.project_id = $1
		 ORDER BY o.metric_key, o.phase_key, o.computed_at DESC, o.created_at DESC`,
		projectID)
	if err != nil {
		return nil, fmt.Errorf("читання вимірювань проєкту: %w", err)
	}
	defer rows.Close()

	observations := make([]Observation, 0)
	for rows.Next() {
		var obs Observation
		var maxAge *int
		if err := rows.Scan(&obs.ID, &obs.MetricKey, &obs.ValueType, &obs.ProjectID, &obs.PhaseKey,
			&obs.Quality, &obs.Value, &obs.Detail, &obs.ComputedAt, &maxAge); err != nil {
			return nil, fmt.Errorf("розбір вимірювання: %w", err)
		}
		if obs.Quality == QualityValid && isStale(obs.ComputedAt, maxAge, asOf) {
			obs.Quality = QualityStale
		}
		observations = append(observations, obs)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("обхід вимірювань проєкту: %w", err)
	}
	return observations, nil
}
