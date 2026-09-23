package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"delmos/internal/auth"
	"delmos/internal/migrate"
	"delmos/internal/ratelimit"
	"delmos/internal/testsupport"

	"github.com/jackc/pgx/v5/pgxpool"
)

func newAuthTestRouter(t *testing.T) (http.Handler, *auth.Store) {
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
		LoginLimiter: ratelimit.New(rate.Every(time.Millisecond), 1000), // не заважає функціональним тестам
		CookieSecure: true,
	}

	return newRouter(testLogger(), deps), authStore
}

func bootstrapTestAdmin(t *testing.T, store *auth.Store, login, password string) {
	t.Helper()

	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("хешування пароля: %v", err)
	}
	if _, err := store.BootstrapAdministrator(context.Background(), login, login, hash); err != nil {
		t.Fatalf("bootstrap адміністратора: %v", err)
	}
}

func doJSON(t *testing.T, handler http.Handler, method, path string, body any, cookie *http.Cookie, csrfToken string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("кодування тіла запиту: %v", err)
		}
		reader = bytes.NewReader(encoded)
	}

	req := httptest.NewRequest(method, path, reader)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if csrfToken != "" {
		req.Header.Set("X-CSRF-Token", csrfToken)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	return recorder
}

func sessionCookieFromResponse(t *testing.T, recorder *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()

	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == sessionCookieNameSecure {
			return cookie
		}
	}

	t.Fatal("відповідь на вхід має встановлювати сесійну cookie")
	return nil
}

func TestLoginLogoutSessionFlow(t *testing.T) {
	handler, store := newAuthTestRouter(t)
	bootstrapTestAdmin(t, store, "admin", "Correct-Horse-Battery-Staple")

	login := doJSON(t, handler, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"login": "admin", "password": "Correct-Horse-Battery-Staple"}, nil, "")
	if login.Code != http.StatusOK {
		t.Fatalf("очікувався 200 при вході, отримано %d: %s", login.Code, login.Body.String())
	}

	var loginBody loginResponse
	if err := json.Unmarshal(login.Body.Bytes(), &loginBody); err != nil {
		t.Fatalf("розбір тіла відповіді входу: %v", err)
	}
	if loginBody.CSRFToken == "" {
		t.Fatal("тіло відповіді має містити csrf_token")
	}

	cookie := sessionCookieFromResponse(t, login)

	session := doJSON(t, handler, http.MethodGet, "/api/v1/auth/session", nil, cookie, "")
	if session.Code != http.StatusOK {
		t.Fatalf("очікувався 200 для чинної сесії, отримано %d", session.Code)
	}

	withoutCSRF := doJSON(t, handler, http.MethodPost, "/api/v1/auth/logout", nil, cookie, "")
	if withoutCSRF.Code != http.StatusForbidden {
		t.Errorf("вихід без CSRF-заголовка має повертати 403, отримано %d", withoutCSRF.Code)
	}

	logout := doJSON(t, handler, http.MethodPost, "/api/v1/auth/logout", nil, cookie, loginBody.CSRFToken)
	if logout.Code != http.StatusNoContent {
		t.Fatalf("очікувався 204 при виході, отримано %d", logout.Code)
	}

	afterLogout := doJSON(t, handler, http.MethodGet, "/api/v1/auth/session", nil, cookie, "")
	if afterLogout.Code != http.StatusUnauthorized {
		t.Errorf("після виходу сесія має бути недійсною (SWR-48), отримано %d", afterLogout.Code)
	}
}

func TestLoginRejectsWrongPasswordWithGenericMessage(t *testing.T) {
	handler, store := newAuthTestRouter(t)
	bootstrapTestAdmin(t, store, "admin", "right-password-phrase")

	recorder := doJSON(t, handler, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"login": "admin", "password": "wrong-password"}, nil, "")

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("очікувався 401, отримано %d", recorder.Code)
	}
	if bytes.Contains(recorder.Body.Bytes(), []byte("admin")) {
		t.Error("повідомлення про помилку не повинно розкривати, що логін існує")
	}
}

func TestSessionEndpointRequiresAuthentication(t *testing.T) {
	handler, _ := newAuthTestRouter(t)

	recorder := doJSON(t, handler, http.MethodGet, "/api/v1/auth/session", nil, nil, "")
	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("без сесії очікувався 401, отримано %d", recorder.Code)
	}
}

func TestLoginRateLimitBlocksExcessiveAttempts(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), testsupport.NewDatabase(t))
	if err != nil {
		t.Fatalf("підключення до тимчасової бази: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := migrate.Apply(context.Background(), pool, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatalf("застосування міграцій: %v", err)
	}

	authStore := auth.NewStore(pool)
	deps := Deps{
		Ready:        func(context.Context) error { return nil },
		Auth:         auth.NewService(authStore),
		LoginLimiter: ratelimit.New(rate.Every(time.Hour), 2),
		CookieSecure: true,
	}
	handler := newRouter(testLogger(), deps)

	for i := 0; i < 2; i++ {
		recorder := doJSON(t, handler, http.MethodPost, "/api/v1/auth/login",
			map[string]string{"login": "nobody", "password": "x"}, nil, "")
		if recorder.Code == http.StatusTooManyRequests {
			t.Fatalf("спроба %d не мала ще досягти ліміту", i+1)
		}
	}

	limited := doJSON(t, handler, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"login": "nobody", "password": "x"}, nil, "")
	if limited.Code != http.StatusTooManyRequests {
		t.Errorf("очікувався 429 після вичерпання burst, отримано %d", limited.Code)
	}
}
