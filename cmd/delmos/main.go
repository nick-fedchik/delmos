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

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"delmos/internal/auth"
	"delmos/internal/automation"
	"delmos/internal/config"
	"delmos/internal/economics"
	"delmos/internal/logging"
	"delmos/internal/metrics"
	"delmos/internal/migrate"
	"delmos/internal/pidfile"
	"delmos/internal/project"
	"delmos/internal/ratelimit"
	"delmos/internal/repository"
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
	pidFilePath := flag.String("pid-file", "", "шлях до pid-файлу (перевизначає server.pid_file з конфігурації)")
	flag.Parse()

	if *showVersion {
		fmt.Println("delmos " + version.String())
		return nil
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	effectivePIDFile := cfg.Server.PIDFile
	if *pidFilePath != "" {
		effectivePIDFile = *pidFilePath
	}

	logger := logging.New(cfg.Logging.Level, cfg.Logging.Format, os.Stdout)
	logger.Info("старт DELMOS", "version", version.Version, "commit", version.Commit,
		"pid", os.Getpid(), "pid_file", effectivePIDFile, "database", cfg.Database)

	if effectivePIDFile != "" {
		pf, err := pidfile.Write(effectivePIDFile)
		if err != nil {
			return err
		}
		defer func() {
			if rErr := pf.Remove(); rErr != nil {
				logger.Warn("не вдалося видалити pid-файл", "path", effectivePIDFile, "error", rErr)
			}
		}()
	}

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

	projects := project.NewStore(pool)

	// Рушій автоматизації (ADR-007, SPEC-04): диспетчер outbox і планувальник
	// у тому самому процесі, що й HTTP-сервер (монолітний бінарник).
	automationEngine := automation.NewEngine(pool, logger)
	automationEngine.SetLeaseDuration(time.Duration(cfg.Automation.LeaseSeconds) * time.Second)
	automationEngine.SetBatchSize(cfg.Automation.BatchSize)
	automationEngine.RegisterHandler("trigger.core.after_revision_committed",
		func(ctx context.Context, tx pgx.Tx, env automation.Envelope) error {
			workProductID, err := uuid.Parse(fmt.Sprint(env.Payload["work_product_id"]))
			if err != nil {
				return fmt.Errorf("некоректний work_product_id у події %s: %w", env.EventKey, err)
			}
			revisionID, err := uuid.Parse(fmt.Sprint(env.Payload["revision_id"]))
			if err != nil {
				return fmt.Errorf("некоректний revision_id у події %s: %w", env.EventKey, err)
			}
			return projects.ApplyRevisionCommittedEffects(ctx, tx, workProductID, revisionID)
		})

	// Показники обчислюються у фоновому обробнику, а не на шляху запиту
	// (SWR-22.1). Шлюз завершення фази спирається саме на ці вимірювання, тож без
	// цієї реєстрації економічно контрольовані фази не закривалися б узагалі.
	economicsStore := economics.New(pool)
	metricsStore := metrics.New(pool)
	collector := metrics.NewCollector(economicsStore)
	automationEngine.RegisterHandler("trigger.core.after_economics_changed",
		func(ctx context.Context, tx pgx.Tx, env automation.Envelope) error {
			if env.ScopeID == nil {
				return fmt.Errorf("подія %s без ідентифікатора проєкту", env.EventKey)
			}
			return collector.HandleEvent(ctx, tx, *env.ScopeID, env.CorrelationID)
		})
	go automationEngine.Run(ctx, time.Duration(cfg.Automation.PollInterval))

	ready := func(ctx context.Context) error {
		if err := pool.Ping(ctx); err != nil {
			return fmt.Errorf("пінг PostgreSQL: %w", err)
		}
		return migrate.Verify(ctx, pool)
	}

	bootCheck := func(ctx context.Context) server.BootStatusResponse {
		components := make([]server.ComponentHealth, 0, 4)
		overall := server.StatusGreen

		components = append(components, server.ComponentHealth{
			ID:      "core",
			Name:    "Ядро DELMOS",
			Status:  server.StatusGreen,
			Message: "Активне (" + version.Version + ")",
		})

		dbStatus := server.StatusGreen
		dbMsg := "Підключено"
		if err := pool.Ping(ctx); err != nil {
			dbStatus = server.StatusRed
			dbMsg = "Помилка з'єднання з PostgreSQL"
			overall = server.StatusRed
		}
		components = append(components, server.ComponentHealth{
			ID:      "database",
			Name:    "База даних (PostgreSQL)",
			Status:  dbStatus,
			Message: dbMsg,
		})

		schemaStatus := server.StatusGreen
		schemaMsg := "Схема міграцій актуальна"
		if dbStatus != server.StatusGreen {
			schemaStatus = server.StatusYellow
			schemaMsg = "Очікування бази даних"
			if overall != server.StatusRed {
				overall = server.StatusYellow
			}
		} else if err := migrate.Verify(ctx, pool); err != nil {
			schemaStatus = server.StatusRed
			schemaMsg = "Помилка перевірки схеми"
			overall = server.StatusRed
		}
		components = append(components, server.ComponentHealth{
			ID:      "schema",
			Name:    "Схема даних",
			Status:  schemaStatus,
			Message: schemaMsg,
		})

		components = append(components, server.ComponentHealth{
			ID:      "git_storage",
			Name:    "Сховище репозиторіїв",
			Status:  server.StatusGreen,
			Message: "Plain Git готовий",
		})

		return server.BootStatusResponse{
			Status:     overall,
			Version:    version.Version,
			Components: components,
		}
	}

	deps := server.Deps{
		Ready:        ready,
		Auth:         auth.NewService(authStore),
		Projects:     projects,
		Repositories: repository.NewStore(pool, repository.NewPlainGitProvider()),
		Automation:   automationEngine,
		Economics:    economicsStore,
		Metrics:      metricsStore,
		LoginLimiter: ratelimit.New(rate.Every(3*time.Second), 5), // 5 спроб одразу, далі 1 на 3 секунди на IP
		CookieSecure: cfg.Server.CookieSecure,
		BootStatus:   bootCheck,
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
