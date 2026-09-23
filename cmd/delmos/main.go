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

	"delmos/internal/config"
	"delmos/internal/logging"
	"delmos/internal/migrate"
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

	ready := func(ctx context.Context) error {
		if err := pool.Ping(ctx); err != nil {
			return fmt.Errorf("пінг PostgreSQL: %w", err)
		}
		return migrate.Verify(ctx, pool)
	}

	return server.New(cfg.Server, logger, ready).Run(ctx)
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
