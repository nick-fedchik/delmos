package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID           uuid.UUID
	Login        string
	DisplayName  string
	PasswordHash string
	IsActive     bool
}

type Session struct {
	UserID     uuid.UUID
	Login      string
	CSRFSecret []byte
	ExpiresAt  time.Time
}

// AuthContext — розпізнаний актор чинної сесії та його активні дозволи (SWR-42 §2).
type AuthContext struct {
	UserID      uuid.UUID
	Login       string
	Permissions map[string]bool
}

func (a AuthContext) HasPermission(key string) bool {
	return a.Permissions[key]
}

// Store інкапсулює доступ до core.users/sessions/role_bindings/audit_events.
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Pool повертає базовий пул з'єднань для операцій поза межами цього пакета
// (наприклад, /readyz або перевірка стану БД у тестах).
func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}

func (s *Store) FindActiveUserByLogin(ctx context.Context, login string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx,
		`SELECT id, login, display_name, password_hash, is_active FROM core.users WHERE login = $1`,
		login,
	).Scan(&u.ID, &u.Login, &u.DisplayName, &u.PasswordHash, &u.IsActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, fmt.Errorf("пошук користувача: %w", err)
	}

	return u, nil
}

// CreateSession зберігає лише хеш токена й CSRF-секрет; сирий токен клієнту не персистується.
func (s *Store) CreateSession(ctx context.Context, userID uuid.UUID, tokenHash, csrfSecret []byte, ttl time.Duration, ip, userAgent string) (time.Time, error) {
	expiresAt := time.Now().Add(ttl)

	_, err := s.pool.Exec(ctx,
		`INSERT INTO core.sessions (token_hash, user_id, csrf_secret, expires_at, ip_address, user_agent)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		tokenHash, userID, csrfSecret, expiresAt, ip, userAgent)
	if err != nil {
		return time.Time{}, fmt.Errorf("створення сесії: %w", err)
	}

	return expiresAt, nil
}

func (s *Store) FindActiveSession(ctx context.Context, tokenHash []byte) (Session, error) {
	var session Session
	err := s.pool.QueryRow(ctx,
		`SELECT s.user_id, u.login, s.csrf_secret, s.expires_at FROM core.sessions s
		 JOIN core.users u ON u.id = s.user_id
		 WHERE s.token_hash = $1 AND s.revoked_at IS NULL AND s.expires_at > now() AND u.is_active`,
		tokenHash,
	).Scan(&session.UserID, &session.Login, &session.CSRFSecret, &session.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrSessionInvalid
	}
	if err != nil {
		return Session{}, fmt.Errorf("пошук сесії: %w", err)
	}

	return session, nil
}

func (s *Store) TouchSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.pool.Exec(ctx, `UPDATE core.sessions SET last_seen_at = now() WHERE token_hash = $1`, tokenHash)
	if err != nil {
		return fmt.Errorf("оновлення часу активності сесії: %w", err)
	}

	return nil
}

// RevokeSession видаляє чинність сесії для будь-якого наступного запиту (SWR-48 §1).
func (s *Store) RevokeSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE core.sessions SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`, tokenHash)
	if err != nil {
		return fmt.Errorf("відкликання сесії: %w", err)
	}

	return nil
}

// ActivePermissions повертає об'єднання permission_keys усіх чинних (не відкликаних,
// не прострочених) RoleBinding користувача в System scope.
func (s *Store) ActivePermissions(ctx context.Context, userID uuid.UUID) (map[string]bool, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT rd.permission_keys FROM core.role_bindings rb
		 JOIN core.role_definitions rd ON rd.key = rb.role_key
		 WHERE rb.user_id = $1 AND rb.revoked_at IS NULL
		   AND (rb.expires_at IS NULL OR rb.expires_at > now())`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("читання активних ролей: %w", err)
	}
	defer rows.Close()

	permissions := make(map[string]bool)
	for rows.Next() {
		var keys []string
		if err := rows.Scan(&keys); err != nil {
			return nil, fmt.Errorf("розбір дозволів ролі: %w", err)
		}
		for _, key := range keys {
			permissions[key] = true
		}
	}

	return permissions, rows.Err()
}

// RecordAuditEvent фіксує незмінний доказ рішення (ADR-010): аудит не залежить від outbox.
func (s *Store) RecordAuditEvent(ctx context.Context, actorUserID *uuid.UUID, action, scopeType string, scopeID *uuid.UUID, outcome string, correlationID uuid.UUID, detail map[string]any) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO core.audit_events (actor_user_id, action, scope_type, scope_id, outcome, detail, correlation_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		actorUserID, action, scopeType, scopeID, outcome, detail, correlationID)
	if err != nil {
		return fmt.Errorf("запис аудиторської події: %w", err)
	}

	return nil
}

// ActiveProjectPermissions повертає об'єднання permission_keys усіх чинних RoleBinding
// користувача саме в цьому Project scope (права одного проєкту не переходять до іншого, SWR-42 §3).
func (s *Store) ActiveProjectPermissions(ctx context.Context, userID, projectID uuid.UUID) (map[string]bool, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT rd.permission_keys FROM core.role_bindings rb
		 JOIN core.role_definitions rd ON rd.key = rb.role_key
		 WHERE rb.user_id = $1 AND rb.scope_type = 'project' AND rb.scope_id = $2
		   AND rb.revoked_at IS NULL AND (rb.expires_at IS NULL OR rb.expires_at > now())`,
		userID, projectID)
	if err != nil {
		return nil, fmt.Errorf("читання активних ролей проєкту: %w", err)
	}
	defer rows.Close()

	permissions := make(map[string]bool)
	for rows.Next() {
		var keys []string
		if err := rows.Scan(&keys); err != nil {
			return nil, fmt.Errorf("розбір дозволів ролі проєкту: %w", err)
		}
		for _, key := range keys {
			permissions[key] = true
		}
	}

	return permissions, rows.Err()
}

// RoleExists перевіряє наявність ролі в каталозі перед видачею RoleBinding.
func (s *Store) RoleExists(ctx context.Context, roleKey string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM core.role_definitions WHERE key = $1)`, roleKey).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("перевірка наявності ролі: %w", err)
	}
	return exists, nil
}

// UserExists перевіряє наявність активного користувача перед видачею RoleBinding.
func (s *Store) UserExists(ctx context.Context, userID uuid.UUID) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM core.users WHERE id = $1 AND is_active)`, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("перевірка наявності користувача: %w", err)
	}
	return exists, nil
}

// GrantSystemRole видає System-scope RoleBinding (SWR-43): грантер не може підвищити себе
// понад власную стелю (перевіряється викликаючо на рівні HTTP-шару через наявність access.grant).
func (s *Store) GrantSystemRole(ctx context.Context, granterID, userID uuid.UUID, roleKey, reason string) (uuid.UUID, error) {
	var bindingID uuid.UUID
	err := s.pool.QueryRow(ctx,
		`INSERT INTO core.role_bindings (user_id, role_key, scope_type, scope_id, granted_by, granted_reason)
		 VALUES ($1, $2, 'system', NULL, $3, $4) RETURNING id`,
		userID, roleKey, granterID, reason,
	).Scan(&bindingID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("видача System-scope ролі: %w", err)
	}

	return bindingID, nil
}

// RevokeRoleBinding відкликає чинну прив'язку; повторний виклик без чинного запису — безпечний no-op.
func (s *Store) RevokeRoleBinding(ctx context.Context, bindingID uuid.UUID) (bool, error) {
	tag, err := s.pool.Exec(ctx,
		`UPDATE core.role_bindings SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, bindingID)
	if err != nil {
		return false, fmt.Errorf("відкликання RoleBinding: %w", err)
	}

	return tag.RowsAffected() > 0, nil
}
