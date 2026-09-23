// Команда delmos запускає сервер платформи: завантажує конфігурацію,
// накочує міграції схеми та обслуговує HTTP-запити до сигналу завершення.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/time/rate"

	"delmos/internal/auth"
	"delmos/internal/config"
	"delmos/internal/logging"
	"delmos/internal/migrate"
	"delmos/internal/ratelimit"
	"delmos/internal/server"
	"delmos/internal/storage/postgres"
	"delmos/internal/version"
)

func main() {
	if err := run(); err != nil {
		slog.Error("аварійне завершення DELMOS", "error", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", config.DefaultPath, "шлях до файлу конфігурації delmos.yaml")
	showVersion := flag.Bool("version", false, "вивести версію та завершити роботу")
	migrateOnly := flag.Bool("migrate-only", false, "застосувати міграції схеми і завершити роботу")
	bootstrapAdmin := flag.String("bootstrap-admin", "",
		"створити System Administrator з цим логіном і завершити роботу (пароль з DELMOS_BOOTSTRAP_PASSWORD)")
	flag.Parse()

	if *showVersion {
		fmt.Println("delmos " + version.String())
		return nil
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	logger := logging.New(cfg.Logging.Level, cfg.Logging.Format, os.Stdout)
	logger.Info("старт DELMOS", "version", version.Version, "commit", version.Commit, "database", cfg.Database)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := applyMigrations(ctx, cfg, logger); err != nil {
		return err
	}
	if *migrateOnly {
		logger.Info("міграції застосовано, завершення за прапорцем -migrate-only")
		return nil
	}

	pool, err := postgres.NewPool(ctx, cfg.Database.AppDSN(), cfg.Database.MaxConnections)
	if err != nil {
		return err
	}
	defer pool.Close()

	authStore := auth.NewStore(pool)

	if *bootstrapAdmin != "" {
		return runBootstrapAdmin(ctx, authStore, *bootstrapAdmin)
	}

	ready := func(ctx context.Context) error {
		if err := pool.Ping(ctx); err != nil {
			return fmt.Errorf("пінг PostgreSQL: %w", err)
		}
		return migrate.Verify(ctx, pool)
	}

	deps := server.Deps{
		Ready:        ready,
		Auth:         auth.NewService(authStore),
		LoginLimiter: ratelimit.New(rate.Every(3*time.Second), 5), // 5 спроб одразу, далі 1 на 3 секунди на IP
		CookieSecure: cfg.Server.CookieSecure,
	}

	return server.New(cfg.Server, logger, deps).Run(ctx)
}

// runBootstrapAdmin створює першого System Administrator; пароль передається лише через
// змінну середовища, щоб не потрапити в аргументи процесу чи історію оболонки.
func runBootstrapAdmin(ctx context.Context, store *auth.Store, login string) error {
	password := os.Getenv("DELMOS_BOOTSTRAP_PASSWORD")
	if password == "" {
		return fmt.Errorf("задайте пароль через змінну середовища DELMOS_BOOTSTRAP_PASSWORD")
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	userID, err := store.BootstrapAdministrator(ctx, login, login, hash)
	if err != nil {
		return err
	}

	fmt.Printf("System Administrator створено: %s (%s)\n", login, userID)
	return nil
}

// applyMigrations виконується окремим пулом під роллю мігратора, який закривається одразу після DDL.
func applyMigrations(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	migratorPool, err := postgres.NewPool(ctx, cfg.Database.MigratorDSN(), 1)
	if err != nil {
		return err
	}
	defer migratorPool.Close()

	return migrate.Apply(ctx, migratorPool, logger)
}
