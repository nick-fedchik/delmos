package auth_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"delmos/internal/auth"
	"delmos/internal/migrate"
	"delmos/internal/testsupport"
)

func newTestStore(t *testing.T) *auth.Store {
	t.Helper()

	pool, err := pgxpool.New(context.Background(), testsupport.NewDatabase(t))
	if err != nil {
		t.Fatalf("підключення до тимчасової бази: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := migrate.Apply(context.Background(), pool, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatalf("застосування міграцій: %v", err)
	}

	return auth.NewStore(pool)
}

func TestLoginSucceedsAndGrantsBootstrappedPermissions(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	svc := auth.NewService(store)

	hash, err := auth.HashPassword("Correct-Horse-Battery-Staple-1")
	if err != nil {
		t.Fatalf("хешування пароля: %v", err)
	}
	if _, err := store.BootstrapAdministrator(ctx, "admin", "Перший адміністратор", hash); err != nil {
		t.Fatalf("bootstrap адміністратора: %v", err)
	}

	result, err := svc.Login(ctx, "admin", "Correct-Horse-Battery-Staple-1", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("очікувався успішний вхід: %v", err)
	}

	if !result.Permission["access.grant"] {
		t.Errorf("bootstrap-адміністратор має отримати access.grant, маємо %v", result.Permission)
	}
	if result.Token == "" || result.CSRFToken == "" {
		t.Error("токен сесії та CSRF-токен мають бути непорожніми")
	}

	actor, csrfSecret, err := svc.Authenticate(ctx, result.Token)
	if err != nil {
		t.Fatalf("автентифікація за токеном сесії: %v", err)
	}
	if actor.Login != "admin" || !actor.HasPermission("access.revoke") {
		t.Errorf("неочікуваний AuthContext: %+v", actor)
	}
	if err := auth.CheckCSRF(csrfSecret, result.CSRFToken); err != nil {
		t.Errorf("CSRF-токен, виданий при вході, має проходити перевірку: %v", err)
	}
}

func TestProjectPermissionsDoNotBecomeSystemPermissions(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	hash, err := auth.HashPassword("Scope-Test-Password-1")
	if err != nil {
		t.Fatal(err)
	}
	userID, err := store.BootstrapAdministrator(ctx, "scope-admin", "Адміністратор", hash)
	if err != nil {
		t.Fatal(err)
	}
	var projectID uuid.UUID
	if err := store.Pool().QueryRow(ctx,
		`INSERT INTO core.projects (code, name, created_by) VALUES ('SCOPE-TEST', 'Scope test', $1) RETURNING id`, userID).
		Scan(&projectID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Pool().Exec(ctx,
		`INSERT INTO core.role_bindings (user_id, role_key, scope_type, scope_id, granted_by, granted_reason)
		 VALUES ($1, 'project.owner', 'project', $2, $1, 'test setup')`, userID, projectID); err != nil {
		t.Fatal(err)
	}
	systemPermissions, err := store.ActivePermissions(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if systemPermissions["wp.edit"] {
		t.Fatal("проєктне wp.edit не повинне потрапити до системних дозволів")
	}
	projectPermissions, err := store.ActiveProjectPermissions(ctx, userID, projectID)
	if err != nil || !projectPermissions["wp.edit"] {
		t.Fatalf("wp.edit повинне залишитися чинним у проєкті: %v, %v", projectPermissions, err)
	}
}

func TestProjectReviewRolesAreScopedAndAudited(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	hash, err := auth.HashPassword("Project-Role-Test-Pass-1")
	if err != nil {
		t.Fatal(err)
	}
	adminID, err := store.BootstrapAdministrator(ctx, "role-admin", "Адміністратор", hash)
	if err != nil {
		t.Fatal(err)
	}
	reviewerID, err := store.BootstrapAdministrator(ctx, "role-reviewer", "Рецензент", hash)
	if err != nil {
		t.Fatal(err)
	}
	var projectID, otherProjectID uuid.UUID
	for index, project := range []struct {
		code string
		id   *uuid.UUID
	}{{"ROLE-TEST-1", &projectID}, {"ROLE-TEST-2", &otherProjectID}} {
		if err := store.Pool().QueryRow(ctx,
			`INSERT INTO core.projects (code, name, created_by) VALUES ($1, $2, $3) RETURNING id`,
			project.code, "Тест "+project.code, adminID).Scan(project.id); err != nil {
			t.Fatalf("створення проєкту %d: %v", index, err)
		}
	}
	users, err := store.SearchReviewRoleUsers(ctx, "role-reviewer", adminID)
	if err != nil || len(users) != 1 || users[0].ID != reviewerID {
		t.Fatalf("кандидат має знаходитися за логіном: %+v, %v", users, err)
	}
	projects, err := store.SearchReviewRoleProjects(ctx, "ROLE-TEST-1")
	if err != nil || len(projects) != 1 || projects[0].ID != projectID {
		t.Fatalf("проєкт має знаходитися за кодом: %+v, %v", projects, err)
	}
	if _, err := store.GrantProjectReviewRole(ctx, adminID, adminID, projectID, "project.reviewer", "самопризначення"); !errors.Is(err, auth.ErrReviewRoleSelfGrant) {
		t.Fatalf("самопризначення має бути заборонено: %v", err)
	}
	if _, err := store.GrantProjectReviewRole(ctx, adminID, reviewerID, projectID, "project.owner", "розширення прав"); !errors.Is(err, auth.ErrReviewRoleInvalid) {
		t.Fatalf("зайві ролі мають бути заборонені: %v", err)
	}
	bindingID, err := store.GrantProjectReviewRole(ctx, adminID, reviewerID, projectID, "project.reviewer", "Незалежна рецензія")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.GrantProjectReviewRole(ctx, adminID, reviewerID, projectID, "project.reviewer", "повтор"); !errors.Is(err, auth.ErrReviewRoleActive) {
		t.Fatalf("повторна роль має відхилятися: %v", err)
	}
	bindings, err := store.ListProjectReviewRoleBindings(ctx, projectID)
	if err != nil || len(bindings) != 1 || bindings[0].ID != bindingID {
		t.Fatalf("надана роль має з'явитися лише в потрібному проєкті: %+v, %v", bindings, err)
	}
	otherBindings, err := store.ListProjectReviewRoleBindings(ctx, otherProjectID)
	if err != nil || len(otherBindings) != 0 {
		t.Fatalf("роль не повинна переходити до іншого проєкту: %+v, %v", otherBindings, err)
	}
	if _, err := store.RevokeProjectReviewRole(ctx, adminID, otherProjectID, bindingID); err != nil {
		t.Fatal(err)
	}
	permissions, err := store.ActiveProjectPermissions(ctx, reviewerID, projectID)
	if err != nil || !permissions["wp.review"] {
		t.Fatalf("відкликання за чужим scope не має діяти: %v, %v", permissions, err)
	}
	if revoked, err := store.RevokeProjectReviewRole(ctx, adminID, projectID, bindingID); err != nil || !revoked {
		t.Fatalf("відкликання ролі: %v, %v", revoked, err)
	}
	permissions, err = store.ActiveProjectPermissions(ctx, reviewerID, projectID)
	if err != nil || permissions["wp.review"] {
		t.Fatalf("відкликана роль не повинна давати wp.review: %v, %v", permissions, err)
	}
	var auditCount int
	if err := store.Pool().QueryRow(ctx,
		`SELECT count(*) FROM core.audit_events WHERE scope_type = 'project' AND scope_id = $1
		   AND action IN ('access.grant', 'access.revoke')`, projectID).Scan(&auditCount); err != nil || auditCount != 2 {
		t.Fatalf("надання і відкликання мають бути аудійовані: %d, %v", auditCount, err)
	}
}

func TestConcurrentProjectReviewRoleGrantsDoNotDuplicate(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	hash, err := auth.HashPassword("Concurrent-Role-Pass-1")
	if err != nil {
		t.Fatal(err)
	}
	adminID, err := store.BootstrapAdministrator(ctx, "grant-admin", "Адміністратор", hash)
	if err != nil {
		t.Fatal(err)
	}
	userID, err := store.BootstrapAdministrator(ctx, "grant-reviewer", "Рецензент", hash)
	if err != nil {
		t.Fatal(err)
	}
	var projectID uuid.UUID
	if err := store.Pool().QueryRow(ctx,
		`INSERT INTO core.projects (code, name, created_by) VALUES ('ROLE-CONCURRENT', 'Конкурентна роль', $1) RETURNING id`, adminID).
		Scan(&projectID); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var workers sync.WaitGroup
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			_, err := store.GrantProjectReviewRole(ctx, adminID, userID, projectID, "project.reviewer", "Рецензія")
			results <- err
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	granted, duplicate := 0, 0
	for err := range results {
		switch {
		case err == nil:
			granted++
		case errors.Is(err, auth.ErrReviewRoleActive):
			duplicate++
		default:
			t.Fatalf("неочікувана помилка конкурентної видачі: %v", err)
		}
	}
	if granted != 1 || duplicate != 1 {
		t.Fatalf("очікували одну видачу та один конфлікт: granted=%d duplicate=%d", granted, duplicate)
	}
}

func TestLoginRejectsUnknownLoginAndWrongPassword(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	svc := auth.NewService(store)

	hash, _ := auth.HashPassword("right-password")
	if _, err := store.BootstrapAdministrator(ctx, "admin", "Адмін", hash); err != nil {
		t.Fatalf("bootstrap адміністратора: %v", err)
	}

	if _, err := svc.Login(ctx, "no-such-user", "anything", "127.0.0.1", "ua"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("невідомий логін: очікувалась ErrInvalidCredentials, отримано %v", err)
	}
	if _, err := svc.Login(ctx, "admin", "wrong-password", "127.0.0.1", "ua"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("невірний пароль: очікувалась та сама ErrInvalidCredentials, отримано %v", err)
	}
}

func TestLogoutRevokesSessionImmediately(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	svc := auth.NewService(store)

	hash, _ := auth.HashPassword("s3cret-pass-phrase")
	if _, err := store.BootstrapAdministrator(ctx, "admin", "Адмін", hash); err != nil {
		t.Fatalf("bootstrap адміністратора: %v", err)
	}

	result, err := svc.Login(ctx, "admin", "s3cret-pass-phrase", "127.0.0.1", "ua")
	if err != nil {
		t.Fatalf("вхід: %v", err)
	}

	if err := svc.Logout(ctx, result.Token); err != nil {
		t.Fatalf("вихід: %v", err)
	}

	if _, _, err := svc.Authenticate(ctx, result.Token); !errors.Is(err, auth.ErrSessionInvalid) {
		t.Errorf("після виходу сесія має бути недійсною для наступного запиту (SWR-48), отримано %v", err)
	}

	// Повторний вихід тим самим токеном — безпечний no-op.
	if err := svc.Logout(ctx, result.Token); err != nil {
		t.Errorf("повторний вихід не повинен повертати помилку: %v", err)
	}
}

func TestRevokedRoleBindingLosesPermissionOnNextCheck(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	svc := auth.NewService(store)

	hash, _ := auth.HashPassword("another-pass-phrase")
	userID, err := store.BootstrapAdministrator(ctx, "admin", "Адмін", hash)
	if err != nil {
		t.Fatalf("bootstrap адміністратора: %v", err)
	}

	result, err := svc.Login(ctx, "admin", "another-pass-phrase", "127.0.0.1", "ua")
	if err != nil {
		t.Fatalf("вхід: %v", err)
	}
	if !result.Permission["access.grant"] {
		t.Fatalf("очікувалося право access.grant одразу після bootstrap")
	}

	pool := store.Pool()
	if _, err := pool.Exec(ctx,
		`UPDATE core.role_bindings SET revoked_at = now() WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("відкликання RoleBinding: %v", err)
	}

	actor, _, err := svc.Authenticate(ctx, result.Token)
	if err != nil {
		t.Fatalf("сесія лишається дійсною після відкликання ролі: %v", err)
	}
	if actor.HasPermission("access.grant") {
		t.Error("відкликаний RoleBinding не повинен давати право на наступному запиті (SWR-48 §1)")
	}
}

func TestExpiredSessionIsRejected(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	svc := auth.NewService(store)

	hash, _ := auth.HashPassword("expiring-pass-phrase")
	if _, err := store.BootstrapAdministrator(ctx, "admin", "Адмін", hash); err != nil {
		t.Fatalf("bootstrap адміністратора: %v", err)
	}

	result, err := svc.Login(ctx, "admin", "expiring-pass-phrase", "127.0.0.1", "ua")
	if err != nil {
		t.Fatalf("вхід: %v", err)
	}

	pool := store.Pool()
	if _, err := pool.Exec(ctx,
		`UPDATE core.sessions SET expires_at = $1 WHERE token_hash = $2`,
		time.Now().Add(-time.Minute), auth.HashToken(result.Token)); err != nil {
		t.Fatalf("форсування прострочення сесії: %v", err)
	}

	if _, _, err := svc.Authenticate(ctx, result.Token); !errors.Is(err, auth.ErrSessionInvalid) {
		t.Errorf("прострочена сесія має відхилятися, отримано %v", err)
	}
}
