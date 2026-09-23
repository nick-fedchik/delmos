package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func do(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

	return recorder
}

func TestHealthzAlwaysOK(t *testing.T) {
	handler := newRouter(testLogger(), func(context.Context) error { return errors.New("база недоступна") })

	if code := do(t, handler, "/healthz").Code; code != http.StatusOK {
		t.Errorf("liveness має не залежати від СУБД, отримано %d", code)
	}
}

func TestReadyzReflectsDependencies(t *testing.T) {
	ready := newRouter(testLogger(), func(context.Context) error { return nil })
	if code := do(t, ready, "/readyz").Code; code != http.StatusOK {
		t.Errorf("очікувався 200, отримано %d", code)
	}

	broken := newRouter(testLogger(), func(context.Context) error { return errors.New("пінг PostgreSQL: timeout") })
	recorder := do(t, broken, "/readyz")

	if recorder.Code != http.StatusServiceUnavailable {
		t.Errorf("очікувався 503, отримано %d", recorder.Code)
	}
	if body := recorder.Body.String(); body != "not ready\n" {
		t.Errorf("тіло відповіді не повинно розкривати деталі інфраструктури: %q", body)
	}
}

func TestPanicIsContained(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /boom", func(http.ResponseWriter, *http.Request) { panic("збій обробника") })
	handler := withRequestLogging(testLogger(), withRecovery(testLogger(), mux))

	if code := do(t, handler, "/boom").Code; code != http.StatusInternalServerError {
		t.Errorf("паніка має перетворюватися на 500, отримано %d", code)
	}
}

func TestRequestIDHeaderIsSet(t *testing.T) {
	handler := newRouter(testLogger(), func(context.Context) error { return nil })

	if do(t, handler, "/healthz").Header().Get("X-Request-Id") == "" {
		t.Error("кожна відповідь має містити ідентифікатор запиту для трасування")
	}
}
