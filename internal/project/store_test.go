package project_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"delmos/internal/auth"
	"delmos/internal/automation"
	"delmos/internal/migrate"
	"delmos/internal/project"
	"delmos/internal/testsupport"
)

func newTestStore(t *testing.T) (*project.Store, *auth.Store) {
	t.Helper()

	pool, err := pgxpool.New(context.Background(), testsupport.NewDatabase(t))
	if err != nil {
		t.Fatalf("підключення до тимчасової бази: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := migrate.Apply(context.Background(), pool, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatalf("застосування міграцій: %v", err)
	}

	return project.NewStore(pool), auth.NewStore(pool)
}

// newTestStoreWithEngine — той самий контур, що й newTestStore, але додатково
// підключає рушій автоматизації з зареєстрованим обробником
// trigger.core.after_revision_committed (як у бойовому cmd/delmos/main.go),
// щоб тести могли детерміновано прогнати asynchronous-ефекти через
// runOutboxOnce замість очікування фонового тикера.
func newTestStoreWithEngine(t *testing.T) (*project.Store, *auth.Store, *automation.Engine) {
	t.Helper()

	pool, err := pgxpool.New(context.Background(), testsupport.NewDatabase(t))
	if err != nil {
		t.Fatalf("підключення до тимчасової бази: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := migrate.Apply(context.Background(), pool, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatalf("застосування міграцій: %v", err)
	}

	projects := project.NewStore(pool)
	engine := automation.NewEngine(pool, slog.New(slog.NewTextHandler(io.Discard, nil)))
	engine.RegisterHandler("trigger.core.after_revision_committed", func(ctx context.Context, tx pgx.Tx, env automation.Envelope) error {
		workProductID, err := uuid.Parse(env.Payload["work_product_id"].(string))
		if err != nil {
			return err
		}
		revisionID, err := uuid.Parse(env.Payload["revision_id"].(string))
		if err != nil {
			return err
		}
		return projects.ApplyRevisionCommittedEffects(ctx, tx, workProductID, revisionID)
	})

	return projects, auth.NewStore(pool), engine
}

// runOutboxOnce виконує один цикл диспетчеризації й обробки — детерміновано
// доводить чергу outbox до порожнього стану для вже записаних подій.
func runOutboxOnce(t *testing.T, engine *automation.Engine) {
	t.Helper()
	ctx := context.Background()
	if _, err := engine.DispatchPending(ctx); err != nil {
		t.Fatalf("диспетчеризація подій outbox: %v", err)
	}
	if _, err := engine.ClaimAndProcess(ctx); err != nil {
		t.Fatalf("обробка доставок outbox: %v", err)
	}
}

// mustParseUUIDField читає рядкове поле payload конверта події як uuid.UUID
// у тестових обробниках trigger.core.after_revision_committed.
func mustParseUUIDField(t *testing.T, env automation.Envelope, field string) uuid.UUID {
	t.Helper()
	raw, ok := env.Payload[field].(string)
	if !ok {
		t.Fatalf("поле %s відсутнє у payload події %s", field, env.EventKey)
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		t.Fatalf("некоректний uuid у полі %s: %v", field, err)
	}
	return id
}

func newTestUser(t *testing.T, authStore *auth.Store, login string) uuid.UUID {
	t.Helper()

	hash, err := auth.HashPassword("irrelevant-password")
	if err != nil {
		t.Fatalf("хешування пароля: %v", err)
	}
	userID, err := authStore.BootstrapAdministrator(context.Background(), login, login, hash)
	if err != nil {
		t.Fatalf("створення тестового користувача: %v", err)
	}

	return userID
}

func TestCreateWithPlanIsAtomicAndImmutable(t *testing.T) {
	ctx := context.Background()
	store, authStore := newTestStore(t)
	userID := newTestUser(t, authStore, "creator")

	detail, err := store.CreateWithPlan(ctx, userID, "ATOM-001", "Atomic Project", "опис")
	if err != nil {
		t.Fatalf("створення проєкту: %v", err)
	}

	if detail.Plan.Code != "PLAN-001" || detail.Plan.Status != "draft" {
		t.Errorf("PLAN-001 має створюватися в статусі draft, отримано %+v", detail.Plan)
	}
	if detail.PlanLatest.RevisionNumber != 1 {
		t.Errorf("перша ревізія плану має мати номер 1, отримано %d", detail.PlanLatest.RevisionNumber)
	}
	if len(detail.PlanLatest.PayloadHash) != 32 || len(detail.PlanLatest.ContentHash) != 32 {
		t.Errorf("хеші SHA-256 мають бути по 32 байти, отримано payload=%d content=%d",
			len(detail.PlanLatest.PayloadHash), len(detail.PlanLatest.ContentHash))
	}

	fetched, err := store.Get(ctx, detail.Project.ID)
	if err != nil {
		t.Fatalf("читання проєкту: %v", err)
	}
	if fetched.PlanLatest.RevisionNumber != detail.PlanLatest.RevisionNumber {
		t.Errorf("повторне читання має повертати ту саму ревізію плану")
	}
}

func TestCreateWithPlanRejectsDuplicateCode(t *testing.T) {
	ctx := context.Background()
	store, authStore := newTestStore(t)
	userID := newTestUser(t, authStore, "creator")

	if _, err := store.CreateWithPlan(ctx, userID, "DUP-001", "First", ""); err != nil {
		t.Fatalf("перше створення: %v", err)
	}

	if _, err := store.CreateWithPlan(ctx, userID, "DUP-001", "Second", ""); !errors.Is(err, project.ErrCodeTaken) {
		t.Errorf("очікувалася ErrCodeTaken, отримано %v", err)
	}
}

func TestCreatorReceivesProjectOwnerBinding(t *testing.T) {
	ctx := context.Background()
	store, authStore := newTestStore(t)
	userID := newTestUser(t, authStore, "creator")

	detail, err := store.CreateWithPlan(ctx, userID, "OWN-001", "Owned", "")
	if err != nil {
		t.Fatalf("створення проєкту: %v", err)
	}

	permissions, err := authStore.ActiveProjectPermissions(ctx, userID, detail.Project.ID)
	if err != nil {
		t.Fatalf("читання дозволів проєкту: %v", err)
	}
	if !permissions["project.read"] {
		t.Error("творець проєкту має автоматично отримати project.read (project.owner)")
	}
}

func TestListForActorExcludesForeignProjects(t *testing.T) {
	ctx := context.Background()
	store, authStore := newTestStore(t)
	owner := newTestUser(t, authStore, "owner")
	outsider := newTestUser(t, authStore, "outsider")

	if _, err := store.CreateWithPlan(ctx, owner, "OWN-002", "Owned2", ""); err != nil {
		t.Fatalf("створення проєкту: %v", err)
	}

	summaries, err := store.ListForActor(ctx, outsider)
	if err != nil {
		t.Fatalf("читання списку проєктів: %v", err)
	}
	if len(summaries) != 0 {
		t.Errorf("сторонній користувач не повинен бачити чужі проєкти, отримано %+v", summaries)
	}
}
