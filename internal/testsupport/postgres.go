// Package testsupport готує ізольовану базу даних на кожен інтеграційний тест
// (docs/testing/TEST_STRATEGY.md): тест ніколи не працює в спільній схемі.
package testsupport

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

const (
	dsnEnv      = "DELMOS_TEST_DSN"
	templateEnv = "DELMOS_TEST_TEMPLATE"
)

// NewDatabase створює тимчасову базу даних і повертає рядок підключення до неї.
// Без DELMOS_TEST_DSN тест пропускається, щоб `go test ./...` працював без СУБД.
func NewDatabase(t *testing.T) string {
	t.Helper()

	adminDSN, ok := os.LookupEnv(dsnEnv)
	if !ok {
		t.Skipf("інтеграційний тест пропущено: не задано %s (див. make db-test-setup)", dsnEnv)
	}

	ctx := context.Background()
	adminConn, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		t.Fatalf("підключення до %s: %v", dsnEnv, err)
	}
	defer func() { _ = adminConn.Close(ctx) }()

	name := "delmos_test_" + randomSuffix(t)
	create := fmt.Sprintf("CREATE DATABASE %s", pgx.Identifier{name}.Sanitize())
	if template := os.Getenv(templateEnv); template != "" {
		create += " TEMPLATE " + pgx.Identifier{template}.Sanitize()
	}

	if _, err := adminConn.Exec(ctx, create); err != nil {
		t.Fatalf("створення тимчасової бази %s: %v", name, err)
	}

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		conn, err := pgx.Connect(cleanupCtx, adminDSN)
		if err != nil {
			t.Errorf("підключення для видалення бази %s: %v", name, err)
			return
		}
		defer func() { _ = conn.Close(cleanupCtx) }()

		drop := fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", pgx.Identifier{name}.Sanitize())
		if _, err := conn.Exec(cleanupCtx, drop); err != nil {
			t.Errorf("видалення тимчасової бази %s: %v", name, err)
		}
	})

	return replaceDatabase(t, adminDSN, name)
}

func replaceDatabase(t *testing.T, dsn, name string) string {
	t.Helper()

	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("%s має бути URL вигляду postgres://user@host/db: %v", dsnEnv, err)
	}

	parsed.Path = "/" + name
	return parsed.String()
}

func randomSuffix(t *testing.T) string {
	t.Helper()

	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("генерація імені тимчасової бази: %v", err)
	}

	return hex.EncodeToString(buf)
}
