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
		        o.id, o.metric_key, o.value_type, o.unit, o.project_id, COALESCE(o.phase_key, ''), o.quality,
		        COALESCE(o.value_integer::text, o.value_decimal::text, o.value_duration::text,
		                 o.value_boolean::text, o.value_enum, o.value_distribution::text),
		        COALESCE(o.detail, ''), o.computed_at, d.max_age_seconds,
		        o.source_version, o.baseline_id, o.baseline_version, o.config_generation,
		        COALESCE(iv.version, 0), cb.id, cb.version, b.config_generation
		 FROM core.metric_observations o
		 JOIN core.metric_definitions d ON d.metric_key = o.metric_key
		 LEFT JOIN core.metric_input_versions iv ON iv.project_id = o.project_id
		 LEFT JOIN core.cost_baselines cb ON cb.project_id = o.project_id AND cb.status = 'approved'
		 LEFT JOIN core.project_plan_bindings b ON b.project_id = o.project_id
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
		var currentVersion int64
		var currentBaseline *uuid.UUID
		var currentBaselineVersion *int
		var currentGeneration *int64
		if err := rows.Scan(&obs.ID, &obs.MetricKey, &obs.ValueType, &obs.Unit, &obs.ProjectID, &obs.PhaseKey,
			&obs.Quality, &obs.Value, &obs.Detail, &obs.ComputedAt, &maxAge,
			&obs.SourceVersion, &obs.BaselineID, &obs.BaselineVersion, &obs.ConfigGeneration,
			&currentVersion, &currentBaseline, &currentBaselineVersion, &currentGeneration); err != nil {
			return nil, fmt.Errorf("розбір вимірювання: %w", err)
		}
		if obs.Quality == QualityValid && (isStale(obs.ComputedAt, maxAge, asOf) || obs.SourceVersion != currentVersion ||
			!sameOptional(obs.BaselineID, currentBaseline) || !sameOptional(obs.BaselineVersion, currentBaselineVersion) ||
			!sameOptional(obs.ConfigGeneration, currentGeneration)) {
			obs.Quality = QualityStale
		}
		observations = append(observations, obs)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("обхід вимірювань проєкту: %w", err)
	}
	return observations, nil
}
