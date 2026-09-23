package server

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/jackc/pgx/v5/pgxpool"

	"delmos/internal/auth"
	"delmos/internal/migrate"
	"delmos/internal/project"
	"delmos/internal/ratelimit"
	"delmos/internal/repository"
	"delmos/internal/testsupport"
)

func newRepositoryTestRouter(t *testing.T) (http.Handler, *auth.Store) {
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
		Repositories: repository.NewStore(pool, repository.NewPlainGitProvider()),
		LoginLimiter: ratelimit.New(rate.Every(time.Millisecond), 1000),
		CookieSecure: true,
	}

	return newRouter(testLogger(), deps), authStore
}

func TestBindRepositoryAndExportWorkProduct(t *testing.T) {
	handler, store := newRepositoryTestRouter(t)
	cookie, csrfToken := loginAsProjectManager(t, handler, store)
	proj := createTestProject(t, handler, cookie, csrfToken, "REPO-H-001")

	remote := filepath.Join(t.TempDir(), "repo.git")
	bind := doJSON(t, handler, http.MethodPost, "/api/v1/projects/"+proj.ID+"/repository",
		map[string]string{"remote_url": remote}, cookie, csrfToken)
	if bind.Code != http.StatusCreated {
		t.Fatalf("очікувався 201 при прив'язці сховища, отримано %d: %s", bind.Code, bind.Body.String())
	}

	var binding repositoryBindingView
	_ = json.Unmarshal(bind.Body.Bytes(), &binding)
	if binding.Status != "active" {
		t.Errorf("очікувався статус active, отримано %+v", binding)
	}

	get := doJSON(t, handler, http.MethodGet, "/api/v1/projects/"+proj.ID+"/repository", nil, cookie, "")
	if get.Code != http.StatusOK {
		t.Fatalf("очікувався 200 при читанні прив'язки, отримано %d", get.Code)
	}
}

func TestGetRepositoryWithoutBindingReturnsNotFound(t *testing.T) {
	handler, store := newRepositoryTestRouter(t)
	cookie, csrfToken := loginAsProjectManager(t, handler, store)
	proj := createTestProject(t, handler, cookie, csrfToken, "REPO-H-002")

	get := doJSON(t, handler, http.MethodGet, "/api/v1/projects/"+proj.ID+"/repository", nil, cookie, "")
	if get.Code != http.StatusNotFound {
		t.Errorf("проєкт без прив'язки має повертати 404, отримано %d", get.Code)
	}
}

func TestExportPlanWorkProductAfterBinding(t *testing.T) {
	handler, store := newRepositoryTestRouter(t)
	cookie, csrfToken := loginAsProjectManager(t, handler, store)
	proj := createTestProject(t, handler, cookie, csrfToken, "REPO-H-003")

	remote := filepath.Join(t.TempDir(), "repo.git")
	doJSON(t, handler, http.MethodPost, "/api/v1/projects/"+proj.ID+"/repository",
		map[string]string{"remote_url": remote}, cookie, csrfToken)

	list := doJSON(t, handler, http.MethodGet, "/api/v1/projects/"+proj.ID+"/work-products", nil, cookie, "")
	var summaries []workProductSummaryView
	_ = json.Unmarshal(list.Body.Bytes(), &summaries)
	if len(summaries) != 1 {
		t.Fatalf("очікувався один work product (PLAN-001), отримано %+v", summaries)
	}

	export := doJSON(t, handler, http.MethodPost,
		"/api/v1/projects/"+proj.ID+"/work-products/"+summaries[0].ID+"/export", nil, cookie, csrfToken)
	if export.Code != http.StatusOK {
		t.Fatalf("очікувався 200 при експорті PLAN-001, отримано %d: %s", export.Code, export.Body.String())
	}

	var result exportResultView
	_ = json.Unmarshal(export.Body.Bytes(), &result)
	if result.CommitSHA == "" {
		t.Error("очікувався непорожній commit_sha")
	}
}
