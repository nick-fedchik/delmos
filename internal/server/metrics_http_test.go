package server

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"delmos/internal/automation"
	"delmos/internal/economics"
	"delmos/internal/metrics"
)

// Наскрізна перевірка SWR-22: запис економіки через HTTP -> подія в outbox ->
// фоновий обробник -> незмінні вимірювання -> читання через HTTP.
//
// Тест іде тим самим шляхом, що й користувач. Без нього повторилася б хиба,
// виявлена ревізією: механізм працює у внутрішніх викликах, але недосяжний у
// продукті, бо десь не зареєстровано обробник чи маршрут.

// drainAutomation синхронно виконує те, що в робочому процесі робить фоновий
// цикл диспетчера.
func drainAutomation(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	engine := automation.NewEngine(pool, testLogger())
	collector := metrics.NewCollector(economics.New(pool))
	engine.RegisterHandler("trigger.core.after_economics_changed",
		func(ctx context.Context, tx pgx.Tx, env automation.Envelope) error {
			if env.ScopeID == nil {
				t.Fatalf("подія %s без ідентифікатора проєкту", env.EventKey)
			}
			return collector.HandleEvent(ctx, tx, *env.ScopeID, env.CorrelationID)
		})

	ctx := context.Background()
	if _, err := engine.DispatchPending(ctx); err != nil {
		t.Fatalf("диспетчеризація подій: %v", err)
	}
	if _, err := engine.ClaimAndProcess(ctx); err != nil {
		t.Fatalf("обробка доставок: %v", err)
	}
}

func TestMetricObservationsProducedAndServedOverHTTP(t *testing.T) {
	handler, store, pool := newEconomicsTestRouter(t)
	cookie, csrf := loginAsEconomist(t, handler, store, "metrics-econ")

	created := doJSON(t, handler, http.MethodPost, "/api/v1/projects",
		map[string]string{"code": "MET-HTTP", "name": "Метрики через HTTP"}, cookie, csrf)
	if created.Code != http.StatusCreated {
		t.Fatalf("створення проєкту: %d %s", created.Code, created.Body.String())
	}
	var projectBody struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &projectBody); err != nil {
		t.Fatalf("розбір проєкту: %v", err)
	}
	projectID := projectBody.ID
	grantProjectRole(t, pool, store, "metrics-econ", "project.manager", projectID)

	if _, err := pool.Exec(context.Background(),
		`INSERT INTO core.project_phases
		   (project_id, phase_key, name, planned_start, planned_finish, status, config_generation)
		 VALUES ($1::uuid, 'design', 'Проєктування', DATE '2026-01-01', DATE '2026-01-31', 'active', 1)`,
		projectID); err != nil {
		t.Fatalf("створення фази: %v", err)
	}

	base := "/api/v1/projects/" + projectID + "/economics"

	// Доки кошторису немає, вимірювань теж немає — і це не нуль.
	empty := doJSON(t, handler, http.MethodGet,
		"/api/v1/projects/"+projectID+"/metrics/observations", nil, cookie, csrf)
	if empty.Code != http.StatusOK {
		t.Fatalf("читання вимірювань: %d %s", empty.Code, empty.Body.String())
	}
	var emptyBody struct {
		Observations []metrics.Observation `json:"observations"`
	}
	if err := json.Unmarshal(empty.Body.Bytes(), &emptyBody); err != nil {
		t.Fatalf("розбір вимірювань: %v", err)
	}
	if len(emptyBody.Observations) != 0 {
		t.Fatalf("до обчислень вимірювань має не бути, отримано %d", len(emptyBody.Observations))
	}

	baseline := doJSON(t, handler, http.MethodPost, base+"/cost-baselines", map[string]any{
		"name": "Базовий", "currency": "EUR",
		"lines": []map[string]string{{
			"phase_key": "design", "cost_category": "labor",
			"planned_amount": "10000.00", "funding_limit": "12000.00",
		}},
	}, cookie, csrf)
	if baseline.Code != http.StatusCreated {
		t.Fatalf("створення кошторису: %d %s", baseline.Code, baseline.Body.String())
	}
	var baselineBody struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(baseline.Body.Bytes(), &baselineBody); err != nil {
		t.Fatalf("розбір кошторису: %v", err)
	}

	approverCookie, approverCSRF := loginAsEconomist(t, handler, store, "metrics-approver")
	grantProjectRole(t, pool, store, "metrics-approver", "project.financial_controller", projectID)
	approve := doJSON(t, handler, http.MethodPost,
		base+"/cost-baselines/"+baselineBody.ID+"/approve", nil, approverCookie, approverCSRF)
	if approve.Code != http.StatusOK {
		t.Fatalf("затвердження кошторису: %d %s", approve.Code, approve.Body.String())
	}

	expense := doJSON(t, handler, http.MethodPost, base+"/expenses", map[string]string{
		"phase_key": "design", "cost_category": "hardware_prototypes", "expense_type": "capex",
		"amount": "4000.00", "currency": "EUR", "expense_date": "2026-01-10",
	}, cookie, csrf)
	if expense.Code != http.StatusCreated {
		t.Fatalf("запис витрати: %d %s", expense.Code, expense.Body.String())
	}

	// Записи економіки породили події; тепер їх обробляє фоновий збирач.
	drainAutomation(t, pool)

	got := doJSON(t, handler, http.MethodGet,
		"/api/v1/projects/"+projectID+"/metrics/observations", nil, cookie, csrf)
	if got.Code != http.StatusOK {
		t.Fatalf("читання вимірювань: %d %s", got.Code, got.Body.String())
	}
	var body struct {
		Observations []metrics.Observation `json:"observations"`
	}
	if err := json.Unmarshal(got.Body.Bytes(), &body); err != nil {
		t.Fatalf("розбір вимірювань: %v", err)
	}

	byKey := make(map[string]metrics.Observation, len(body.Observations))
	for _, obs := range body.Observations {
		byKey[obs.MetricKey] = obs
	}

	ac, ok := byKey["economics.ac"]
	if !ok {
		t.Fatalf("фактичну вартість не виміряно; отримано %d вимірювань", len(body.Observations))
	}
	if ac.Quality != metrics.QualityValid {
		t.Errorf("якість AC = %q, очікувано valid", ac.Quality)
	}
	if ac.Value == nil || *ac.Value != "4000.000000" {
		t.Errorf("AC = %v, очікувано 4000.000000", ac.Value)
	}
	if ac.PhaseKey != "design" {
		t.Errorf("AC віднесено до фази %q, очікувано design", ac.PhaseKey)
	}

	// Жодного результату не затверджено, тож здобута цінність нульова.
	// CPI при цьому дорівнює саме нулю — це справжнє вимірювання («витрачено
	// 4000, здобуто нічого»), а не брак даних. Розрізнення двох станів і є
	// предметом METRICS.md §6, тому перевіряємо його явно.
	cpi, ok := byKey["economics.cpi"]
	if !ok {
		t.Fatal("CPI відсутній серед вимірювань")
	}
	if cpi.Quality != metrics.QualityValid {
		t.Errorf("якість CPI = %q, очікувано valid", cpi.Quality)
	}
	if cpi.Value == nil || *cpi.Value != "0.000000" {
		t.Errorf("CPI = %v, очікувано 0.000000", cpi.Value)
	}

	// Дзеркальний випадок: SPI має нульовий знаменник лише за відсутності
	// планової вартості, тут вона є, тож показник теж визначений.
	spi, ok := byKey["economics.spi"]
	if !ok {
		t.Fatal("SPI відсутній серед вимірювань")
	}
	if spi.Quality != metrics.QualityValid {
		t.Errorf("якість SPI = %q, очікувано valid", spi.Quality)
	}
}
