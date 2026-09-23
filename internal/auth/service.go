package auth

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SessionTTL — тривалість життя сесії до примусового повторного входу.
const SessionTTL = 12 * time.Hour

type Service struct {
	store *Store
}

func NewService(store *Store) *Service {
	return &Service{store: store}
}

type LoginResult struct {
	Token      string // сирий токен сесії — кладеться в HttpOnly cookie, ніде більше не зберігається
	CSRFToken  string // повертається лише в тілі відповіді, не в cookie (SWR-44 §2, ACCESS_CONTROL §2)
	User       User
	ExpiresAt  time.Time
	Permission map[string]bool
}

// Login перевіряє облікові дані та створює нову сесію. Помилка не розрізняє
// "немає такого логіна" і "невірний пароль" (SWR-44 §2).
func (s *Service) Login(ctx context.Context, login, password, ip, userAgent string) (LoginResult, error) {
	correlationID := uuid.New()

	user, err := s.store.FindActiveUserByLogin(ctx, login)
	if err != nil {
		_ = s.store.RecordAuditEvent(ctx, nil, "auth.login", "system", nil, "denied", correlationID, map[string]any{"login": login})
		return LoginResult{}, err
	}

	if !user.IsActive {
		_ = s.store.RecordAuditEvent(ctx, &user.ID, "auth.login", "system", nil, "denied", correlationID, map[string]any{"reason": "inactive"})
		return LoginResult{}, ErrAccountInactive
	}

	valid, err := VerifyPassword(user.PasswordHash, password)
	if err != nil {
		return LoginResult{}, fmt.Errorf("перевірка пароля: %w", err)
	}
	if !valid {
		_ = s.store.RecordAuditEvent(ctx, &user.ID, "auth.login", "system", nil, "denied", correlationID, map[string]any{"reason": "bad_password"})
		return LoginResult{}, ErrInvalidCredentials
	}

	rawToken, tokenHash, err := SecureToken()
	if err != nil {
		return LoginResult{}, err
	}

	rawCSRF, csrfSecret, err := RandomSecret()
	if err != nil {
		return LoginResult{}, err
	}

	expiresAt, err := s.store.CreateSession(ctx, user.ID, tokenHash, csrfSecret, SessionTTL, ip, userAgent)
	if err != nil {
		return LoginResult{}, err
	}

	permissions, err := s.store.ActivePermissions(ctx, user.ID)
	if err != nil {
		return LoginResult{}, err
	}

	if err := s.store.RecordAuditEvent(ctx, &user.ID, "auth.login", "system", nil, "success", correlationID, map[string]any{}); err != nil {
		return LoginResult{}, err
	}

	return LoginResult{
		Token:      rawToken,
		CSRFToken:  rawCSRF,
		User:       user,
		ExpiresAt:  expiresAt,
		Permission: permissions,
	}, nil
}

// Authenticate перевіряє сесійний токен, отриманий із cookie, і повертає
// доступні дозволи актора. Прострочена/відкликана сесія завжди повертає ErrSessionInvalid.
func (s *Service) Authenticate(ctx context.Context, rawToken string) (AuthContext, []byte, error) {
	session, err := s.store.FindActiveSession(ctx, HashToken(rawToken))
	if err != nil {
		return AuthContext{}, nil, err
	}

	permissions, err := s.store.ActivePermissions(ctx, session.UserID)
	if err != nil {
		return AuthContext{}, nil, err
	}

	if err := s.store.TouchSession(ctx, HashToken(rawToken)); err != nil {
		return AuthContext{}, nil, err
	}

	return AuthContext{UserID: session.UserID, Login: session.Login, Permissions: permissions}, session.CSRFSecret, nil
}

// Logout відкликає сесію; повторний виклик того самого токена є безпечним no-op.
func (s *Service) Logout(ctx context.Context, rawToken string) error {
	return s.store.RevokeSession(ctx, HashToken(rawToken))
}

// ActiveProjectPermissions повертає дозволи актора в конкретному Project scope
// (SWR-42 §3): використовується обробниками поза межами середовища сесії.
func (s *Service) ActiveProjectPermissions(ctx context.Context, userID, projectID uuid.UUID) (map[string]bool, error) {
	return s.store.ActiveProjectPermissions(ctx, userID, projectID)
}

// GrantSystemRole видає System-scope RoleBinding; повертає ErrRoleUnknown чи
// ErrUserUnknown, якщо роль або отримувач не існують (SWR-43).
func (s *Service) GrantSystemRole(ctx context.Context, granterID, userID uuid.UUID, roleKey, reason string) (uuid.UUID, error) {
	roleOK, err := s.store.RoleExists(ctx, roleKey)
	if err != nil {
		return uuid.Nil, err
	}
	if !roleOK {
		return uuid.Nil, ErrRoleUnknown
	}

	userOK, err := s.store.UserExists(ctx, userID)
	if err != nil {
		return uuid.Nil, err
	}
	if !userOK {
		return uuid.Nil, ErrUserUnknown
	}

	bindingID, err := s.store.GrantSystemRole(ctx, granterID, userID, roleKey, reason)
	if err != nil {
		return uuid.Nil, err
	}

	correlationID := uuid.New()
	if err := s.store.RecordAuditEvent(ctx, &granterID, "access.grant", "system", nil, "success", correlationID,
		map[string]any{"role_key": roleKey, "user_id": userID.String()}); err != nil {
		return uuid.Nil, err
	}

	return bindingID, nil
}

// RevokeRoleBinding відкликає RoleBinding; false означає, що прив'язку вже було
// відкликано раніше (безпечний no-op, а не помилка).
func (s *Service) RevokeRoleBinding(ctx context.Context, revokerID, bindingID uuid.UUID) (bool, error) {
	revoked, err := s.store.RevokeRoleBinding(ctx, bindingID)
	if err != nil {
		return false, err
	}
	if !revoked {
		return false, nil
	}

	correlationID := uuid.New()
	if err := s.store.RecordAuditEvent(ctx, &revokerID, "access.revoke", "system", nil, "success", correlationID,
		map[string]any{"binding_id": bindingID.String()}); err != nil {
		return false, err
	}

	return true, nil
}

// CheckCSRF звіряє заголовок X-CSRF-Token із секретом сесії в сталий за часом спосіб.
func CheckCSRF(sessionSecret []byte, headerValue string) error {
	provided, err := base64.RawURLEncoding.DecodeString(headerValue)
	if err != nil || len(provided) == 0 {
		return ErrCSRFMismatch
	}

	if subtle.ConstantTimeCompare(sessionSecret, provided) != 1 {
		return ErrCSRFMismatch
	}

	return nil
}
