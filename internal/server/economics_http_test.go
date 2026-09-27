package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"

	"github.com/jackc/pgx/v5/pgxpool"

	"delmos/internal/auth"
	"delmos/internal/economics"
	"delmos/internal/metrics"
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
		Metrics:      metrics.New(pool),
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

	work := doJSON(t, handler, http.MethodPost, base+"/work-records", map[string]string{
		"phase_key": "design", "role_key": "engineer", "work_date": "2026-01-10",
		"duration_hours": "8.00", "work_category": "design",
	}, cookie, csrf)
	if work.Code != http.StatusCreated {
		t.Fatalf("облік роботи без ставки: %d %s", work.Code, work.Body.String())
	}
	evm = doJSON(t, handler, http.MethodGet, base+"/earned-value", nil, cookie, "")
	if evm.Code != http.StatusUnprocessableEntity || !strings.Contains(evm.Body.String(), "incomplete_cost_data") {
		t.Fatalf("неповний AC має давати 422, отримано %d %s", evm.Code, evm.Body.String())
	}
	closure = doJSON(t, handler, http.MethodPost,
		"/api/v1/projects/"+projectID+"/phases/design/transition",
		map[string]string{"target_status": "completed"}, cookie, csrf)
	if closure.Code != http.StatusUnprocessableEntity || !strings.Contains(closure.Body.String(), "неможливо достовірно") {
		t.Fatalf("шлюз має відхилити неповний AC: %d %s", closure.Code, closure.Body.String())
	}
}

func TestMilestoneDecisionThroughHTTP(t *testing.T) {
	handler, store, pool := newEconomicsTestRouter(t)
	owner, ownerCSRF := loginAsEconomist(t, handler, store, "gate-owner")
	created := doJSON(t, handler, http.MethodPost, "/api/v1/projects",
		map[string]string{"code": "GATE-HTTP", "name": "Phase gate"}, owner, ownerCSRF)
	if created.Code != http.StatusCreated {
		t.Fatalf("створення проєкту: %d %s", created.Code, created.Body.String())
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	grantProjectRole(t, pool, store, "gate-owner", "project.manager", body.ID)
	var revisionID uuid.UUID
	if err := pool.QueryRow(context.Background(), `SELECT r.id FROM core.project_plan_bindings b
		JOIN core.work_product_revisions r ON r.work_product_id = b.plan_work_product_id
		WHERE b.project_id = $1::uuid`, body.ID).Scan(&revisionID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `UPDATE core.project_plan_bindings
		SET effective_plan_revision_id = $2, config_generation = 1 WHERE project_id = $1::uuid`, body.ID, revisionID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `UPDATE core.project_plan_manifests
		SET manifest = jsonb_set(manifest, '{milestones}', $2::jsonb)
		WHERE revision_id = $1`, revisionID,
		`[{"key":"MS-1","name":"Exit review","phase_key":"design","target_date":"2026-01-30","deliverable_keys":[],"acceptance_rule_keys":[]}]`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO core.project_phases
		(project_id, phase_key, name, status, config_generation)
		VALUES ($1::uuid, 'design', 'Design', 'active', 1)`, body.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO core.project_milestones
		(project_id, milestone_key, phase_key, name, status, config_generation)
		VALUES ($1::uuid, 'MS-1', 'design', 'Exit review', 'pending', 1)`, body.ID); err != nil {
		t.Fatal(err)
	}
	phaseURL := "/api/v1/projects/" + body.ID + "/phases/design/transition"
	if blocked := doJSON(t, handler, http.MethodPost, phaseURL,
		map[string]string{"target_status": "completed"}, owner, ownerCSRF); blocked.Code != http.StatusUnprocessableEntity {
		t.Fatalf("без gate decision очікувано 422: %d %s", blocked.Code, blocked.Body.String())
	}
	acceptURL := "/api/v1/projects/" + body.ID + "/milestones/MS-1/accept"
	if self := doJSON(t, handler, http.MethodPost, acceptURL, nil, owner, ownerCSRF); self.Code == http.StatusOK {
		t.Fatalf("автор плану не має самостійно приймати віху: %s", self.Body.String())
	}
	approver, approverCSRF := loginAsEconomist(t, handler, store, "gate-approver")
	grantProjectRole(t, pool, store, "gate-approver", "project.approver", body.ID)
	if _, err := pool.Exec(context.Background(), `INSERT INTO core.rule_definitions
		(rule_key, trigger_key, enforcement_level, assert_condition)
		VALUES ('rule.test.http_gate_veto', 'trigger.core.before_milestone_accept', 'MANDATORY_VETO',
		'{"predicate_key":"always_false"}')`); err != nil {
		t.Fatal(err)
	}
	vetoed := doJSON(t, handler, http.MethodPost, acceptURL, nil, approver, approverCSRF)
	if vetoed.Code != http.StatusUnprocessableEntity {
		t.Fatalf("mandatory milestone veto має повертати 422: %d %s", vetoed.Code, vetoed.Body.String())
	}
	if _, err := pool.Exec(context.Background(),
		`UPDATE core.rule_definitions SET active = false WHERE rule_key = 'rule.test.http_gate_veto'`); err != nil {
		t.Fatal(err)
	}
	accepted := doJSON(t, handler, http.MethodPost, acceptURL, nil, approver, approverCSRF)
	if accepted.Code != http.StatusOK {
		t.Fatalf("приймання: %d %s", accepted.Code, accepted.Body.String())
	}
	repeated := doJSON(t, handler, http.MethodPost, acceptURL, nil, approver, approverCSRF)
	if repeated.Code != http.StatusOK || repeated.Body.String() != accepted.Body.String() {
		t.Fatalf("повторне приймання має бути ідемпотентним: %d %s", repeated.Code, repeated.Body.String())
	}
	closed := doJSON(t, handler, http.MethodPost, phaseURL,
		map[string]string{"target_status": "completed"}, owner, ownerCSRF)
	if closed.Code != http.StatusOK {
		t.Fatalf("завершення після рішення: %d %s", closed.Code, closed.Body.String())
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
func TestWPTransitionRulesThroughHTTP(t *testing.T) {
	handler, store, pool := newEconomicsTestRouter(t)
	ownerCookie, ownerCSRF := loginAsEconomist(t, handler, store, "workflow-owner")
	created := doJSON(t, handler, http.MethodPost, "/api/v1/projects",
		map[string]string{"code": "WP-RULE-HTTP", "name": "Workflow rules"}, ownerCookie, ownerCSRF)
	if created.Code != http.StatusCreated {
		t.Fatalf("створення проєкту: %d %s", created.Code, created.Body.String())
	}
	var projectBody struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &projectBody); err != nil {
		t.Fatal(err)
	}
	grantProjectRole(t, pool, store, "workflow-owner", "project.owner", projectBody.ID)
	owner, err := store.FindActiveUserByLogin(context.Background(), "workflow-owner")
	if err != nil {
		t.Fatal(err)
	}
	var workProductID uuid.UUID
	if err := pool.QueryRow(context.Background(), `INSERT INTO core.work_products
		(project_id, code, type, profile, title) VALUES ($1, 'REQ-WF', 'requirement', 'core:requirement', 'Workflow') RETURNING id`,
		projectBody.ID).Scan(&workProductID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO core.work_product_revisions
		(work_product_id, revision_number, body, metadata, payload_hash, content_hash, created_by)
		VALUES ($1, 1, 'text', '{}'::jsonb, sha256('text'::bytea), sha256('text'::bytea), $2)`,
		workProductID, owner.ID); err != nil {
		t.Fatal(err)
	}
	reviewerCookie, reviewerCSRF := loginAsEconomist(t, handler, store, "workflow-reviewer")
	approverCookie, approverCSRF := loginAsEconomist(t, handler, store, "workflow-approver")
	grantProjectRole(t, pool, store, "workflow-reviewer", "project.reviewer", projectBody.ID)
	grantProjectRole(t, pool, store, "workflow-approver", "project.approver", projectBody.ID)
	reviewer, err := store.FindActiveUserByLogin(context.Background(), "workflow-reviewer")
	if err != nil {
		t.Fatal(err)
	}
	approver, err := store.FindActiveUserByLogin(context.Background(), "workflow-approver")
	if err != nil {
		t.Fatal(err)
	}
	base := "/api/v1/projects/" + projectBody.ID + "/work-products/" + workProductID.String()
	assignments := map[string]any{"assignments": []map[string]string{
		{"user_id": reviewer.ID.String(), "assignment_role": "reviewer"},
		{"user_id": approver.ID.String(), "assignment_role": "approver"},
	}}
	if _, err := pool.Exec(context.Background(), `INSERT INTO core.rule_definitions
		(rule_key, trigger_key, enforcement_level, when_condition, assert_condition)
		VALUES ('rule.test.http_submit', 'trigger.core.before_wp_transition', 'MANDATORY_VETO',
		'{"predicate_key":"field_equals","params":{"field":"target_status","value":"in_review"}}',
		'{"predicate_key":"always_false"}')`); err != nil {
		t.Fatal(err)
	}
	if veto := doJSON(t, handler, http.MethodPost, base+"/submit", assignments, ownerCookie, ownerCSRF); veto.Code != http.StatusUnprocessableEntity {
		t.Fatalf("submit veto має дати 422, отримано %d: %s", veto.Code, veto.Body.String())
	}
	if _, err := pool.Exec(context.Background(), `UPDATE core.rule_definitions SET active = false WHERE rule_key = 'rule.test.http_submit'`); err != nil {
		t.Fatal(err)
	}
	if submit := doJSON(t, handler, http.MethodPost, base+"/submit", assignments, ownerCookie, ownerCSRF); submit.Code != http.StatusCreated {
		t.Fatalf("подання без вето: %d %s", submit.Code, submit.Body.String())
	}
	if review := doJSON(t, handler, http.MethodPost, base+"/reviews", nil, reviewerCookie, reviewerCSRF); review.Code != http.StatusCreated {
		t.Fatalf("рецензія: %d %s", review.Code, review.Body.String())
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO core.rule_definitions
		(rule_key, trigger_key, enforcement_level, when_condition, assert_condition)
		VALUES ('rule.test.http_approve', 'trigger.core.before_wp_transition', 'MANDATORY_VETO',
		'{"predicate_key":"field_equals","params":{"field":"target_status","value":"approved"}}',
		'{"predicate_key":"always_false"}')`); err != nil {
		t.Fatal(err)
	}
	if veto := doJSON(t, handler, http.MethodPost, base+"/approvals", nil, approverCookie, approverCSRF); veto.Code != http.StatusUnprocessableEntity {
		t.Fatalf("approval veto має дати 422, отримано %d: %s", veto.Code, veto.Body.String())
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO core.rule_definitions
		(rule_key, trigger_key, enforcement_level, when_condition, assert_condition)
		VALUES ('rule.test.http_retire', 'trigger.core.before_wp_transition', 'MANDATORY_VETO',
		'{"predicate_key":"field_equals","params":{"field":"action","value":"retire"}}',
		'{"predicate_key":"always_false"}')`); err != nil {
		t.Fatal(err)
	}
	retire := doJSON(t, handler, http.MethodPost, base+"/retire",
		map[string]int{"expected_row_version": 2}, ownerCookie, ownerCSRF)
	if retire.Code != http.StatusUnprocessableEntity {
		t.Fatalf("retire veto має дати 422, отримано %d: %s", retire.Code, retire.Body.String())
	}
	var planID uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`SELECT plan_work_product_id FROM core.project_plan_bindings WHERE project_id = $1`, projectBody.ID).Scan(&planID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO core.rule_definitions
		(rule_key, trigger_key, enforcement_level, assert_condition)
		VALUES ('rule.test.http_trace', 'trigger.core.before_trace_create', 'MANDATORY_VETO',
		'{"predicate_key":"always_false"}')`); err != nil {
		t.Fatal(err)
	}
	trace := doJSON(t, handler, http.MethodPost, "/api/v1/projects/"+projectBody.ID+"/trace-links",
		map[string]string{"source_id": workProductID.String(), "target_id": planID.String(), "relation_kind": "verifies"},
		ownerCookie, ownerCSRF)
	if trace.Code != http.StatusUnprocessableEntity {
		t.Fatalf("trace veto має дати 422, отримано %d: %s", trace.Code, trace.Body.String())
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
