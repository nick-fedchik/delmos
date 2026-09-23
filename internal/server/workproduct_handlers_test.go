package server

import (
	"encoding/json"
	"net/http"
	"testing"
)

func createTestProject(t *testing.T, handler http.Handler, cookie *http.Cookie, csrfToken, code string) projectView {
	t.Helper()

	create := doJSON(t, handler, http.MethodPost, "/api/v1/projects",
		map[string]string{"code": code, "name": "Project " + code}, cookie, csrfToken)
	if create.Code != http.StatusCreated {
		t.Fatalf("очікувався 201 при створенні проєкту, отримано %d: %s", create.Code, create.Body.String())
	}

	var view projectView
	if err := json.Unmarshal(create.Body.Bytes(), &view); err != nil {
		t.Fatalf("розбір тіла відповіді створення проєкту: %v", err)
	}

	return view
}

func TestCreateWorkProductFlow(t *testing.T) {
	handler, store := newProjectTestRouter(t)
	cookie, csrfToken := loginAsProjectManager(t, handler, store)
	proj := createTestProject(t, handler, cookie, csrfToken, "WP-H-001")

	create := doJSON(t, handler, http.MethodPost, "/api/v1/projects/"+proj.ID+"/work-products",
		map[string]any{"code": "REQ-001", "type": "requirement", "title": "Живлення", "body": "перша версія"},
		cookie, csrfToken)
	if create.Code != http.StatusCreated {
		t.Fatalf("очікувався 201, отримано %d: %s", create.Code, create.Body.String())
	}

	var wp workProductView
	if err := json.Unmarshal(create.Body.Bytes(), &wp); err != nil {
		t.Fatalf("розбір тіла відповіді створення work product: %v", err)
	}
	if wp.Status != "draft" || wp.Latest.RevisionNumber != 1 {
		t.Errorf("очікувався draft/r1, отримано %+v", wp)
	}

	get := doJSON(t, handler, http.MethodGet, "/api/v1/projects/"+proj.ID+"/work-products/"+wp.ID, nil, cookie, "")
	if get.Code != http.StatusOK {
		t.Fatalf("очікувався 200 при читанні work product, отримано %d", get.Code)
	}

	revise := doJSON(t, handler, http.MethodPost, "/api/v1/projects/"+proj.ID+"/work-products/"+wp.ID+"/revisions",
		map[string]any{"expected_row_version": wp.RowVersion, "body": "друга версія"}, cookie, csrfToken)
	if revise.Code != http.StatusCreated {
		t.Fatalf("очікувався 201 при ревізії, отримано %d: %s", revise.Code, revise.Body.String())
	}

	var revised workProductRevisionView
	_ = json.Unmarshal(revise.Body.Bytes(), &revised)
	if revised.RevisionNumber != 2 {
		t.Errorf("очікувався revision_number=2, отримано %d", revised.RevisionNumber)
	}

	staleRevise := doJSON(t, handler, http.MethodPost, "/api/v1/projects/"+proj.ID+"/work-products/"+wp.ID+"/revisions",
		map[string]any{"expected_row_version": wp.RowVersion, "body": "застаріла версія"}, cookie, csrfToken)
	if staleRevise.Code != http.StatusConflict {
		t.Errorf("повторна ревізія зі старим row_version має повертати 409, отримано %d", staleRevise.Code)
	}

	retire := doJSON(t, handler, http.MethodPost, "/api/v1/projects/"+proj.ID+"/work-products/"+wp.ID+"/retire",
		map[string]any{"expected_row_version": wp.RowVersion + 1}, cookie, csrfToken)
	if retire.Code != http.StatusNoContent {
		t.Fatalf("очікувався 204 при виведенні з експлуатації, отримано %d: %s", retire.Code, retire.Body.String())
	}

	list := doJSON(t, handler, http.MethodGet, "/api/v1/projects/"+proj.ID+"/work-products", nil, cookie, "")
	var summaries []workProductSummaryView
	_ = json.Unmarshal(list.Body.Bytes(), &summaries)

	var reqSummary *workProductSummaryView
	for i := range summaries {
		if summaries[i].Code == "REQ-001" {
			reqSummary = &summaries[i]
		}
	}
	// PLAN-001 теж є у цьому проєкті (створюється атомарно з Project) — перевіряємо саме REQ-001.
	if reqSummary == nil || reqSummary.Status != "obsolete" {
		t.Errorf("очікувався obsolete REQ-001 у списку, отримано %+v", summaries)
	}
}

func TestWorkProductEndpointsHiddenFromNonMember(t *testing.T) {
	handler, store := newProjectTestRouter(t)
	ownerCookie, ownerCSRF := loginAsProjectManager(t, handler, store)
	proj := createTestProject(t, handler, ownerCookie, ownerCSRF, "WP-H-002")

	bootstrapTestAdmin(t, store, "wp-outsider", "Outsider-Pass-1")
	outsiderLogin := doJSON(t, handler, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"login": "wp-outsider", "password": "Outsider-Pass-1"}, nil, "")
	var outsiderBody loginResponse
	_ = json.Unmarshal(outsiderLogin.Body.Bytes(), &outsiderBody)
	outsiderCookie := sessionCookieFromResponse(t, outsiderLogin)

	create := doJSON(t, handler, http.MethodPost, "/api/v1/projects/"+proj.ID+"/work-products",
		map[string]any{"code": "REQ-001", "type": "requirement", "title": "T", "body": "v1"}, outsiderCookie, outsiderBody.CSRFToken)
	if create.Code != http.StatusNotFound {
		t.Errorf("сторонній без wp.create має отримати 404 (не 403), отримано %d", create.Code)
	}
}

func TestCreateWorkProductRejectsUnknownType(t *testing.T) {
	handler, store := newProjectTestRouter(t)
	cookie, csrfToken := loginAsProjectManager(t, handler, store)
	proj := createTestProject(t, handler, cookie, csrfToken, "WP-H-003")

	create := doJSON(t, handler, http.MethodPost, "/api/v1/projects/"+proj.ID+"/work-products",
		map[string]any{"code": "X-001", "type": "not_a_type", "title": "T", "body": "v1"}, cookie, csrfToken)
	if create.Code != http.StatusUnprocessableEntity {
		t.Errorf("невідомий тип має повертати 422, отримано %d", create.Code)
	}
}
