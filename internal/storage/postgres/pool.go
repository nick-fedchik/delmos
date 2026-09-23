// Package postgres надає підключення до єдиного сховища DELMOS (ADR-001).
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool створює пул з'єднань і перевіряє доступність СУБД.
func NewPool(ctx context.Context, dsn string, maxConns int32) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("розбір параметрів підключення: %w", err)
	}

	poolConfig.MaxConns = maxConns
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("створення пулу з'єднань: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("перевірка з'єднання з PostgreSQL: %w", err)
	}

	return pool, nil
}
