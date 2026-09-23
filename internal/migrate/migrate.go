// Package migrate застосовує forward-only міграції схеми PostgreSQL.
//
// Міграції вбудовані в бінарник, серіалізуються сесійним advisory lock і
// захищені контрольними сумами SHA-256 (docs/requirements/SYSTEM_REQUIREMENTS.md, §3.4).
package migrate

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed sql/*.sql
var migrationsFS embed.FS

// AdvisoryLockKey — фіксований ключ блокування міграцій (SYSTEM_REQUIREMENTS §3.4).
const AdvisoryLockKey int64 = 0x7108ecb72639

const createHistoryTable = `
CREATE TABLE IF NOT EXISTS public.schema_migrations (
    version     text        PRIMARY KEY,
    name        text        NOT NULL,
    checksum    text        NOT NULL,
    applied_at  timestamptz NOT NULL DEFAULT now(),
    applied_by  text        NOT NULL DEFAULT current_user,
    duration_ms integer     NOT NULL
)`

type Migration struct {
	Version  string
	Name     string
	Checksum string
	SQL      string
}

// Querier охоплює і пул, і окреме з'єднання pgx.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Load читає вбудовані міграції у порядку версій.
func Load() ([]Migration, error) {
	entries, err := fs.ReadDir(migrationsFS, "sql")
	if err != nil {
		return nil, fmt.Errorf("читання вбудованих міграцій: %w", err)
	}

	migrations := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		content, err := migrationsFS.ReadFile("sql/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("читання %s: %w", entry.Name(), err)
		}

		base := strings.TrimSuffix(entry.Name(), ".sql")
		version, name, found := strings.Cut(base, "_")
		if !found {
			return nil, fmt.Errorf("ім'я міграції %s не відповідає формату NNNN_опис.sql", entry.Name())
		}

		sum := sha256.Sum256(content)
		migrations = append(migrations, Migration{
			Version:  version,
			Name:     name,
			Checksum: hex.EncodeToString(sum[:]),
			SQL:      string(content),
		})
	}

	sort.Slice(migrations, func(i, j int) bool { return migrations[i].Version < migrations[j].Version })

	for i := 1; i < len(migrations); i++ {
		if migrations[i].Version == migrations[i-1].Version {
			return nil, fmt.Errorf("дубльована версія міграції %s", migrations[i].Version)
		}
	}

	return migrations, nil
}

// Apply накочує всі незастосовані міграції під роллю мігратора.
func Apply(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) error {
	migrations, err := Load()
	if err != nil {
		return err
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("отримання з'єднання для міграцій: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", AdvisoryLockKey); err != nil {
		return fmt.Errorf("захоплення advisory lock міграцій: %w", err)
	}
	defer func() {
		if _, err := conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", AdvisoryLockKey); err != nil {
			logger.Error("не вдалося звільнити advisory lock міграцій", "error", err)
		}
	}()

	if _, err := conn.Exec(ctx, createHistoryTable); err != nil {
		return fmt.Errorf("створення таблиці schema_migrations: %w", err)
	}

	applied, err := appliedChecksums(ctx, conn)
	if err != nil {
		return err
	}

	known := make(map[string]bool, len(migrations))
	for _, migration := range migrations {
		known[migration.Version] = true
	}
	for version := range applied {
		if !known[version] {
			return fmt.Errorf("у базі застосовано міграцію %s, відсутню в бінарнику: запущено застарілу версію DELMOS", version)
		}
	}

	for _, migration := range migrations {
		checksum, ok := applied[migration.Version]
		if ok {
			if checksum != migration.Checksum {
				return fmt.Errorf("контрольна сума міграції %s не збігається: файл змінено після застосування", migration.Version)
			}
			continue
		}

		start := time.Now()
		if err := applyOne(ctx, conn, migration, start); err != nil {
			return err
		}
		logger.Info("міграцію застосовано",
			"version", migration.Version,
			"name", migration.Name,
			"duration_ms", time.Since(start).Milliseconds())
	}

	return nil
}

func applyOne(ctx context.Context, conn *pgxpool.Conn, migration Migration, start time.Time) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("початок транзакції міграції %s: %w", migration.Version, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, migration.SQL); err != nil {
		return fmt.Errorf("виконання міграції %s: %w", migration.Version, err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO public.schema_migrations (version, name, checksum, duration_ms) VALUES ($1, $2, $3, $4)`,
		migration.Version, migration.Name, migration.Checksum, time.Since(start).Milliseconds())
	if err != nil {
		return fmt.Errorf("запис історії міграції %s: %w", migration.Version, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("фіксація міграції %s: %w", migration.Version, err)
	}

	return nil
}

// Verify перевіряє, що схема бази відповідає міграціям цього бінарника (використовується в /readyz).
func Verify(ctx context.Context, q Querier) error {
	migrations, err := Load()
	if err != nil {
		return err
	}

	applied, err := appliedChecksums(ctx, q)
	if err != nil {
		return err
	}

	for _, migration := range migrations {
		checksum, ok := applied[migration.Version]
		if !ok {
			return fmt.Errorf("міграцію %s не застосовано", migration.Version)
		}
		if checksum != migration.Checksum {
			return fmt.Errorf("контрольна сума міграції %s не збігається зі схемою бази", migration.Version)
		}
	}

	return nil
}

func appliedChecksums(ctx context.Context, q Querier) (map[string]string, error) {
	rows, err := q.Query(ctx, `SELECT version, checksum FROM public.schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("читання історії міграцій: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]string)
	for rows.Next() {
		var version, checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			return nil, fmt.Errorf("розбір історії міграцій: %w", err)
		}
		applied[version] = checksum
	}

	return applied, rows.Err()
}
