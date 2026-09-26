package server

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/jackc/pgx/v5/pgxpool"

	"delmos/internal/auth"
	"delmos/internal/economics"
	"delmos/internal/migrate"
	"delmos/internal/project"
	"delmos/internal/ratelimit"
	"delmos/internal/testsupport"
)

// Ревізія коду показала корінь проблеми: тести зверталися до сховища напряму,
// тож відсутність HTTP-шару економіки лишалася непоміченою — шлюз ліміту
// фінансування «працював» лише в тестах. Цей тест іде тим самим шляхом, що й
// користувач: логін, CSRF, реальні маршрути.

func newEconomicsTestRouter(t *testing.T) (http.Handler, *auth.Store, *pgxpool.Pool) {
	t.Helper()

	pool, err := pgxpool.New(context.Background(), testsupport.NewDatabase(t))
	if err != nil {
		t.Fatalf("підключення до тимчасової бази: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := migrate.Apply(context.Background(), pool, testLogger()); err != nil {
		t.Fatalf("застосування міграцій: %v", err)
	}

	authStore := auth.NewStore(pool)
	deps := Deps{
		Ready:        func(context.Context) error { return nil },
		Auth:         auth.NewService(authStore),
		Projects:     project.NewStore(pool),
		Economics:    economics.New(pool),
		LoginLimiter: ratelimit.New(rate.Every(time.Millisecond), 1000),
		CookieSecure: true,
	}
	return newRouter(testLogger(), deps), authStore, pool
}

// Права економіки не входять у project.manager повністю: economics.approve
// навмисно віддане окремій ролі. Тут надаємо обидві, щоб перевірити саме
// маршрути, а розподіл обовʼязків перевіряється окремим тестом нижче.
func loginAsEconomist(t *testing.T, handler http.Handler, store *auth.Store, login string) (*http.Cookie, string) {
	t.Helper()
	bootstrapTestAdmin(t, store, login, "Economics-Pass-12345")

	svc := auth.NewService(store)
	user, err := store.FindActiveUserByLogin(context.Background(), login)
	if err != nil {
		t.Fatalf("пошук користувача %s: %v", login, err)
	}
	for _, role := range []string{"project.manager", "project.owner", "project.financial_controller"} {
		if _, err := svc.GrantSystemRole(context.Background(), user.ID, user.ID, role, "test setup"); err != nil {
			t.Fatalf("надання %s: %v", role, err)
		}
	}

	resp := doJSON(t, handler, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"login": login, "password": "Economics-Pass-12345"}, nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("вхід %s: %d %s", login, resp.Code, resp.Body.String())
	}
	var body loginResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("розбір відповіді входу: %v", err)
	}
	return sessionCookieFromResponse(t, resp), body.CSRFToken
}

// grantProjectRole надає роль у межах конкретного проєкту. Публічного API для
// цього ще немає (є лише GrantSystemRole), тому прив'язка створюється прямо —
// це підготовка оточення, а не предмет перевірки.
func grantProjectRole(t *testing.T, pool *pgxpool.Pool, store *auth.Store, login, roleKey, projectID string) {
	t.Helper()
	user, err := store.FindActiveUserByLogin(context.Background(), login)
	if err != nil {
		t.Fatalf("пошук користувача %s: %v", login, err)
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO core.role_bindings (user_id, role_key, scope_type, scope_id, granted_by, granted_reason)
		 VALUES ($1, $2, 'project', $3::uuid, $1, 'test setup')`,
		user.ID, roleKey, projectID); err != nil {
		t.Fatalf("прив'язка ролі %s у проєкті: %v", roleKey, err)
	}
}

// Наскрізний шлях: проєкт -> фаза -> кошторис -> затвердження -> витрата ->
// спроба закрити фазу. Усе виключно через HTTP.
func TestFundingLimitGateThroughHTTP(t *testing.T) {
	handler, store, pool := newEconomicsTestRouter(t)
	cookie, csrf := loginAsEconomist(t, handler, store, "econ")

	created := doJSON(t, handler, http.MethodPost, "/api/v1/projects",
		map[string]string{"code": "ECO-HTTP", "name": "Економіка через HTTP"}, cookie, csrf)
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
	if projectID == "" {
		t.Fatalf("порожній ідентифікатор проєкту: %s", created.Body.String())
	}

	grantProjectRole(t, pool, store, "econ", "project.manager", projectID)

	// Фаза створюється напряму: plan.apply потребує погодження плану, що є
	// окремим сценарієм і тут лише зашумило б перевірку шлюзу.
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO core.project_phases
		   (project_id, phase_key, name, planned_start, planned_finish, status, config_generation)
		 VALUES ($1::uuid, 'design', 'Проєктування', DATE '2026-01-01', DATE '2026-01-31', 'active', 1)`,
		projectID); err != nil {
		t.Fatalf("створення фази: %v", err)
	}

	base := "/api/v1/projects/" + projectID + "/economics"

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

	// Затверджує інша особа: складач кошторису не має на це права.
	approverCookie, approverCSRF := loginAsEconomist(t, handler, store, "econ-approver")
	grantProjectRole(t, pool, store, "econ-approver", "project.financial_controller", projectID)
	approve := doJSON(t, handler, http.MethodPost,
		base+"/cost-baselines/"+baselineBody.ID+"/approve", nil, approverCookie, approverCSRF)
	if approve.Code != http.StatusOK {
		t.Fatalf("затвердження кошторису: %d %s", approve.Code, approve.Body.String())
	}

	// Витрата перевищує ліміт 12000.
	expense := doJSON(t, handler, http.MethodPost, base+"/expenses", map[string]string{
		"phase_key": "design", "cost_category": "hardware_prototypes", "expense_type": "capex",
		"amount": "13000.00", "currency": "EUR", "expense_date": "2026-01-10",
	}, cookie, csrf)
	if expense.Code != http.StatusCreated {
		t.Fatalf("запис витрати: %d %s", expense.Code, expense.Body.String())
	}

	// Головна перевірка: шлюз блокує закриття фази через реальний маршрут.
	closure := doJSON(t, handler, http.MethodPost,
		"/api/v1/projects/"+projectID+"/phases/design/transition",
		map[string]string{"target_status": "completed"}, cookie, csrf)
	if closure.Code != http.StatusUnprocessableEntity {
		t.Fatalf("очікувався 422 від шлюзу ліміту, отримано %d: %s", closure.Code, closure.Body.String())
	}

	var phaseStatus string
	if err := pool.QueryRow(context.Background(),
		`SELECT status FROM core.project_phases WHERE project_id = $1::uuid AND phase_key = 'design'`,
		projectID).Scan(&phaseStatus); err != nil {
		t.Fatalf("читання статусу фази: %v", err)
	}
	if phaseStatus != "active" {
		t.Errorf("статус фази = %q, очікувано active", phaseStatus)
	}

	// Показники доступні для читання тим самим шляхом.
	evm := doJSON(t, handler, http.MethodGet, base+"/earned-value", nil, cookie, "")
	if evm.Code != http.StatusOK {
		t.Fatalf("читання показників: %d %s", evm.Code, evm.Body.String())
	}
	var snapshot struct {
		ActualCost string `json:"ac"`
	}
	if err := json.Unmarshal(evm.Body.Bytes(), &snapshot); err != nil {
		t.Fatalf("розбір показників: %v", err)
	}
	if snapshot.ActualCost != "13000.00" {
		t.Errorf("AC = %s, очікувано 13000.00", snapshot.ActualCost)
	}
}

// Розподіл обовʼязків діє й через HTTP: складач кошторису не затверджує його.
func TestCostBaselineSelfApprovalRejectedThroughHTTP(t *testing.T) {
	handler, store, pool := newEconomicsTestRouter(t)
	cookie, csrf := loginAsEconomist(t, handler, store, "econ")

	created := doJSON(t, handler, http.MethodPost, "/api/v1/projects",
		map[string]string{"code": "ECO-SOD", "name": "Розподіл обовʼязків"}, cookie, csrf)
	if created.Code != http.StatusCreated {
		t.Fatalf("створення проєкту: %d %s", created.Code, created.Body.String())
	}
	var projectBody struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &projectBody); err != nil {
		t.Fatalf("розбір проєкту: %v", err)
	}
	grantProjectRole(t, pool, store, "econ", "project.manager", projectBody.ID)
	grantProjectRole(t, pool, store, "econ", "project.financial_controller", projectBody.ID)
	base := "/api/v1/projects/" + projectBody.ID + "/economics"

	baseline := doJSON(t, handler, http.MethodPost, base+"/cost-baselines", map[string]any{
		"name": "Базовий", "currency": "EUR", "lines": []map[string]string{},
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

	self := doJSON(t, handler, http.MethodPost,
		base+"/cost-baselines/"+baselineBody.ID+"/approve", nil, cookie, csrf)
	if self.Code != http.StatusUnprocessableEntity {
		t.Fatalf("очікувався 422 на самозатвердження, отримано %d: %s", self.Code, self.Body.String())
	}
}

// Без права economics.manage запис у економіку недоступний.
func TestEconomicsWriteRequiresPermission(t *testing.T) {
	handler, store, _ := newEconomicsTestRouter(t)
	owner, ownerCSRF := loginAsEconomist(t, handler, store, "econ")

	created := doJSON(t, handler, http.MethodPost, "/api/v1/projects",
		map[string]string{"code": "ECO-PERM", "name": "Права"}, owner, ownerCSRF)
	if created.Code != http.StatusCreated {
		t.Fatalf("створення проєкту: %d %s", created.Code, created.Body.String())
	}
	var projectBody struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &projectBody); err != nil {
		t.Fatalf("розбір проєкту: %v", err)
	}

	// Сторонній користувач без ролей у проєкті.
	bootstrapTestAdmin(t, store, "outsider", "Outsider-Pass-12345")
	resp := doJSON(t, handler, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"login": "outsider", "password": "Outsider-Pass-12345"}, nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("вхід стороннього: %d", resp.Code)
	}
	var body loginResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("розбір відповіді входу: %v", err)
	}
	outsider := sessionCookieFromResponse(t, resp)

	expense := doJSON(t, handler, http.MethodPost,
		"/api/v1/projects/"+projectBody.ID+"/economics/expenses",
		map[string]string{
			"phase_key": "design", "cost_category": "labor", "expense_type": "opex",
			"amount": "1.00", "currency": "EUR", "expense_date": "2026-01-10",
		}, outsider, body.CSRFToken)
	if expense.Code != http.StatusForbidden && expense.Code != http.StatusNotFound {
		t.Errorf("очікувалась відмова доступу, отримано %d: %s", expense.Code, expense.Body.String())
	}
}
