// Package server надає HTTP-шар DELMOS: маршрутизацію, журналювання запитів
// та діагностичні ендпоінти /healthz і /readyz.
package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"delmos/internal/config"
	"delmos/internal/version"
)

// ReadinessCheck підтверджує готовність обслуговувати трафік: доступність СУБД
// та відповідність схеми міграціям цього бінарника.
type ReadinessCheck func(ctx context.Context) error

const readinessTimeout = 5 * time.Second

type Server struct {
	http            *http.Server
	logger          *slog.Logger
	shutdownTimeout time.Duration
}

func New(cfg config.Server, logger *slog.Logger, ready ReadinessCheck) *Server {
	handler := newRouter(logger, ready)

	return &Server{
		http: &http.Server{
			Addr:              cfg.Address,
			Handler:           handler,
			ReadHeaderTimeout: cfg.ReadTimeout.Duration(),
			ReadTimeout:       cfg.ReadTimeout.Duration(),
			WriteTimeout:      cfg.WriteTimeout.Duration(),
			IdleTimeout:       cfg.IdleTimeout.Duration(),
		},
		logger:          logger,
		shutdownTimeout: cfg.ShutdownTimeout.Duration(),
	}
}

// Run обслуговує запити до скасування ctx, після чого завершує роботу коректно.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		s.logger.Info("сервер запущено", "address", s.http.Addr, "version", version.Version)
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("прослуховування %s: %w", s.http.Addr, err)
	case <-ctx.Done():
	}

	s.logger.Info("отримано сигнал завершення, припиняємо прийом нових з'єднань",
		"shutdown_timeout", s.shutdownTimeout.String())

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.shutdownTimeout)
	defer cancel()

	if err := s.http.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("коректне завершення роботи сервера: %w", err)
	}

	return nil
}

func newRouter(logger *slog.Logger, ready ReadinessCheck) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealth)
	mux.HandleFunc("GET /readyz", handleReady(logger, ready))

	return withRequestLogging(logger, withRecovery(logger, mux))
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writePlain(w, http.StatusOK, "ok "+version.Version)
}

func handleReady(logger *slog.Logger, ready ReadinessCheck) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), readinessTimeout)
		defer cancel()

		if err := ready(ctx); err != nil {
			// Причина фіксується в журналі; клієнт отримує загальний статус без деталей інфраструктури.
			logger.Error("перевірка готовності не пройдена", "error", err, "request_id", requestIDFrom(r.Context()))
			writePlain(w, http.StatusServiceUnavailable, "not ready")
			return
		}

		writePlain(w, http.StatusOK, "ready")
	}
}

func writePlain(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body + "\n"))
}

type contextKey string

const requestIDKey contextKey = "request_id"

func requestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func newRequestID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(buf)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func withRequestLogging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := newRequestID()
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)

		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		recorder.Header().Set("X-Request-Id", requestID)

		start := time.Now()
		next.ServeHTTP(recorder, r.WithContext(ctx))

		logger.Info("http-запит",
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"duration_ms", time.Since(start).Milliseconds())
	})
}

func withRecovery(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("паніка під час обробки запиту",
					"request_id", requestIDFrom(r.Context()),
					"path", r.URL.Path,
					"panic", fmt.Sprint(recovered))
				writePlain(w, http.StatusInternalServerError, "internal error")
			}
		}()

		next.ServeHTTP(w, r)
	})
}
