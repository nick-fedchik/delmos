package server

import (
	"context"
	"log/slog"
	"net/http"

	"delmos/internal/auth"
)

type authContextKey string

const authContextValueKey authContextKey = "auth_context"

func authContextFrom(ctx context.Context) (auth.AuthContext, bool) {
	actor, ok := ctx.Value(authContextValueKey).(auth.AuthContext)
	return actor, ok
}

// withSession розпізнає сесію з cookie (за наявності) і кладе AuthContext у
// контекст запиту; відсутня чи недійсна сесія не блокує запит тут — це робить
// requireAuth для конкретних маршрутів (SWR-42 §2: перевірка на кожен запит,
// а не глобальний редирект).
func withSession(authSvc *auth.Service, cookieSecure bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := sessionTokenFrom(r, cookieSecure)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		actor, csrfSecret, err := authSvc.Authenticate(r.Context(), token)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		ctx := context.WithValue(r.Context(), authContextValueKey, actor)
		ctx = context.WithValue(ctx, csrfSecretKey, csrfSecret)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type csrfSecretContextKey string

const csrfSecretKey csrfSecretContextKey = "csrf_secret"

func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authContextFrom(r.Context()); !ok {
			writeJSONError(w, http.StatusUnauthorized, "unauthenticated", "сесія відсутня або недійсна")
			return
		}
		next.ServeHTTP(w, r)
	}
}

// requireCSRF перевіряє заголовок X-CSRF-Token для мутуючих запитів (ACCESS_CONTROL.md §2).
// Застосовується після requireAuth, тому секрет сесії вже є в контексті.
func requireCSRF(logger *slog.Logger, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		secret, _ := r.Context().Value(csrfSecretKey).([]byte)

		if err := auth.CheckCSRF(secret, r.Header.Get("X-CSRF-Token")); err != nil {
			logger.Warn("невідповідність CSRF-токена", "request_id", requestIDFrom(r.Context()))
			writeJSONError(w, http.StatusForbidden, "csrf_mismatch", "невідповідність CSRF-токена")
			return
		}
		next.ServeHTTP(w, r)
	}
}
