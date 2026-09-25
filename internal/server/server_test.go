package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testDeps(ready ReadinessCheck) Deps {
	return Deps{Ready: ready, CookieSecure: true}
}

func do(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

	return recorder
}

func TestHealthzAlwaysOK(t *testing.T) {
	handler := newRouter(testLogger(), testDeps(func(context.Context) error { return errors.New("база недоступна") }))

	if code := do(t, handler, "/healthz").Code; code != http.StatusOK {
		t.Errorf("liveness має не залежати від СУБД, отримано %d", code)
	}
}

func TestReadyzReflectsDependencies(t *testing.T) {
	ready := newRouter(testLogger(), testDeps(func(context.Context) error { return nil }))
	if code := do(t, ready, "/readyz").Code; code != http.StatusOK {
		t.Errorf("очікувався 200, отримано %d", code)
	}

	broken := newRouter(testLogger(), testDeps(func(context.Context) error { return errors.New("пінг PostgreSQL: timeout") }))
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
	handler := newRouter(testLogger(), testDeps(func(context.Context) error { return nil }))

	if do(t, handler, "/healthz").Header().Get("X-Request-Id") == "" {
		t.Error("кожна відповідь має містити ідентифікатор запиту для трасування")
	}
}

func TestSPARoutesAreServedByDELMOS(t *testing.T) {
	handler := newRouter(testLogger(), testDeps(func(context.Context) error { return nil }))
	recorder := do(t, handler, "/login")

	if recorder.Code != http.StatusOK {
		t.Fatalf("SPA route має повертати 200, отримано %d", recorder.Code)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "text/html; charset=utf-8" {
		t.Errorf("SPA route має повертати HTML, отримано %q", contentType)
	}
	if !strings.Contains(recorder.Body.String(), `<div id="app"></div>`) {
		t.Error("SPA entry point має бути вбудований у binary")
	}
}

func TestUnknownAPIRouteDoesNotFallBackToSPA(t *testing.T) {
	handler := newRouter(testLogger(), testDeps(func(context.Context) error { return nil }))
	recorder := do(t, handler, "/api/v1/not-found")

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("невідомий API route має повертати 404, отримано %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), `<div id="app"></div>`) {
		t.Error("API 404 не має повертати SPA entry point")
	}
}

func TestBootStatusEndpoint(t *testing.T) {
	deps := testDeps(func(context.Context) error { return nil })
	handler := newRouter(testLogger(), deps)
	recorder := do(t, handler, "/api/v1/system/boot-status")

	if recorder.Code != http.StatusOK {
		t.Fatalf("boot-status має повертати 200, отримано %d", recorder.Code)
	}

	var resp BootStatusResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("розбір JSON boot-status: %v", err)
	}

	if resp.Status != StatusGreen {
		t.Errorf("очікувався статус green, отримано %s", resp.Status)
	}
	if len(resp.Components) != 4 {
		t.Fatalf("очікувалося 4 компоненти, отримано %d", len(resp.Components))
	}

	for _, comp := range resp.Components {
		if comp.ID == "" || comp.Name == "" || comp.Status == "" {
			t.Errorf("компонент має мати id, name та status: %+v", comp)
		}
	}

	// Перевірка дзеркального маршруту /boot-status
	ginRecorder := do(t, handler, "/boot-status")
	if ginRecorder.Code != http.StatusOK {
		t.Fatalf("/boot-status має повертати 200, отримано %d", ginRecorder.Code)
	}
}

func TestBootStatusDegraded(t *testing.T) {
	deps := testDeps(func(context.Context) error { return errors.New("помилка PostgreSQL") })
	handler := newRouter(testLogger(), deps)
	recorder := do(t, handler, "/api/v1/system/boot-status")

	if recorder.Code != http.StatusOK {
		t.Fatalf("boot-status має повертати 200 навіть при деградації, отримано %d", recorder.Code)
	}

	var resp BootStatusResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("розбір JSON boot-status: %v", err)
	}

	if resp.Status != StatusRed {
		t.Errorf("очікувався статус red через збій готовності, отримано %s", resp.Status)
	}
}
