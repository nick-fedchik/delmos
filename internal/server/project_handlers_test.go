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
	"delmos/internal/migrate"
	"delmos/internal/project"
	"delmos/internal/ratelimit"
	"delmos/internal/testsupport"
)

// newProjectTestRouter готує роутер із bootstrap-адміністратором, якому надано
// project.manager (scopes.manage), і повертає готовий до логіну обліковий запис.
func newProjectTestRouter(t *testing.T) (http.Handler, *auth.Store) {
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
		LoginLimiter: ratelimit.New(rate.Every(time.Millisecond), 1000),
		CookieSecure: true,
	}

	return newRouter(testLogger(), deps), authStore
}

// loginAsProjectManager створює адміністратора, надає йому project.manager і повертає
// cookie та CSRF-токен чинної сесії, готової створювати проєкти.
func loginAsProjectManager(t *testing.T, handler http.Handler, store *auth.Store) (*http.Cookie, string) {
	t.Helper()

	bootstrapTestAdmin(t, store, "pm", "Project-Manager-Pass-1")

	svc := auth.NewService(store)
	adminUser, err := store.FindActiveUserByLogin(context.Background(), "pm")
	if err != nil {
		t.Fatalf("пошук bootstrap-користувача: %v", err)
	}
	if _, err := svc.GrantSystemRole(context.Background(), adminUser.ID, adminUser.ID, "project.manager", "test setup"); err != nil {
		t.Fatalf("надання project.manager: %v", err)
	}

	login := doJSON(t, handler, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"login": "pm", "password": "Project-Manager-Pass-1"}, nil, "")
	if login.Code != http.StatusOK {
		t.Fatalf("очікувався 200 при вході, отримано %d: %s", login.Code, login.Body.String())
	}

	var body loginResponse
	if err := json.Unmarshal(login.Body.Bytes(), &body); err != nil {
		t.Fatalf("розбір тіла відповіді входу: %v", err)
	}

	return sessionCookieFromResponse(t, login), body.CSRFToken
}

func TestCreateProjectCreatesPlanAtomically(t *testing.T) {
	handler, store := newProjectTestRouter(t)
	cookie, csrfToken := loginAsProjectManager(t, handler, store)

	create := doJSON(t, handler, http.MethodPost, "/api/v1/projects",
		map[string]string{"code": "INV-001", "name": "Inverter Control", "description": "HW+SW"}, cookie, csrfToken)
	if create.Code != http.StatusCreated {
		t.Fatalf("очікувався 201, отримано %d: %s", create.Code, create.Body.String())
	}

	var view projectView
	if err := json.Unmarshal(create.Body.Bytes(), &view); err != nil {
		t.Fatalf("розбір тіла відповіді створення проєкту: %v", err)
	}
	if view.Code != "INV-001" || view.Plan.Code != "PLAN-001" || view.Plan.RevisionNumber != 1 {
		t.Errorf("очікувався проєкт з PLAN-001/r1, отримано %+v", view)
	}
	if view.Plan.PayloadHash == "" {
		t.Error("PLAN-001 має мати обчислений payload_hash")
	}

	get := doJSON(t, handler, http.MethodGet, "/api/v1/projects/"+view.ID, nil, cookie, "")
	if get.Code != http.StatusOK {
		t.Fatalf("власник щойно створеного проєкту має мати доступ на читання, отримано %d", get.Code)
	}
}

func TestPlanEndpointCreatesStructuredRevision(t *testing.T) {
	handler, store := newProjectTestRouter(t)
	cookie, csrfToken := loginAsProjectManager(t, handler, store)
	createdProject := createTestProject(t, handler, cookie, csrfToken, "PLAN-H-001")

	get := doJSON(t, handler, http.MethodGet, "/api/v1/projects/"+createdProject.ID+"/plan", nil, cookie, "")
	if get.Code != http.StatusOK {
		t.Fatalf("очікувався 200 для плану, отримано %d: %s", get.Code, get.Body.String())
	}
	var plan planDetailView
	if err := json.Unmarshal(get.Body.Bytes(), &plan); err != nil {
		t.Fatalf("розбір плану: %v", err)
	}
	if plan.TemplateKey != "generic-project-plan" || plan.TemplateVersion != 1 || plan.RevisionNumber != 1 {
		t.Fatalf("очікувався generic-project-plan@1/r1, отримано %+v", plan)
	}

	plan.Manifest.Phases = []project.PlanPhase{{Key: "PH-001", Name: "Delivery", PlannedStart: "2026-10-01", PlannedFinish: "2026-10-31"}}
	revise := doJSON(t, handler, http.MethodPost, "/api/v1/projects/"+createdProject.ID+"/plan/revisions",
		map[string]any{"expected_row_version": plan.RowVersion, "body": "# Plan\n", "manifest": plan.Manifest}, cookie, csrfToken)
	if revise.Code != http.StatusCreated {
		t.Fatalf("очікувався 201 для ревізії плану, отримано %d: %s", revise.Code, revise.Body.String())
	}
	var revised planDetailView
	if err := json.Unmarshal(revise.Body.Bytes(), &revised); err != nil {
		t.Fatalf("розбір ревізії плану: %v", err)
	}
	if revised.RevisionNumber != 2 || revised.RowVersion != 2 {
		t.Fatalf("очікувалися r2 та row_version=2, отримано %+v", revised)
	}
}

func TestCreateProjectRejectsDuplicateCode(t *testing.T) {
	handler, store := newProjectTestRouter(t)
	cookie, csrfToken := loginAsProjectManager(t, handler, store)

	body := map[string]string{"code": "DUP-001", "name": "First", "description": ""}
	first := doJSON(t, handler, http.MethodPost, "/api/v1/projects", body, cookie, csrfToken)
	if first.Code != http.StatusCreated {
		t.Fatalf("перше створення має пройти, отримано %d: %s", first.Code, first.Body.String())
	}

	second := doJSON(t, handler, http.MethodPost, "/api/v1/projects", body, cookie, csrfToken)
	if second.Code != http.StatusConflict {
		t.Errorf("дубльований код проєкту має повертати 409, отримано %d", second.Code)
	}
}

func TestCreateProjectRequiresScopesManage(t *testing.T) {
	handler, store := newProjectTestRouter(t)

	bootstrapTestAdmin(t, store, "plain-admin", "Plain-Admin-Pass-1")
	login := doJSON(t, handler, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"login": "plain-admin", "password": "Plain-Admin-Pass-1"}, nil, "")
	var body loginResponse
	_ = json.Unmarshal(login.Body.Bytes(), &body)
	cookie := sessionCookieFromResponse(t, login)

	create := doJSON(t, handler, http.MethodPost, "/api/v1/projects",
		map[string]string{"code": "NOPE-001", "name": "Nope"}, cookie, body.CSRFToken)
	if create.Code != http.StatusForbidden {
		t.Errorf("system.administrator без scopes.manage не повинен створювати проєкт, отримано %d", create.Code)
	}
}

func TestGetProjectHiddenFromNonMember(t *testing.T) {
	handler, store := newProjectTestRouter(t)
	ownerCookie, ownerCSRF := loginAsProjectManager(t, handler, store)

	created := doJSON(t, handler, http.MethodPost, "/api/v1/projects",
		map[string]string{"code": "PRIV-001", "name": "Private"}, ownerCookie, ownerCSRF)
	var view projectView
	_ = json.Unmarshal(created.Body.Bytes(), &view)

	bootstrapTestAdmin(t, store, "outsider", "Outsider-Pass-1")
	outsiderLogin := doJSON(t, handler, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"login": "outsider", "password": "Outsider-Pass-1"}, nil, "")
	outsiderCookie := sessionCookieFromResponse(t, outsiderLogin)

	get := doJSON(t, handler, http.MethodGet, "/api/v1/projects/"+view.ID, nil, outsiderCookie, "")
	if get.Code != http.StatusNotFound {
		t.Errorf("сторонній користувач не повинен бачити чужий проєкт (SWR-42 §3), отримано %d", get.Code)
	}
}

func TestListProjectsOnlyReturnsMemberProjects(t *testing.T) {
	handler, store := newProjectTestRouter(t)
	ownerCookie, ownerCSRF := loginAsProjectManager(t, handler, store)

	doJSON(t, handler, http.MethodPost, "/api/v1/projects",
		map[string]string{"code": "LIST-001", "name": "Listed"}, ownerCookie, ownerCSRF)

	list := doJSON(t, handler, http.MethodGet, "/api/v1/projects", nil, ownerCookie, "")
	if list.Code != http.StatusOK {
		t.Fatalf("очікувався 200, отримано %d", list.Code)
	}

	var summaries []projectSummaryView
	if err := json.Unmarshal(list.Body.Bytes(), &summaries); err != nil {
		t.Fatalf("розбір списку проєктів: %v", err)
	}
	if len(summaries) != 1 || summaries[0].Code != "LIST-001" {
		t.Errorf("очікувався рівно один проєкт LIST-001, отримано %+v", summaries)
	}
}
