package auth_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

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
