package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	legacyrouter "github.com/getkin/kin-openapi/routers/legacy"
)

// loadContractSpec завантажує заморожений REST-контракт v1.0.0. Шлях обчислюється
// відносно цього файлу, щоб тест не залежав від робочого каталогу запуску `go test`.
func loadContractSpec(t *testing.T) (*openapi3.T, routers.Router) {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("не вдалося визначити шлях до контрактного тесту")
	}
	specPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "docs", "api", "openapi.v1.yaml")

	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		t.Fatalf("завантаження %s: %v", specPath, err)
	}
	if err := doc.Validate(loader.Context); err != nil {
		t.Fatalf("специфікація %s невалідна: %v", specPath, err)
	}

	router, err := legacyrouter.NewRouter(doc)
	if err != nil {
		t.Fatalf("побудова роутера зі специфікації: %v", err)
	}

	return doc, router
}

// contractCall виконує один HTTP-виклик через реальний mux сервера й звіряє і
// запит, і відповідь із docs/api/openapi.v1.yaml (справжній contract test, а не
// лише ручні перевірки статус-кодів).
func contractCall(
	t *testing.T,
	handler http.Handler,
	router routers.Router,
	method, path string,
	cookie *http.Cookie,
	csrfToken string,
	body any,
) *httptest.ResponseRecorder {
	t.Helper()

	var bodyReader *bytes.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("кодування тіла запиту: %v", err)
		}
		bodyReader = bytes.NewReader(encoded)
	} else {
		bodyReader = bytes.NewReader(nil)
	}

	req, err := http.NewRequest(method, path, bodyReader)
	if err != nil {
		t.Fatalf("побудова запиту %s %s: %v", method, path, err)
	}
	req.RemoteAddr = "192.0.2.1:1234"
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if csrfToken != "" {
		req.Header.Set("X-CSRF-Token", csrfToken)
	}

	route, pathParams, err := router.FindRoute(req)
	if err != nil {
		t.Fatalf("маршрут %s %s відсутній у docs/api/openapi.v1.yaml: %v", method, path, err)
	}

	reqValidation := &openapi3filter.RequestValidationInput{
		Request:     req,
		PathParams:  pathParams,
		Route:       route,
		QueryParams: req.URL.Query(),
		Options: &openapi3filter.Options{
			// Cookie-сесія перевіряється сервером поза цим тестом; тут звіряється
			// лише форма запиту/відповіді, а не саму автентифікацію.
			AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
		},
	}
	if err := openapi3filter.ValidateRequest(context.Background(), reqValidation); err != nil {
		t.Fatalf("запит %s %s не відповідає специфікації: %v", method, path, err)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	responseValidation := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: reqValidation,
		Status:                 recorder.Code,
		Header:                 recorder.Header(),
		Body:                   nopCloser{bytes.NewReader(recorder.Body.Bytes())},
	}
	if err := openapi3filter.ValidateResponse(context.Background(), responseValidation); err != nil {
		t.Fatalf("відповідь %s %s (%d) не відповідає специфікації:\n%s\n%v",
			method, path, recorder.Code, recorder.Body.String(), err)
	}

	return recorder
}

type nopCloser struct{ *bytes.Reader }

func (nopCloser) Close() error { return nil }

// TestContractFullLifecycle проганяє наскрізний сценарій MVP (вхід → роль →
// проєкт/план → work product → сховище/експорт → вихід) через реальний mux і
// перевіряє кожен запит/відповідь проти docs/api/openapi.v1.yaml.
func TestContractFullLifecycle(t *testing.T) {
	handler, authStore := newProjectTestRouter(t)
	_, router := loadContractSpec(t)

	bootstrapTestAdmin(t, authStore, "contract-admin", "Contract-Admin-Pass-1")
	login := contractCall(t, handler, router, http.MethodPost, "/api/v1/auth/login", nil, "",
		map[string]string{"login": "contract-admin", "password": "Contract-Admin-Pass-1"})

	var loginBody loginResponse
	mustUnmarshal(t, login, &loginBody)
	cookie := sessionCookieFromResponse(t, login)

	contractCall(t, handler, router, http.MethodGet, "/api/v1/auth/session", cookie, "", nil)

	adminUser, err := authStore.FindActiveUserByLogin(context.Background(), "contract-admin")
	if err != nil {
		t.Fatalf("пошук користувача: %v", err)
	}
	grant := contractCall(t, handler, router, http.MethodPost, "/api/v1/role-bindings", cookie, loginBody.CSRFToken,
		map[string]string{"user_id": adminUser.ID.String(), "role_key": "project.manager", "reason": "contract test"})
	var grantBody roleBindingView
	mustUnmarshal(t, grant, &grantBody)

	// Дозволи оновлюються лише в новій сесії (SWR-42 §2) — повторний вхід.
	login = contractCall(t, handler, router, http.MethodPost, "/api/v1/auth/login", nil, "",
		map[string]string{"login": "contract-admin", "password": "Contract-Admin-Pass-1"})
	mustUnmarshal(t, login, &loginBody)
	cookie = sessionCookieFromResponse(t, login)

	create := contractCall(t, handler, router, http.MethodPost, "/api/v1/projects", cookie, loginBody.CSRFToken,
		map[string]string{"code": "CONTRACT-001", "name": "Contract Project", "description": "e2e"})
	var project projectView
	mustUnmarshal(t, create, &project)

	contractCall(t, handler, router, http.MethodGet, "/api/v1/projects", cookie, "", nil)
	contractCall(t, handler, router, http.MethodGet, "/api/v1/projects/"+project.ID, cookie, "", nil)

	createWP := contractCall(t, handler, router, http.MethodPost, "/api/v1/projects/"+project.ID+"/work-products",
		cookie, loginBody.CSRFToken,
		map[string]any{"code": "REQ-001", "type": "requirement", "title": "Живлення", "body": "v1"})
	var wp workProductView
	mustUnmarshal(t, createWP, &wp)

	contractCall(t, handler, router, http.MethodGet, "/api/v1/projects/"+project.ID+"/work-products", cookie, "", nil)
	contractCall(t, handler, router, http.MethodGet,
		"/api/v1/projects/"+project.ID+"/work-products/"+wp.ID, cookie, "", nil)

	revise := contractCall(t, handler, router, http.MethodPost,
		"/api/v1/projects/"+project.ID+"/work-products/"+wp.ID+"/revisions", cookie, loginBody.CSRFToken,
		map[string]any{"expected_row_version": wp.RowVersion, "body": "v2"})
	var revision workProductRevisionView
	mustUnmarshal(t, revise, &revision)

	remote := t.TempDir() + "/contract-repo.git"
	contractCall(t, handler, router, http.MethodPost, "/api/v1/projects/"+project.ID+"/repository",
		cookie, loginBody.CSRFToken, map[string]string{"remote_url": remote})
	contractCall(t, handler, router, http.MethodGet, "/api/v1/projects/"+project.ID+"/repository", cookie, "", nil)

	contractCall(t, handler, router, http.MethodPost,
		"/api/v1/projects/"+project.ID+"/work-products/"+wp.ID+"/export", cookie, loginBody.CSRFToken, nil)

	contractCall(t, handler, router, http.MethodPost,
		"/api/v1/projects/"+project.ID+"/work-products/"+wp.ID+"/retire", cookie, loginBody.CSRFToken,
		map[string]any{"expected_row_version": wp.RowVersion + 1})

	contractCall(t, handler, router, http.MethodDelete, "/api/v1/role-bindings/"+grantBody.ID, cookie, loginBody.CSRFToken, nil)
	contractCall(t, handler, router, http.MethodPost, "/api/v1/auth/logout", cookie, loginBody.CSRFToken, nil)
}

// TestContractNegativeResponses перевіряє, що і заборонені/невідомі відповіді
// (401/404) також відповідають ErrorBody зі специфікації.
func TestContractNegativeResponses(t *testing.T) {
	handler, _ := newProjectTestRouter(t)
	_, router := loadContractSpec(t)

	contractCall(t, handler, router, http.MethodGet, "/api/v1/auth/session", nil, "", nil)
	contractCall(t, handler, router, http.MethodGet,
		"/api/v1/projects/00000000-0000-0000-0000-000000000000", nil, "", nil)
}

func mustUnmarshal(t *testing.T, recorder *httptest.ResponseRecorder, target any) {
	t.Helper()

	if err := json.Unmarshal(recorder.Body.Bytes(), target); err != nil {
		t.Fatalf("розбір тіла відповіді (%s): %v", recorder.Body.String(), err)
	}
}
