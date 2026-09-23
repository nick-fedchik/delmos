package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// ErrLoginTaken повертається, коли логін першого адміністратора вже зайнятий.
var ErrLoginTaken = errors.New("логін уже використовується")

const systemAdministratorRole = "system.administrator"

// BootstrapAdministrator створює першого System Administrator поза звичайним API
// (SWR-43 §1: жодна роль не призначається автоматично; тут — усвідомлена дія оператора хоста).
func (s *Store) BootstrapAdministrator(ctx context.Context, login, displayName, passwordHash string) (uuid.UUID, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("початок транзакції bootstrap: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM core.users WHERE login = $1)`, login).Scan(&exists); err != nil {
		return uuid.Nil, fmt.Errorf("перевірка зайнятості логіна: %w", err)
	}
	if exists {
		return uuid.Nil, ErrLoginTaken
	}

	var userID uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO core.users (login, display_name, password_hash) VALUES ($1, $2, $3) RETURNING id`,
		login, displayName, passwordHash,
	).Scan(&userID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("створення користувача: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO core.role_bindings (user_id, role_key, scope_type, scope_id, granted_by, granted_reason)
		 VALUES ($1, $2, 'system', NULL, $1, 'bootstrap: перший System Administrator інсталяції')`,
		userID, systemAdministratorRole)
	if err != nil {
		return uuid.Nil, fmt.Errorf("прив'язка ролі System Administrator: %w", err)
	}

	correlationID := uuid.New()
	_, err = tx.Exec(ctx,
		`INSERT INTO core.audit_events (actor_user_id, action, scope_type, outcome, detail, correlation_id)
		 VALUES ($1, 'auth.bootstrap_administrator', 'system', 'success', '{}'::jsonb, $2)`,
		userID, correlationID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("запис аудиторської події bootstrap: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("фіксація bootstrap: %w", err)
	}

	return userID, nil
}
