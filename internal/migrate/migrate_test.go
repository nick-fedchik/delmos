package migrate_test

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"delmos/internal/migrate"
	"delmos/internal/testsupport"
)

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	pool, err := pgxpool.New(context.Background(), testsupport.NewDatabase(t))
	if err != nil {
		t.Fatalf("підключення до тимчасової бази: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestApplyIsIdempotent(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)

	if err := migrate.Apply(ctx, pool, discardLogger()); err != nil {
		t.Fatalf("перше застосування міграцій: %v", err)
	}
	if err := migrate.Apply(ctx, pool, discardLogger()); err != nil {
		t.Fatalf("повторне застосування має бути no-op: %v", err)
	}

	if err := migrate.Verify(ctx, pool); err != nil {
		t.Fatalf("перевірка схеми після міграцій: %v", err)
	}

	var applied int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM public.schema_migrations").Scan(&applied); err != nil {
		t.Fatalf("читання історії міграцій: %v", err)
	}

	expected, err := migrate.Load()
	if err != nil {
		t.Fatalf("читання вбудованих міграцій: %v", err)
	}
	if applied != len(expected) {
		t.Errorf("застосовано %d міграцій, очікувалося %d", applied, len(expected))
	}
}

func TestApplyDetectsModifiedMigration(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)

	if err := migrate.Apply(ctx, pool, discardLogger()); err != nil {
		t.Fatalf("застосування міграцій: %v", err)
	}

	if _, err := pool.Exec(ctx, "UPDATE public.schema_migrations SET checksum = 'підроблено'"); err != nil {
		t.Fatalf("підготовка сценарію зміненої міграції: %v", err)
	}

	err := migrate.Apply(ctx, pool, discardLogger())
	if err == nil || !strings.Contains(err.Error(), "контрольна сума") {
		t.Fatalf("зміна вже застосованої міграції має блокувати старт, отримано: %v", err)
	}
	if err := migrate.Verify(ctx, pool); err == nil {
		t.Fatal("Verify має відхиляти схему з невідповідною контрольною сумою")
	}
}

func TestApplyRejectsUnknownAppliedMigration(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)

	if err := migrate.Apply(ctx, pool, discardLogger()); err != nil {
		t.Fatalf("застосування міграцій: %v", err)
	}

	_, err := pool.Exec(ctx,
		`INSERT INTO public.schema_migrations (version, name, checksum, duration_ms) VALUES ('9999', 'future', 'x', 0)`)
	if err != nil {
		t.Fatalf("підготовка сценарію застарілого бінарника: %v", err)
	}

	if err := migrate.Apply(ctx, pool, discardLogger()); err == nil {
		t.Fatal("міграція з майбутньої версії має блокувати старт застарілого бінарника")
	}
}

func TestVerifyFailsOnEmptyDatabase(t *testing.T) {
	if err := migrate.Verify(context.Background(), newPool(t)); err == nil {
		t.Fatal("порожня база не може вважатися готовою")
	}
}
