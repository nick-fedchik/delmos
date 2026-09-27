// Package metrics реалізує типізований реєстр метрик та незмінні вимірювання
// (SWR-21, SWR-22).
//
// Значення метрики ніколи не зберігається як довільний JSON: воно лягає в
// колонку, що відповідає оголошеному типу, а відповідність типу тримає
// складений зовнішній ключ у СУБД.
package metrics

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Якість вимірювання (SWR-22.2).
const (
	QualityValid  = "valid"
	QualityStale  = "stale"
	QualityNoData = "no_data"
	QualityError  = "error"
)

// Типи значень (SWR-21.1).
const (
	TypeInteger      = "integer"
	TypeDecimal      = "decimal"
	TypeDuration     = "duration"
	TypeBoolean      = "boolean"
	TypeEnum         = "enum"
	TypeDistribution = "distribution"
)

var (
	ErrDefinitionNotFound = errors.New("метрику не зареєстровано")
	ErrValueTypeMismatch  = errors.New("значення не відповідає оголошеному типу метрики")
	ErrEnumValueNotFound  = errors.New("значення відсутнє серед допустимих для enum-метрики")
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Definition — оголошення метрики в реєстрі.
type Definition struct {
	MetricKey       string   `json:"metric_key"`
	Name            string   `json:"name"`
	ValueType       string   `json:"value_type"`
	Unit            string   `json:"unit"`
	OwnerModule     string   `json:"owner_module"`
	EnumValues      []string `json:"enum_values,omitempty"`
	MaxAgeSeconds   *int     `json:"max_age_seconds,omitempty"`
	RequiredForGate bool     `json:"required_for_gate"`
}

// Observation — зафіксоване вимірювання. Поле Value заповнюється лише для
// якостей valid та stale; для no_data та error значення не існує.
type Observation struct {
	ID               uuid.UUID  `json:"id"`
	MetricKey        string     `json:"metric_key"`
	ValueType        string     `json:"value_type"`
	Unit             string     `json:"unit"`
	ProjectID        uuid.UUID  `json:"project_id"`
	PhaseKey         string     `json:"phase_key,omitempty"`
	Quality          string     `json:"quality"`
	Value            *string    `json:"value,omitempty"`
	Detail           string     `json:"detail,omitempty"`
	ComputedAt       time.Time  `json:"computed_at"`
	SourceVersion    int64      `json:"source_version"`
	BaselineID       *uuid.UUID `json:"baseline_id,omitempty"`
	BaselineVersion  *int       `json:"baseline_version,omitempty"`
	ConfigGeneration *int64     `json:"config_generation,omitempty"`
	inputVersion     *int64
	// Кореляція з подією, що спричинила обчислення. Назовні не публікується.
	CorrelationID uuid.UUID `json:"-"`
}

// Record фіксує вимірювання. Значення передається рядком у десятковому
// записі: для грошей і відношень проміжне двійкове представлення неприйнятне,
// а приведення до потрібної колонки виконує PostgreSQL.
//
// Порожній value означає відсутність значення й допустимий лише для
// якостей no_data та error.
func Record(ctx context.Context, q Querier, obs Observation) (uuid.UUID, error) {
	def, err := loadDefinition(ctx, q, obs.MetricKey)
	if err != nil {
		return uuid.Nil, err
	}
	if obs.Quality == QualityValid || obs.Quality == QualityStale {
		if obs.Value == nil || *obs.Value == "" {
			return uuid.Nil, fmt.Errorf("%w: якість %q потребує значення", ErrValueTypeMismatch, obs.Quality)
		}
		if def.ValueType == TypeEnum && !containsValue(def.EnumValues, *obs.Value) {
			return uuid.Nil, fmt.Errorf("%w: %q", ErrEnumValueNotFound, *obs.Value)
		}
	}

	computedAt := obs.ComputedAt
	if computedAt.IsZero() {
		computedAt = time.Now().UTC()
	}
	unit := obs.Unit
	if unit == "" {
		unit = def.Unit
		if unit == "currency" {
			unit = "currency:EUR"
			if obs.BaselineID != nil {
				var currency string
				if err := q.QueryRow(ctx, `SELECT currency FROM core.cost_baselines WHERE id = $1`, *obs.BaselineID).Scan(&currency); err != nil {
					return uuid.Nil, err
				}
				unit = "currency:" + currency
			}
		}
	}
	if def.Unit != "currency" && unit != def.Unit ||
		def.Unit == "currency" && (len(unit) != 12 || !strings.HasPrefix(unit, "currency:")) {
		return uuid.Nil, fmt.Errorf("%w: одиниця %q несумісна з %q", ErrValueTypeMismatch, unit, def.Unit)
	}
	version := obs.inputVersion
	if version == nil {
		current, err := CurrentInputVersion(ctx, q, obs.ProjectID)
		if err != nil {
			return uuid.Nil, err
		}
		version = &current
	}

	var id uuid.UUID
	err = q.QueryRow(ctx,
		`INSERT INTO core.metric_observations
		   (metric_key, value_type, project_id, phase_key, quality,
		    value_integer, value_decimal, value_duration, value_boolean, value_enum, value_distribution,
		    detail, computed_at, correlation_id, source_version, baseline_id, baseline_version, config_generation, unit)
		 VALUES ($1, $2, $3, $4, $5,
		         CASE WHEN $2 = 'integer'      THEN ($6::text)::bigint   END,
		         CASE WHEN $2 = 'decimal'      THEN ($6::text)::numeric  END,
		         CASE WHEN $2 = 'duration'     THEN ($6::text)::interval END,
		         CASE WHEN $2 = 'boolean'      THEN ($6::text)::boolean  END,
		         CASE WHEN $2 = 'enum'         THEN  $6::text            END,
		         CASE WHEN $2 = 'distribution' THEN ($6::text)::jsonb    END,
		         $7, $8, $9, $10, $11, $12, $13, $14)
		 RETURNING id`,
		obs.MetricKey, def.ValueType, obs.ProjectID, nullIfEmpty(obs.PhaseKey), obs.Quality,
		obs.Value, nullIfEmpty(obs.Detail), computedAt, nullUUID(obs.CorrelationID),
		*version, obs.BaselineID, obs.BaselineVersion, obs.ConfigGeneration, unit).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("запис вимірювання %s: %w", obs.MetricKey, err)
	}
	return id, nil
}

// Latest повертає найсвіжіше вимірювання метрики в межах проєкту або фази.
// Якість перераховується на момент читання: вимірювання, що пережило
// max_age_seconds, повертається як stale, навіть якщо записувалося як valid.
// Інакше застаріле значення виглядало б чинним (SWR-22.2).
func Latest(ctx context.Context, q Querier, projectID uuid.UUID, metricKey, phaseKey string, asOf time.Time) (Observation, bool, error) {
	var obs Observation
	var phase *string
	var value *string
	var detail *string
	var maxAge *int
	var currentVersion int64
	var currentBaseline *uuid.UUID
	var currentBaselineVersion *int
	var currentGeneration *int64

	err := q.QueryRow(ctx,
		`SELECT o.id, o.metric_key, o.value_type, o.unit, o.project_id, o.phase_key, o.quality,
		        COALESCE(o.value_integer::text, o.value_decimal::text, o.value_duration::text,
		                 o.value_boolean::text, o.value_enum, o.value_distribution::text),
		        o.detail, o.computed_at, d.max_age_seconds, o.source_version,
		        o.baseline_id, o.baseline_version, o.config_generation,
		        COALESCE(iv.version, 0), cb.id, cb.version, b.config_generation
		 FROM core.metric_observations o
		 JOIN core.metric_definitions d ON d.metric_key = o.metric_key
		 LEFT JOIN core.metric_input_versions iv ON iv.project_id = o.project_id
		 LEFT JOIN core.cost_baselines cb ON cb.project_id = o.project_id AND cb.status = 'approved'
		 LEFT JOIN core.project_plan_bindings b ON b.project_id = o.project_id
		 WHERE o.project_id = $1 AND o.metric_key = $2
		   AND o.phase_key IS NOT DISTINCT FROM $3
		 ORDER BY o.computed_at DESC, o.created_at DESC
		 LIMIT 1`,
		projectID, metricKey, nullIfEmpty(phaseKey)).
		Scan(&obs.ID, &obs.MetricKey, &obs.ValueType, &obs.Unit, &obs.ProjectID, &phase, &obs.Quality,
			&value, &detail, &obs.ComputedAt, &maxAge, &obs.SourceVersion,
			&obs.BaselineID, &obs.BaselineVersion, &obs.ConfigGeneration, &currentVersion,
			&currentBaseline, &currentBaselineVersion, &currentGeneration)
	if errors.Is(err, pgx.ErrNoRows) {
		return Observation{}, false, nil
	}
	if err != nil {
		return Observation{}, false, fmt.Errorf("читання вимірювання %s: %w", metricKey, err)
	}

	if phase != nil {
		obs.PhaseKey = *phase
	}
	obs.Value = value
	if detail != nil {
		obs.Detail = *detail
	}
	if obs.Quality == QualityValid && (isStale(obs.ComputedAt, maxAge, asOf) || obs.SourceVersion != currentVersion ||
		!sameOptional(obs.BaselineID, currentBaseline) || !sameOptional(obs.BaselineVersion, currentBaselineVersion) ||
		!sameOptional(obs.ConfigGeneration, currentGeneration)) {
		obs.Quality = QualityStale
	}
	return obs, true, nil
}

func sameOptional[T comparable](first, second *T) bool {
	if first == nil || second == nil {
		return first == nil && second == nil
	}
	return *first == *second
}

func CurrentInputVersion(ctx context.Context, q Querier, projectID uuid.UUID) (int64, error) {
	var version int64
	err := q.QueryRow(ctx, `SELECT COALESCE((SELECT version FROM core.metric_input_versions
		WHERE project_id = $1), 0)`, projectID).Scan(&version)
	return version, err
}

// GateBlockers повертає метрики, що блокують фазовий шлюз: обов'язкові для
// шлюзу, але відсутні, застарілі чи помилкові (SWR-22.3).
func GateBlockers(ctx context.Context, q Querier, projectID uuid.UUID, phaseKey string, asOf time.Time) ([]string, error) {
	rows, err := q.Query(ctx,
		`SELECT metric_key FROM core.metric_definitions WHERE required_for_gate ORDER BY metric_key`)
	if err != nil {
		return nil, fmt.Errorf("читання обов'язкових для шлюзу метрик: %w", err)
	}
	var required []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			rows.Close()
			return nil, fmt.Errorf("розбір ключа метрики: %w", err)
		}
		required = append(required, key)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("обхід обов'язкових метрик: %w", err)
	}

	var blockers []string
	for _, key := range required {
		obs, found, err := Latest(ctx, q, projectID, key, phaseKey, asOf)
		if err != nil {
			return nil, err
		}
		// Відсутність вимірювання блокує так само, як застаріле: немає
		// підстав вважати шлюз пройденим за браку даних (METRICS.md §6).
		if !found || obs.Quality != QualityValid {
			blockers = append(blockers, key)
		}
	}
	return blockers, nil
}

// Querier дає змогу читати і з пулу, і з відкритої транзакції: перевірка
// шлюзу виконується всередині транзакції переходу фази.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Pool повертає пул для читань поза транзакцією.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

func loadDefinition(ctx context.Context, q Querier, metricKey string) (Definition, error) {
	var def Definition
	var enumValues []string
	err := q.QueryRow(ctx,
		`SELECT metric_key, name, value_type, unit, owner_module,
		        COALESCE(enum_values, '{}'), max_age_seconds, required_for_gate
		 FROM core.metric_definitions WHERE metric_key = $1`, metricKey).
		Scan(&def.MetricKey, &def.Name, &def.ValueType, &def.Unit, &def.OwnerModule,
			&enumValues, &def.MaxAgeSeconds, &def.RequiredForGate)
	if errors.Is(err, pgx.ErrNoRows) {
		return Definition{}, fmt.Errorf("%w: %s", ErrDefinitionNotFound, metricKey)
	}
	if err != nil {
		return Definition{}, fmt.Errorf("читання визначення метрики %s: %w", metricKey, err)
	}
	def.EnumValues = enumValues
	return def, nil
}

func isStale(computedAt time.Time, maxAgeSeconds *int, asOf time.Time) bool {
	if maxAgeSeconds == nil {
		return false
	}
	return asOf.Sub(computedAt) > time.Duration(*maxAgeSeconds)*time.Second
}

func containsValue(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}
