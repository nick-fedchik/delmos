package repository_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"delmos/internal/auth"
	"delmos/internal/migrate"
	"delmos/internal/project"
	"delmos/internal/repository"
	"delmos/internal/testsupport"
)

func newTestStores(t *testing.T) (*repository.Store, *project.Store, *auth.Store) {
	t.Helper()

	pool, err := pgxpool.New(context.Background(), testsupport.NewDatabase(t))
	if err != nil {
		t.Fatalf("підключення до тимчасової бази: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := migrate.Apply(context.Background(), pool, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatalf("застосування міграцій: %v", err)
	}

	return repository.NewStore(pool, repository.NewPlainGitProvider()), project.NewStore(pool), auth.NewStore(pool)
}

func TestBindAndGetRepositoryBinding(t *testing.T) {
	ctx := context.Background()
	repos, projects, authStore := newTestStores(t)

	hash, _ := auth.HashPassword("irrelevant-password")
	userID, err := authStore.BootstrapAdministrator(ctx, "creator", "creator", hash)
	if err != nil {
		t.Fatalf("створення тестового користувача: %v", err)
	}
	detail, err := projects.CreateWithPlan(ctx, userID, "REPO-001", "Repo Project", "")
	if err != nil {
		t.Fatalf("створення проєкту: %v", err)
	}

	remote := filepath.Join(t.TempDir(), "repo.git")
	binding, err := repos.Bind(ctx, userID, detail.Project.ID, remote, "")
	if err != nil {
		t.Fatalf("прив'язка сховища: %v", err)
	}
	if binding.Status != "active" || binding.DefaultBranch != "main" {
		t.Errorf("очікувалася активна прив'язка з гілкою main за замовчуванням, отримано %+v", binding)
	}

	fetched, err := repos.Get(ctx, detail.Project.ID)
	if err != nil {
		t.Fatalf("читання прив'язки: %v", err)
	}
	if fetched.RemoteURL != remote {
		t.Errorf("очікувався remote_url=%s, отримано %s", remote, fetched.RemoteURL)
	}
}

func TestGetRepositoryReturnsNotFoundWithoutBinding(t *testing.T) {
	ctx := context.Background()
	repos, projects, authStore := newTestStores(t)

	hash, _ := auth.HashPassword("irrelevant-password")
	userID, _ := authStore.BootstrapAdministrator(ctx, "creator2", "creator2", hash)
	detail, err := projects.CreateWithPlan(ctx, userID, "REPO-002", "Repo Project 2", "")
	if err != nil {
		t.Fatalf("створення проєкту: %v", err)
	}

	if _, err := repos.Get(ctx, detail.Project.ID); !errors.Is(err, repository.ErrBindingNotFound) {
		t.Errorf("очікувалася ErrBindingNotFound, отримано %v", err)
	}
}

func TestExportWorkProductWritesCommit(t *testing.T) {
	ctx := context.Background()
	repos, projects, authStore := newTestStores(t)

	hash, _ := auth.HashPassword("irrelevant-password")
	userID, _ := authStore.BootstrapAdministrator(ctx, "creator3", "creator3", hash)
	detail, err := projects.CreateWithPlan(ctx, userID, "REPO-003", "Repo Project 3", "")
	if err != nil {
		t.Fatalf("створення проєкту: %v", err)
	}

	remote := filepath.Join(t.TempDir(), "repo.git")
	if _, err := repos.Bind(ctx, userID, detail.Project.ID, remote, ""); err != nil {
		t.Fatalf("прив'язка сховища: %v", err)
	}

	result, err := repos.ExportWorkProduct(ctx, userID, detail.Project.ID, "PLAN-001", detail.PlanLatest.Body, "DELMOS", "delmos@example.invalid")
	if err != nil {
		t.Fatalf("експорт work product: %v", err)
	}
	if result.CommitSHA == "" {
		t.Error("очікувався непорожній commit_sha")
	}
}

func TestExportWorkProductFailsWithoutBinding(t *testing.T) {
	ctx := context.Background()
	repos, projects, authStore := newTestStores(t)

	hash, _ := auth.HashPassword("irrelevant-password")
	userID, _ := authStore.BootstrapAdministrator(ctx, "creator4", "creator4", hash)
	detail, err := projects.CreateWithPlan(ctx, userID, "REPO-004", "Repo Project 4", "")
	if err != nil {
		t.Fatalf("створення проєкту: %v", err)
	}

	if _, err := repos.ExportWorkProduct(ctx, userID, detail.Project.ID, "PLAN-001", "body", "A", "a@example.invalid"); !errors.Is(err, repository.ErrBindingNotFound) {
		t.Errorf("очікувалася ErrBindingNotFound, отримано %v", err)
	}
}
