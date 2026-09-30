package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/google/uuid"
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

func TestPlanReviewCandidatesOnlyIncludeIndependentProjectReviewers(t *testing.T) {
	handler, store := newProjectTestRouter(t)
	cookie, csrf := loginAsProjectManager(t, handler, store)
	created := createTestProject(t, handler, cookie, csrf, "PLAN-REVIEW-CANDIDATES")
	pool := store.Pool()
	bootstrapTestAdmin(t, store, "plan-reviewer", "Review-Pass-12345")
	bootstrapTestAdmin(t, store, "plan-approver", "Approve-Pass-12345")
	grantProjectRole(t, pool, store, "plan-reviewer", "project.reviewer", created.ID)
	grantProjectRole(t, pool, store, "plan-approver", "project.approver", created.ID)
	grantProjectRole(t, pool, store, "pm", "project.reviewer", created.ID)

	path := "/api/v1/projects/" + created.ID + "/plan/review-candidates"
	response := doJSON(t, handler, http.MethodGet, path, nil, cookie, "")
	if response.Code != http.StatusOK {
		t.Fatalf("очікувався список кандидатів, отримано %d: %s", response.Code, response.Body.String())
	}
	var candidates []auth.ReviewCandidate
	if err := json.Unmarshal(response.Body.Bytes(), &candidates); err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 {
		t.Fatalf("мають бути лише незалежні кандидати цього проєкту: %+v", candidates)
	}
	for _, candidate := range candidates {
		switch candidate.Login {
		case "plan-reviewer":
			if !candidate.CanReview || candidate.CanApprove {
				t.Errorf("невірні дозволи рецензента: %+v", candidate)
			}
		case "plan-approver":
			if !candidate.CanApprove || candidate.CanReview {
				t.Errorf("невірні дозволи погоджувача: %+v", candidate)
			}
		default:
			t.Errorf("зайва особа у списку: %+v", candidate)
		}
	}
	planResponse := doJSON(t, handler, http.MethodGet, "/api/v1/projects/"+created.ID+"/plan", nil, cookie, "")
	var plan planDetailView
	if err := json.Unmarshal(planResponse.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	for _, candidate := range candidates {
		if candidate.Login == "plan-reviewer" {
			incomplete := doJSON(t, handler, http.MethodPost,
				"/api/v1/projects/"+created.ID+"/work-products/"+plan.WorkProductID+"/submit",
				map[string]any{"assignments": []map[string]string{{"user_id": candidate.ID.String(), "assignment_role": "reviewer"}}}, cookie, csrf)
			if incomplete.Code != http.StatusUnprocessableEntity || !strings.Contains(incomplete.Body.String(), "invalid_assignments") {
				t.Fatalf("неповне призначення має повернути 422 invalid_assignments: %d %s", incomplete.Code, incomplete.Body.String())
			}
			break
		}
	}
	if current := doJSON(t, handler, http.MethodGet, "/api/v1/projects/"+created.ID+"/plan", nil, cookie, ""); !strings.Contains(current.Body.String(), `"status":"draft"`) {
		t.Fatalf("після відмови план має лишатися чернеткою: %s", current.Body.String())
	}

	login := doJSON(t, handler, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"login": "plan-reviewer", "password": "Review-Pass-12345"}, nil, "")
	if login.Code != http.StatusOK {
		t.Fatalf("вхід рецензента: %d", login.Code)
	}
	unauthorized := doJSON(t, handler, http.MethodGet, path, nil, sessionCookieFromResponse(t, login), "")
	if unauthorized.Code != http.StatusNotFound {
		t.Fatalf("без wp.submit список не має розкриватися: %d", unauthorized.Code)
	}
}

func TestAdminManagesProjectReviewRolesThroughHTTP(t *testing.T) {
	handler, store := newProjectTestRouter(t)
	adminCookie, csrf := loginAsProjectManager(t, handler, store)
	created := createTestProject(t, handler, adminCookie, csrf, "ROLE-GUI-TEST")
	bootstrapTestAdmin(t, store, "role-gui-reviewer", "Role-Review-Pass-1")
	reviewer, err := store.FindActiveUserByLogin(context.Background(), "role-gui-reviewer")
	if err != nil {
		t.Fatal(err)
	}
	projects := doJSON(t, handler, http.MethodGet, "/api/v1/admin/review-role-projects?query=ROLE-GUI", nil, adminCookie, "")
	if projects.Code != http.StatusOK || !strings.Contains(projects.Body.String(), created.ID) {
		t.Fatalf("пошук проєкту: %d %s", projects.Code, projects.Body.String())
	}
	users := doJSON(t, handler, http.MethodGet, "/api/v1/admin/review-role-users?query=role-gui", nil, adminCookie, "")
	if users.Code != http.StatusOK || !strings.Contains(users.Body.String(), reviewer.ID.String()) {
		t.Fatalf("пошук користувача: %d %s", users.Code, users.Body.String())
	}
	path := "/api/v1/projects/" + created.ID + "/review-role-bindings"
	request := map[string]string{"user_id": reviewer.ID.String(), "role_key": "project.reviewer", "reason": "Незалежна рецензія плану"}
	if noCsrf := doJSON(t, handler, http.MethodPost, path, request, adminCookie, ""); noCsrf.Code != http.StatusForbidden {
		t.Fatalf("видача без CSRF має бути заборонена: %d", noCsrf.Code)
	}
	createdBinding := doJSON(t, handler, http.MethodPost, path, request, adminCookie, csrf)
	if createdBinding.Code != http.StatusCreated {
		t.Fatalf("видача ролі: %d %s", createdBinding.Code, createdBinding.Body.String())
	}
	var binding roleBindingView
	if err := json.Unmarshal(createdBinding.Body.Bytes(), &binding); err != nil {
		t.Fatal(err)
	}
	if duplicate := doJSON(t, handler, http.MethodPost, path, request, adminCookie, csrf); duplicate.Code != http.StatusConflict {
		t.Fatalf("дубль має бути відхилений: %d", duplicate.Code)
	}
	admin, err := store.FindActiveUserByLogin(context.Background(), "pm")
	if err != nil {
		t.Fatal(err)
	}
	if self := doJSON(t, handler, http.MethodPost, path,
		map[string]string{"user_id": admin.ID.String(), "role_key": "project.approver", "reason": "самопризначення"}, adminCookie, csrf); self.Code != http.StatusUnprocessableEntity {
		t.Fatalf("самопризначення має бути відхилено: %d", self.Code)
	}
	list := doJSON(t, handler, http.MethodGet, path, nil, adminCookie, "")
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), reviewer.Login) {
		t.Fatalf("прив'язка не з'явилася: %d %s", list.Code, list.Body.String())
	}
	if wrongScope := doJSON(t, handler, http.MethodDelete,
		"/api/v1/projects/"+uuid.NewString()+"/review-role-bindings/"+binding.ID, nil, adminCookie, csrf); wrongScope.Code != http.StatusNotFound {
		t.Fatalf("відкликання за іншим проєктом має бути 404: %d", wrongScope.Code)
	}
	if revoke := doJSON(t, handler, http.MethodDelete, path+"/"+binding.ID, nil, adminCookie, csrf); revoke.Code != http.StatusNoContent {
		t.Fatalf("відкликання ролі: %d %s", revoke.Code, revoke.Body.String())
	}
	list = doJSON(t, handler, http.MethodGet, path, nil, adminCookie, "")
	if list.Code != http.StatusOK || strings.Contains(list.Body.String(), reviewer.Login) {
		t.Fatalf("відкликана роль не має залишитися у списку: %d %s", list.Code, list.Body.String())
	}
	unauthorized := doJSON(t, handler, http.MethodGet, "/api/v1/admin/review-role-projects", nil, nil, "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("список проєктів без сесії має бути закритим: %d", unauthorized.Code)
	}
}

func TestPlanReviewProgressFollowsIndependentDecisions(t *testing.T) {
	handler, store := newProjectTestRouter(t)
	cookie, csrf := loginAsProjectManager(t, handler, store)
	created := createTestProject(t, handler, cookie, csrf, "PLAN-REVIEW-PROGRESS")
	pool := store.Pool()
	bootstrapTestAdmin(t, store, "progress-reviewer", "Review-Pass-12345")
	bootstrapTestAdmin(t, store, "progress-approver", "Approve-Pass-12345")
	grantProjectRole(t, pool, store, "progress-reviewer", "project.reviewer", created.ID)
	grantProjectRole(t, pool, store, "progress-approver", "project.approver", created.ID)
	reviewer, err := store.FindActiveUserByLogin(context.Background(), "progress-reviewer")
	if err != nil {
		t.Fatal(err)
	}
	approver, err := store.FindActiveUserByLogin(context.Background(), "progress-approver")
	if err != nil {
		t.Fatal(err)
	}
	planResponse := doJSON(t, handler, http.MethodGet, "/api/v1/projects/"+created.ID+"/plan", nil, cookie, "")
	var plan planDetailView
	if err := json.Unmarshal(planResponse.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	base := "/api/v1/projects/" + created.ID + "/work-products/" + plan.WorkProductID
	assignments := map[string]any{"assignments": []map[string]string{
		{"user_id": reviewer.ID.String(), "assignment_role": "reviewer"},
		{"user_id": approver.ID.String(), "assignment_role": "approver"},
	}}
	if response := doJSON(t, handler, http.MethodPost, base+"/submit", assignments, cookie, csrf); response.Code != http.StatusCreated {
		t.Fatalf("подання плану: %d %s", response.Code, response.Body.String())
	}
	path := "/api/v1/projects/" + created.ID + "/plan/review"
	evidencePath := "/api/v1/projects/" + created.ID + "/plan/approval-evidence"
	if before := doJSON(t, handler, http.MethodGet, evidencePath, nil, cookie, ""); before.Code != http.StatusNotFound {
		t.Fatalf("до погодження не повинно бути підтверджень: %d", before.Code)
	}
	reviewerLogin := doJSON(t, handler, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"login": "progress-reviewer", "password": "Review-Pass-12345"}, nil, "")
	var reviewerBody loginResponse
	if err := json.Unmarshal(reviewerLogin.Body.Bytes(), &reviewerBody); err != nil {
		t.Fatal(err)
	}
	reviewerCookie := sessionCookieFromResponse(t, reviewerLogin)
	progress := doJSON(t, handler, http.MethodGet, path, nil, reviewerCookie, "")
	if progress.Code != http.StatusOK {
		t.Fatalf("стан погодження: %d %s", progress.Code, progress.Body.String())
	}
	var review project.OpenPlanReview
	if err := json.Unmarshal(progress.Body.Bytes(), &review); err != nil {
		t.Fatal(err)
	}
	if review.RevisionID.String() != plan.RevisionID || review.HasPositiveReview || len(review.Participants) != 2 {
		t.Fatalf("неочікуваний початковий стан: %+v", review)
	}
	for _, participant := range review.Participants {
		if participant.AssignedToMe != (participant.Role == "reviewer") {
			t.Errorf("невірне призначення для рецензента: %+v", participant)
		}
	}
	if response := doJSON(t, handler, http.MethodPost, base+"/reviews", map[string]string{"operation_key": "review-progress-key"}, reviewerCookie, reviewerBody.CSRFToken); response.Code != http.StatusCreated {
		t.Fatalf("рецензія: %d %s", response.Code, response.Body.String())
	}
	progress = doJSON(t, handler, http.MethodGet, path, nil, cookie, "")
	if err := json.Unmarshal(progress.Body.Bytes(), &review); err != nil || !review.HasPositiveReview {
		t.Fatalf("позитивна рецензія не відображена: %+v, %v", review, err)
	}
	approverLogin := doJSON(t, handler, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"login": "progress-approver", "password": "Approve-Pass-12345"}, nil, "")
	var approverBody loginResponse
	if err := json.Unmarshal(approverLogin.Body.Bytes(), &approverBody); err != nil {
		t.Fatal(err)
	}
	if conflict := doJSON(t, handler, http.MethodPost, base+"/approvals", map[string]string{"operation_key": "review-progress-key"}, sessionCookieFromResponse(t, approverLogin), approverBody.CSRFToken); conflict.Code != http.StatusConflict || !strings.Contains(conflict.Body.String(), "operation_conflict") {
		t.Fatalf("ключ рецензента не повинен повертати погоджувачу його рішення: %d %s", conflict.Code, conflict.Body.String())
	}
	if response := doJSON(t, handler, http.MethodPost, base+"/approvals", nil, sessionCookieFromResponse(t, approverLogin), approverBody.CSRFToken); response.Code != http.StatusCreated {
		t.Fatalf("погодження: %d %s", response.Code, response.Body.String())
	}
	evidenceResponse := doJSON(t, handler, http.MethodGet, evidencePath, nil, cookie, "")
	if evidenceResponse.Code != http.StatusOK {
		t.Fatalf("підтвердження погодженої ревізії: %d %s", evidenceResponse.Code, evidenceResponse.Body.String())
	}
	var evidence []project.PlanApprovalEvidence
	if err := json.Unmarshal(evidenceResponse.Body.Bytes(), &evidence); err != nil {
		t.Fatal(err)
	}
	if len(evidence) != 2 || evidence[0].DecisionKind != project.DecisionReview || evidence[1].DecisionKind != project.DecisionApproval ||
		evidence[0].RevisionID.String() != plan.RevisionID || evidence[1].RevisionID.String() != plan.RevisionID {
		t.Fatalf("очікували два незалежні рішення щодо точної ревізії: %+v", evidence)
	}
	if closed := doJSON(t, handler, http.MethodGet, path, nil, cookie, ""); closed.Code != http.StatusNotFound {
		t.Fatalf("закритий запит не має показуватися як відкритий: %d", closed.Code)
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
	if _, err := store.Pool().Exec(context.Background(), `INSERT INTO core.rule_definitions
		(rule_key, trigger_key, enforcement_level, when_condition, assert_condition)
		VALUES ('rule.test.http_plan_revise', 'trigger.core.before_wp_transition', 'MANDATORY_VETO',
		'{"predicate_key":"field_equals","params":{"field":"action","value":"plan.revise"}}',
		'{"predicate_key":"always_false"}')`); err != nil {
		t.Fatal(err)
	}
	planURL := "/api/v1/projects/" + createdProject.ID + "/plan/revisions"
	revisionInput := map[string]any{"expected_row_version": plan.RowVersion, "body": "# Plan\n", "manifest": plan.Manifest}
	veto := doJSON(t, handler, http.MethodPost, planURL, revisionInput, cookie, csrfToken)
	if veto.Code != http.StatusUnprocessableEntity {
		t.Fatalf("вето ревізії плану має дати 422, отримано %d: %s", veto.Code, veto.Body.String())
	}
	if _, err := store.Pool().Exec(context.Background(),
		`UPDATE core.rule_definitions SET active = false WHERE rule_key = 'rule.test.http_plan_revise'`); err != nil {
		t.Fatal(err)
	}
	revise := doJSON(t, handler, http.MethodPost, "/api/v1/projects/"+createdProject.ID+"/plan/revisions",
		revisionInput, cookie, csrfToken)
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
