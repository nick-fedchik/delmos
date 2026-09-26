package metrics

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"delmos/internal/migrate"
	"delmos/internal/testsupport"
)

type fixture struct {
	pool    *pgxpool.Pool
	store   *Store
	project uuid.UUID
	author  uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, testsupport.NewDatabase(t))
	if err != nil {
		t.Fatalf("пул підключень: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := migrate.Apply(ctx, pool, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatalf("міграції: %v", err)
	}

	f := &fixture{pool: pool, store: New(pool)}
	err = pool.QueryRow(ctx,
		`INSERT INTO core.users (login, display_name, password_hash, is_active)
		 VALUES ('metrics'::citext, 'metrics'::text, 'x', true) RETURNING id`).Scan(&f.author)
	if err != nil {
		t.Fatalf("створення користувача: %v", err)
	}
	err = pool.QueryRow(ctx,
		`INSERT INTO core.projects (code, name, description, status, created_by)
		 VALUES ('MET-1', 'Метрики', '', 'active', $1) RETURNING id`, f.author).Scan(&f.project)
	if err != nil {
		t.Fatalf("створення проєкту: %v", err)
	}
	return f
}

// inTx виконує дію в транзакції й фіксує її.
func (f *fixture) inTx(t *testing.T, fn func(tx pgx.Tx) error) error {
	t.Helper()
	ctx := context.Background()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("початок транзакції: %v", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("фіксація транзакції: %v", err)
	}
	return nil
}

func (f *fixture) record(t *testing.T, obs Observation) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := f.inTx(t, func(tx pgx.Tx) error {
		var e error
		id, e = Record(context.Background(), tx, obs)
		return e
	})
	if err != nil {
		t.Fatalf("запис вимірювання: %v", err)
	}
	return id
}

func ptr(s string) *string { return &s }

// Реєстр має бути засіяний визначеннями з METRICS.md §5, інакше обчислювачі
// не матимуть куди писати.
func TestSeededDefinitionsAreTyped(t *testing.T) {
	f := newFixture(t)
	def, err := loadDefinition(context.Background(), f.pool, "economics.cpi")
	if err != nil {
		t.Fatalf("читання визначення: %v", err)
	}
	if def.ValueType != TypeDecimal {
		t.Errorf("тип CPI = %q, очікувано decimal", def.ValueType)
	}
	if def.Unit != "ratio" {
		t.Errorf("одиниця CPI = %q, очікувано ratio", def.Unit)
	}

	ev, err := loadDefinition(context.Background(), f.pool, "economics.ev")
	if err != nil {
		t.Fatalf("читання визначення EV: %v", err)
	}
	if ev.Unit != "currency:EUR" {
		t.Errorf("одиниця EV = %q, очікувано currency:EUR", ev.Unit)
	}
	if !ev.RequiredForGate {
		t.Error("EV має бути обов'язковою для фазового шлюзу")
	}
}

// SWR-21.1: невідома метрика не створюється на льоту. Інакше друкарська
// помилка в ключі тихо породжувала б метрику-привида.
func TestUnknownMetricRejected(t *testing.T) {
	f := newFixture(t)
	err := f.inTx(t, func(tx pgx.Tx) error {
		_, e := Record(context.Background(), tx, Observation{
			MetricKey: "economics.cpu", ProjectID: f.project,
			Quality: QualityValid, Value: ptr("1.0"),
		})
		return e
	})
	if err == nil {
		t.Fatal("очікувано відмову для незареєстрованої метрики")
	}
	if !strings.Contains(err.Error(), "не зареєстровано") {
		t.Errorf("несподівана помилка: %v", err)
	}
}

// SWR-21.1, ключова перевірка: заборону нетипізованого значення тримає СУБД.
// Тест обходить Go і пише прямим SQL — якщо інваріант живе лише в коді,
// цей вставний запит пройде і тест впаде.
func TestTypeMismatchRejectedByDatabase(t *testing.T) {
	f := newFixture(t)

	// economics.cpi оголошено як decimal, пробуємо записати його як boolean.
	_, err := f.pool.Exec(context.Background(),
		`INSERT INTO core.metric_observations
		   (metric_key, value_type, project_id, quality, value_boolean)
		 VALUES ('economics.cpi', 'boolean', $1, 'valid', true)`, f.project)
	if err == nil {
		t.Fatal("СУБД прийняла значення типу, не оголошеного для метрики")
	}
	if !strings.Contains(err.Error(), "metric_definitions") && !strings.Contains(err.Error(), "foreign key") {
		t.Errorf("очікувано порушення зовнішнього ключа, отримано: %v", err)
	}

	// Правильний value_type, але значення покладено в чужу колонку.
	_, err = f.pool.Exec(context.Background(),
		`INSERT INTO core.metric_observations
		   (metric_key, value_type, project_id, quality, value_boolean)
		 VALUES ('economics.cpi', 'decimal', $1, 'valid', true)`, f.project)
	if err == nil {
		t.Fatal("СУБД прийняла значення в колонці, що не відповідає типу")
	}
	if !strings.Contains(err.Error(), "typed_value_check") {
		t.Errorf("очікувано metric_observations_typed_value_check, отримано: %v", err)
	}
}

// METRICS.md §6: відсутність даних ніколи не є нулем. no_data з числом —
// саме та підміна, яку заборонено.
func TestNoDataCannotCarryValue(t *testing.T) {
	f := newFixture(t)
	_, err := f.pool.Exec(context.Background(),
		`INSERT INTO core.metric_observations
		   (metric_key, value_type, project_id, quality, value_decimal, detail)
		 VALUES ('economics.cpi', 'decimal', $1, 'no_data', 0, 'кошторис відсутній')`, f.project)
	if err == nil {
		t.Fatal("СУБД дозволила записати значення для якості no_data")
	}
	if !strings.Contains(err.Error(), "absent_value_check") {
		t.Errorf("очікувано metric_observations_absent_value_check, отримано: %v", err)
	}
}

// Стан no_data та error без пояснення неможливо розслідувати.
func TestErrorRequiresDetail(t *testing.T) {
	f := newFixture(t)
	_, err := f.pool.Exec(context.Background(),
		`INSERT INTO core.metric_observations (metric_key, value_type, project_id, quality)
		 VALUES ('economics.cpi', 'decimal', $1, 'error')`, f.project)
	if err == nil {
		t.Fatal("СУБД дозволила помилкове вимірювання без пояснення")
	}
	if !strings.Contains(err.Error(), "detail_check") {
		t.Errorf("очікувано metric_observations_detail_check, отримано: %v", err)
	}
}

// SWR-22.2: вимірювання незмінне. Перевіряємо саме на рівні СУБД.
func TestObservationIsImmutable(t *testing.T) {
	f := newFixture(t)
	id := f.record(t, Observation{
		MetricKey: "economics.cpi", ProjectID: f.project,
		Quality: QualityValid, Value: ptr("0.85"),
	})

	_, err := f.pool.Exec(context.Background(),
		`UPDATE core.metric_observations SET value_decimal = 1.20 WHERE id = $1`, id)
	if err == nil {
		t.Fatal("вимірювання вдалося змінити")
	}
	if !strings.Contains(err.Error(), "незмінн") {
		t.Errorf("очікувано відмову тригера незмінності, отримано: %v", err)
	}

	_, err = f.pool.Exec(context.Background(),
		`DELETE FROM core.metric_observations WHERE id = $1`, id)
	if err == nil {
		t.Fatal("вимірювання вдалося видалити")
	}
}

// Якість перераховується на момент читання: значення, що пережило поріг,
// не має вдавати чинне.
func TestValidObservationDecaysToStale(t *testing.T) {
	f := newFixture(t)
	computed := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	f.record(t, Observation{
		MetricKey: "economics.ev", ProjectID: f.project, PhaseKey: "design",
		Quality: QualityValid, Value: ptr("1000.00"), ComputedAt: computed,
	})

	ctx := context.Background()
	// Поріг для economics.ev — 86400 с.
	fresh, found, err := Latest(ctx, f.pool, f.project, "economics.ev", "design", computed.Add(time.Hour))
	if err != nil || !found {
		t.Fatalf("читання свіжого вимірювання: err=%v found=%v", err, found)
	}
	if fresh.Quality != QualityValid {
		t.Errorf("через годину якість = %q, очікувано valid", fresh.Quality)
	}

	aged, _, err := Latest(ctx, f.pool, f.project, "economics.ev", "design", computed.Add(48*time.Hour))
	if err != nil {
		t.Fatalf("читання застарілого вимірювання: %v", err)
	}
	if aged.Quality != QualityStale {
		t.Errorf("через 48 годин якість = %q, очікувано stale", aged.Quality)
	}
}

// Latest розрізняє рівень проєкту й рівень фази: інакше показник однієї фази
// підмінював би показник іншої.
func TestLatestIsScopedByPhase(t *testing.T) {
	f := newFixture(t)
	now := time.Now().UTC()
	f.record(t, Observation{MetricKey: "economics.ac", ProjectID: f.project, PhaseKey: "design",
		Quality: QualityValid, Value: ptr("100.00"), ComputedAt: now})
	f.record(t, Observation{MetricKey: "economics.ac", ProjectID: f.project, PhaseKey: "build",
		Quality: QualityValid, Value: ptr("700.00"), ComputedAt: now})

	obs, found, err := Latest(context.Background(), f.pool, f.project, "economics.ac", "build", now)
	if err != nil || !found {
		t.Fatalf("читання: err=%v found=%v", err, found)
	}
	if obs.Value == nil || *obs.Value != "700.000000" {
		t.Errorf("значення фази build = %v, очікувано 700.000000", obs.Value)
	}
}

// SWR-22.3: відсутні, застарілі та помилкові вимірювання блокують шлюз.
func TestGateBlockersDetectBadQuality(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()

	// Жодного вимірювання — блокують усі три обов'язкові метрики.
	blockers, err := GateBlockers(ctx, f.pool, f.project, "design", now)
	if err != nil {
		t.Fatalf("перевірка блокувань: %v", err)
	}
	if len(blockers) != 3 {
		t.Fatalf("без даних блокувань = %v, очікувано 3 (ac, ev, pv)", blockers)
	}

	f.record(t, Observation{MetricKey: "economics.pv", ProjectID: f.project, PhaseKey: "design",
		Quality: QualityValid, Value: ptr("5000.00"), ComputedAt: now})
	f.record(t, Observation{MetricKey: "economics.ev", ProjectID: f.project, PhaseKey: "design",
		Quality: QualityValid, Value: ptr("2500.00"), ComputedAt: now})
	// AC не порахувався — це не нуль, це помилка.
	f.record(t, Observation{MetricKey: "economics.ac", ProjectID: f.project, PhaseKey: "design",
		Quality: QualityError, Detail: "ставка праці не визначена", ComputedAt: now})

	blockers, err = GateBlockers(ctx, f.pool, f.project, "design", now)
	if err != nil {
		t.Fatalf("перевірка блокувань: %v", err)
	}
	if len(blockers) != 1 || blockers[0] != "economics.ac" {
		t.Fatalf("блокування = %v, очікувано [economics.ac]", blockers)
	}

	// Заміщуємо помилку валідним вимірюванням — шлюз відкривається.
	f.record(t, Observation{MetricKey: "economics.ac", ProjectID: f.project, PhaseKey: "design",
		Quality: QualityValid, Value: ptr("3000.00"), ComputedAt: now.Add(time.Second)})

	blockers, err = GateBlockers(ctx, f.pool, f.project, "design", now.Add(time.Second))
	if err != nil {
		t.Fatalf("перевірка блокувань: %v", err)
	}
	if len(blockers) != 0 {
		t.Fatalf("після виправлення блокування = %v, очікувано порожньо", blockers)
	}
}
