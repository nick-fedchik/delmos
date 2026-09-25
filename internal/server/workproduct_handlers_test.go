package server

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"testing"

	"delmos/internal/project"

	"github.com/google/uuid"
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

func TestCreateSpecificationFlow(t *testing.T) {
	handler, store := newProjectTestRouter(t)
	cookie, csrfToken := loginAsProjectManager(t, handler, store)
	proj := createTestProject(t, handler, cookie, csrfToken, "SPEC-H-001")

	createRequirement := doJSON(t, handler, http.MethodPost, "/api/v1/projects/"+proj.ID+"/work-products",
		map[string]any{"code": "REQ-001", "type": "requirement", "title": "Requirement", "body": "body"},
		cookie, csrfToken)
	if createRequirement.Code != http.StatusCreated {
		t.Fatalf("очікувався 201 для вимоги, отримано %d: %s", createRequirement.Code, createRequirement.Body.String())
	}
	var requirement workProductView
	if err := json.Unmarshal(createRequirement.Body.Bytes(), &requirement); err != nil {
		t.Fatalf("розбір вимоги: %v", err)
	}
	payloadHash, err := base64.RawURLEncoding.DecodeString(requirement.Latest.PayloadHash)
	if err != nil {
		t.Fatalf("декодування payload_hash: %v", err)
	}
	occurrenceID := uuid.New().String()
	manifest := map[string]any{
		"manifest_version": "1.0.0",
		"sections":         []map[string]any{{"section_id": "root", "title": "Requirements", "order": 1}},
		"occurrences": []map[string]any{{
			"occurrence_id": occurrenceID, "section_id": "root", "target_wp_id": requirement.ID,
			"target_revision_id": uuid.New().String(), "target_payload_hash": hex.EncodeToString(payloadHash), "order": 1,
		}},
	}
	// The target revision id is filled from the first revision returned by the API contract.
	manifest["occurrences"].([]map[string]any)[0]["target_revision_id"] = requirement.Latest.RevisionID
	elementsHash := project.CalculateElementsHash([]project.CompositionElement{{
		TargetWorkProductID: requirement.ID, TargetRevisionID: requirement.Latest.RevisionID,
		TargetPayloadHash: hex.EncodeToString(payloadHash),
	}})
	manifest["elements_hash"] = hex.EncodeToString(elementsHash)

	create := doJSON(t, handler, http.MethodPost, "/api/v1/projects/"+proj.ID+"/specifications",
		map[string]any{"code": "SPEC-001", "title": "System specification", "body": "body", "manifest": manifest},
		cookie, csrfToken)
	if create.Code != http.StatusCreated {
		t.Fatalf("очікувався 201 для специфікації, отримано %d: %s", create.Code, create.Body.String())
	}
	var specification specificationView
	if err := json.Unmarshal(create.Body.Bytes(), &specification); err != nil {
		t.Fatalf("розбір специфікації: %v", err)
	}
	if specification.ID == "" || specification.WorkProduct.Type != "requirement" || specification.WorkProduct.Status != "draft" {
		t.Fatalf("некоректна специфікація: %+v", specification)
	}

	get := doJSON(t, handler, http.MethodGet, "/api/v1/projects/"+proj.ID+"/specifications/"+specification.ID, nil, cookie, "")
	if get.Code != http.StatusOK {
		t.Fatalf("очікувався 200 при читанні специфікації, отримано %d: %s", get.Code, get.Body.String())
	}
	var fetched specificationView
	if err := json.Unmarshal(get.Body.Bytes(), &fetched); err != nil {
		t.Fatalf("розбір прочитаної специфікації: %v", err)
	}
	if fetched.Manifest.ManifestVersion != "1.0.0" || len(fetched.Manifest.Occurrences) != 1 {
		t.Fatalf("маніфест специфікації не збережено: %+v", fetched.Manifest)
	}

	reviseBody := map[string]any{
		"expected_row_version": fetched.WorkProduct.RowVersion,
		"body":                 "revised body",
		"manifest":             manifest,
	}
	revise := doJSON(t, handler, http.MethodPost, "/api/v1/projects/"+proj.ID+"/specifications/"+specification.ID+"/revisions", reviseBody, cookie, csrfToken)
	if revise.Code != http.StatusCreated {
		t.Fatalf("очікувався 201 при ревізії специфікації, отримано %d: %s", revise.Code, revise.Body.String())
	}
	var revised specificationView
	if err := json.Unmarshal(revise.Body.Bytes(), &revised); err != nil {
		t.Fatalf("розбір ревізії специфікації: %v", err)
	}
	if revised.WorkProduct.Latest.RevisionNumber != 2 || revised.WorkProduct.RowVersion != 2 {
		t.Fatalf("некоректна ревізія специфікації: %+v", revised)
	}
	if revised.ID == specification.ID {
		t.Fatal("нова ревізія має отримати новий specification_id")
	}

	oldSnapshot := doJSON(t, handler, http.MethodGet, "/api/v1/projects/"+proj.ID+"/specifications/"+specification.ID, nil, cookie, "")
	if oldSnapshot.Code != http.StatusOK {
		t.Fatalf("очікувався 200 для старого snapshot, отримано %d: %s", oldSnapshot.Code, oldSnapshot.Body.String())
	}
	var fetchedOld specificationView
	if err := json.Unmarshal(oldSnapshot.Body.Bytes(), &fetchedOld); err != nil {
		t.Fatalf("розбір старого snapshot: %v", err)
	}
	if fetchedOld.Manifest.ElementsHash != specification.Manifest.ElementsHash || fetchedOld.WorkProduct.Latest.RevisionID != specification.WorkProduct.Latest.RevisionID {
		t.Fatal("попередній manifest або revision були змінені після нової ревізії")
	}

	stale := doJSON(t, handler, http.MethodPost, "/api/v1/projects/"+proj.ID+"/specifications/"+specification.ID+"/revisions", reviseBody, cookie, csrfToken)
	if stale.Code != http.StatusConflict {
		t.Fatalf("застаріла ревізія специфікації має повертати 409, отримано %d: %s", stale.Code, stale.Body.String())
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

func TestCreateWorkProductRejectsAdditionalProjectPlan(t *testing.T) {
	handler, store := newProjectTestRouter(t)
	cookie, csrfToken := loginAsProjectManager(t, handler, store)
	proj := createTestProject(t, handler, cookie, csrfToken, "WP-H-PLAN-001")

	create := doJSON(t, handler, http.MethodPost, "/api/v1/projects/"+proj.ID+"/work-products",
		map[string]any{"code": "PLAN-002", "type": "plan", "title": "Other plan", "body": ""}, cookie, csrfToken)
	if create.Code != http.StatusUnprocessableEntity {
		t.Errorf("другий project plan має повертати 422, отримано %d", create.Code)
	}
}
