package server

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"delmos/internal/auth"
	"delmos/internal/ratelimit"
)

const (
	sessionCookieName       = "delmos_session"
	sessionCookieNameSecure = "__Host-delmos_session"
	maxLoginBodyBytes       = 4 * 1024
)

func sessionCookieNameFor(secure bool) string {
	if secure {
		return sessionCookieNameSecure
	}
	return sessionCookieName
}

func newSessionCookie(secure bool, token string, expiresAt time.Time) *http.Cookie {
	//nolint:gosec // G124: Secure є параметризованим свідомо (true в production, false лише для локальної розробки без TLS)
	cookie := &http.Cookie{
		Name:     sessionCookieNameFor(secure),
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		Expires:  expiresAt,
	}
	// __Host- вимагає відсутності атрибута Domain — Path=/ і Secure вже задані вище.
	return cookie
}

func expiredSessionCookie(secure bool) *http.Cookie {
	//nolint:gosec // G124: Secure є параметризованим свідомо, див. newSessionCookie
	return &http.Cookie{
		Name:     sessionCookieNameFor(secure),
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	}
}

func sessionTokenFrom(r *http.Request, secure bool) (string, bool) {
	cookie, err := r.Cookie(sessionCookieNameFor(secure))
	if err != nil || cookie.Value == "" {
		return "", false
	}
	return cookie.Value, true
}

// clientIP довіряє X-Forwarded-For лише тому, що DELMOS обов'язково працює за
// реверс-проксі на loopback-інтерфейсі (SYSTEM_REQUIREMENTS.md §6.1); пряма
// публікація порту застосунку назовні заборонена окремо цією ж вимогою.
func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		if first, _, ok := strings.Cut(forwarded, ","); ok {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(forwarded)
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

type loginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type loginResponse struct {
	CSRFToken string    `json:"csrf_token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      userView  `json:"user"`
}

type userView struct {
	ID          string   `json:"id"`
	Login       string   `json:"login"`
	DisplayName string   `json:"display_name"`
	Permissions []string `json:"permissions"`
}

func handleLogin(logger *slog.Logger, authSvc *auth.Service, limiter *ratelimit.Limiter, cookieSecure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow(clientIP(r)) {
			writeJSONError(w, http.StatusTooManyRequests, "rate_limited", auth.ErrRateLimited.Error())
			return
		}

		var req loginRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBodyBytes)).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_body", "некоректне тіло запиту")
			return
		}
		if req.Login == "" || req.Password == "" {
			writeJSONError(w, http.StatusBadRequest, "invalid_body", "login і password обов'язкові")
			return
		}

		result, err := authSvc.Login(r.Context(), req.Login, req.Password, clientIP(r), r.UserAgent())
		if err != nil {
			status := http.StatusUnauthorized
			if errors.Is(err, auth.ErrAccountInactive) {
				status = http.StatusForbidden
			}
			logger.Warn("невдала спроба входу", "request_id", requestIDFrom(r.Context()), "reason", err.Error())
			writeJSONError(w, status, "invalid_credentials", auth.ErrInvalidCredentials.Error())
			return
		}

		http.SetCookie(w, newSessionCookie(cookieSecure, result.Token, result.ExpiresAt))
		writeJSON(w, http.StatusOK, loginResponse{
			CSRFToken: result.CSRFToken,
			ExpiresAt: result.ExpiresAt,
			User:      toUserView(result.User.ID.String(), result.User.Login, result.User.DisplayName, result.Permission),
		})
	}
}

func handleLogout(authSvc *auth.Service, cookieSecure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := sessionTokenFrom(r, cookieSecure)
		if ok {
			_ = authSvc.Logout(r.Context(), token) // повторний виклик без сесії — безпечний no-op
		}
		http.SetCookie(w, expiredSessionCookie(cookieSecure))
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleCurrentSession() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authContextFrom(r.Context())
		if !ok {
			writeJSONError(w, http.StatusUnauthorized, "unauthenticated", "сесія відсутня")
			return
		}

		if secret, ok := r.Context().Value(csrfSecretKey).([]byte); ok && len(secret) > 0 {
			w.Header().Set("X-CSRF-Token", base64.RawURLEncoding.EncodeToString(secret))
		}

		permissions := make([]string, 0, len(actor.Permissions))
		for key := range actor.Permissions {
			permissions = append(permissions, key)
		}

		writeJSON(w, http.StatusOK, userView{ID: actor.UserID.String(), Login: actor.Login, Permissions: permissions})
	}
}

func toUserView(id, login, displayName string, permissions map[string]bool) userView {
	keys := make([]string, 0, len(permissions))
	for key := range permissions {
		keys = append(keys, key)
	}
	return userView{ID: id, Login: login, DisplayName: displayName, Permissions: keys}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	body := errorBody{}
	body.Error.Code = code
	body.Error.Message = message
	writeJSON(w, status, body)
}
