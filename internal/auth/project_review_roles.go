package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrReviewRoleInvalid       = errors.New("дозволено лише проєктні ролі рецензента й погоджувача")
	ErrReviewRoleSelfGrant     = errors.New("не можна надати собі роль незалежного рецензента чи погоджувача")
	ErrReviewRoleActive        = errors.New("особа вже має чинну роль у цьому проєкті")
	ErrReviewRoleTargetMissing = errors.New("користувач або проєкт не знайдені")
)

type ReviewRoleProject struct {
	ID   uuid.UUID `json:"id"`
	Code string    `json:"code"`
	Name string    `json:"name"`
}

type ReviewRoleUser struct {
	ID          uuid.UUID `json:"id"`
	Login       string    `json:"login"`
	DisplayName string    `json:"display_name"`
}

type ProjectReviewRoleBinding struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	Login       string    `json:"login"`
	DisplayName string    `json:"display_name"`
	RoleKey     string    `json:"role_key"`
}

func (s *Store) SearchReviewRoleProjects(ctx context.Context, query string) ([]ReviewRoleProject, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, code, name FROM core.projects
		 WHERE $1 = '' OR code ILIKE '%' || $1 || '%' OR name ILIKE '%' || $1 || '%'
		 ORDER BY code LIMIT 50`, query)
	if err != nil {
		return nil, fmt.Errorf("пошук проєктів для ролей: %w", err)
	}
	defer rows.Close()
	projects := []ReviewRoleProject{}
	for rows.Next() {
		var item ReviewRoleProject
		if err := rows.Scan(&item.ID, &item.Code, &item.Name); err != nil {
			return nil, err
		}
		projects = append(projects, item)
	}
	return projects, rows.Err()
}

func (s *Store) SearchReviewRoleUsers(ctx context.Context, query string, actorID uuid.UUID) ([]ReviewRoleUser, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, login, display_name FROM core.users
		 WHERE is_active AND id <> $2
		   AND (login::text ILIKE '%' || $1 || '%' OR display_name ILIKE '%' || $1 || '%')
		 ORDER BY login LIMIT 20`, query, actorID)
	if err != nil {
		return nil, fmt.Errorf("пошук користувачів для ролей: %w", err)
	}
	defer rows.Close()
	users := []ReviewRoleUser{}
	for rows.Next() {
		var item ReviewRoleUser
		if err := rows.Scan(&item.ID, &item.Login, &item.DisplayName); err != nil {
			return nil, err
		}
		users = append(users, item)
	}
	return users, rows.Err()
}

func (s *Store) ListProjectReviewRoleBindings(ctx context.Context, projectID uuid.UUID) ([]ProjectReviewRoleBinding, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT rb.id, u.id, u.login, u.display_name, rb.role_key
		 FROM core.role_bindings rb JOIN core.users u ON u.id = rb.user_id
		 WHERE rb.scope_type = 'project' AND rb.scope_id = $1
		   AND rb.role_key IN ('project.reviewer', 'project.approver')
		   AND rb.revoked_at IS NULL AND (rb.expires_at IS NULL OR rb.expires_at > now())
		 ORDER BY u.login, rb.role_key`, projectID)
	if err != nil {
		return nil, fmt.Errorf("читання проєктних ролей погодження: %w", err)
	}
	defer rows.Close()
	bindings := []ProjectReviewRoleBinding{}
	for rows.Next() {
		var binding ProjectReviewRoleBinding
		if err := rows.Scan(&binding.ID, &binding.UserID, &binding.Login, &binding.DisplayName, &binding.RoleKey); err != nil {
			return nil, err
		}
		bindings = append(bindings, binding)
	}
	return bindings, rows.Err()
}

func (s *Store) GrantProjectReviewRole(ctx context.Context, actorID, userID, projectID uuid.UUID, roleKey, reason string) (uuid.UUID, error) {
	if roleKey != "project.reviewer" && roleKey != "project.approver" {
		return uuid.Nil, ErrReviewRoleInvalid
	}
	if actorID == userID {
		return uuid.Nil, ErrReviewRoleSelfGrant
	}
	if strings.TrimSpace(reason) == "" {
		return uuid.Nil, errors.New("підстава надання ролі обов'язкова")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended($1::text || ':' || $2::text || ':' || $3, 0))`,
		userID, projectID, roleKey); err != nil {
		return uuid.Nil, fmt.Errorf("блокування конкурентного надання ролі: %w", err)
	}
	var targetExists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM core.users WHERE id = $1 AND is_active)
		     AND EXISTS (SELECT 1 FROM core.projects WHERE id = $2)`, userID, projectID).Scan(&targetExists); err != nil {
		return uuid.Nil, fmt.Errorf("перевірка отримувача ролі та проєкту: %w", err)
	}
	if !targetExists {
		return uuid.Nil, ErrReviewRoleTargetMissing
	}
	var bindingID uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO core.role_bindings (user_id, role_key, scope_type, scope_id, granted_by, granted_reason)
		 SELECT $1, $2, 'project', $3, $4, $5
		 WHERE NOT EXISTS (SELECT 1 FROM core.role_bindings
		     WHERE user_id = $1 AND role_key = $2 AND scope_type = 'project' AND scope_id = $3
		       AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > now()))
		 RETURNING id`, userID, roleKey, projectID, actorID, strings.TrimSpace(reason)).Scan(&bindingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrReviewRoleActive
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("надання проєктної ролі: %w", err)
	}
	correlationID := uuid.New()
	if _, err := tx.Exec(ctx,
		`INSERT INTO core.audit_events (actor_user_id, action, scope_type, scope_id, outcome, detail, correlation_id)
		 VALUES ($1, 'access.grant', 'project', $2, 'success', $3, $4)`,
		actorID, projectID, map[string]any{"role_key": roleKey, "user_id": userID.String(), "binding_id": bindingID.String()}, correlationID); err != nil {
		return uuid.Nil, fmt.Errorf("аудит надання ролі: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return bindingID, nil
}

func (s *Store) RevokeProjectReviewRole(ctx context.Context, actorID, projectID, bindingID uuid.UUID) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx,
		`UPDATE core.role_bindings SET revoked_at = now(), revoked_by = $1
		 WHERE id = $2 AND scope_type = 'project' AND scope_id = $3
		   AND role_key IN ('project.reviewer', 'project.approver') AND revoked_at IS NULL`,
		actorID, bindingID, projectID)
	if err != nil {
		return false, fmt.Errorf("відкликання проєктної ролі: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	correlationID := uuid.New()
	if _, err := tx.Exec(ctx,
		`INSERT INTO core.audit_events (actor_user_id, action, scope_type, scope_id, outcome, detail, correlation_id)
		 VALUES ($1, 'access.revoke', 'project', $2, 'success', $3, $4)`,
		actorID, projectID, map[string]any{"binding_id": bindingID.String()}, correlationID); err != nil {
		return false, fmt.Errorf("аудит відкликання ролі: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
