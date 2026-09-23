-- 0002: користувачі, ролі та сесії (SWR-42..48, ACCESS_CONTROL.md).
--
-- Мінімальний Scoped RBAC першого зрізу: лише System scope (project_id IS NULL).
-- Project/Programme scope додається разом із сутністю Project у наступній міграції.

CREATE EXTENSION IF NOT EXISTS citext; -- регістронезалежний login без ручного lower()

CREATE TABLE core.users (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    login           citext      NOT NULL UNIQUE,
    display_name    text        NOT NULL,
    password_hash   text        NOT NULL, -- Argon2id, ніколи не читається назад через API
    is_active       boolean     NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now(),
    row_version     bigint      NOT NULL DEFAULT 1
);

COMMENT ON COLUMN core.users.row_version IS 'Оптимістичне блокування для конкурентних змін профілю/пароля.';

CREATE TABLE core.role_definitions (
    key                 text        PRIMARY KEY, -- напр. 'system.administrator'
    name                text        NOT NULL,
    permission_keys     text[]      NOT NULL,
    delegation_ceiling  integer     NOT NULL DEFAULT 0, -- SWR-43 §4: без самопризначення вище стелі
    created_at          timestamptz NOT NULL DEFAULT now()
);

-- Scope першого зрізу: лише 'system'; 'programme'/'project' додаються з відповідними сутностями.
CREATE TABLE core.role_bindings (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         uuid        NOT NULL REFERENCES core.users (id) ON DELETE RESTRICT,
    role_key        text        NOT NULL REFERENCES core.role_definitions (key) ON DELETE RESTRICT,
    scope_type      text        NOT NULL CHECK (scope_type = 'system'),
    scope_id        uuid        NULL, -- NULL для scope_type='system'
    granted_by      uuid        NOT NULL REFERENCES core.users (id) ON DELETE RESTRICT,
    granted_reason  text        NOT NULL,
    granted_at      timestamptz NOT NULL DEFAULT now(),
    expires_at      timestamptz NULL,
    revoked_at      timestamptz NULL,
    revoked_by      uuid        NULL REFERENCES core.users (id) ON DELETE RESTRICT
);

CREATE INDEX role_bindings_active_by_user ON core.role_bindings (user_id) WHERE revoked_at IS NULL;

-- Каталог ролей ядра (SWR-43 §2): лише мінімальний набір дозволів першого зрізу.
INSERT INTO core.role_definitions (key, name, permission_keys, delegation_ceiling) VALUES
    ('system.administrator', 'System Administrator',
     ARRAY['identities.manage', 'access.grant', 'access.revoke'], 100)
ON CONFLICT (key) DO NOTHING;

-- Опаковий токен ніколи не зберігається — лише SHA-256 хеш (втрата БД не розкриває чинні сесії).
CREATE TABLE core.sessions (
    token_hash      bytea       PRIMARY KEY,
    user_id         uuid        NOT NULL REFERENCES core.users (id) ON DELETE RESTRICT,
    csrf_secret     bytea       NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    last_seen_at    timestamptz NOT NULL DEFAULT now(),
    expires_at      timestamptz NOT NULL,
    revoked_at      timestamptz NULL,
    ip_address      inet        NOT NULL,
    user_agent      text        NOT NULL
);

CREATE INDEX sessions_active_by_user ON core.sessions (user_id) WHERE revoked_at IS NULL;

-- append-only аудиторський доказ (ADR-010): не оновлюється й не видаляється звичайною роботою застосунку.
CREATE TABLE core.audit_events (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    occurred_at     timestamptz NOT NULL DEFAULT now(),
    actor_user_id   uuid        NULL REFERENCES core.users (id) ON DELETE RESTRICT,
    action          text        NOT NULL,
    scope_type      text        NOT NULL,
    scope_id        uuid        NULL,
    outcome         text        NOT NULL CHECK (outcome IN ('success', 'denied')),
    detail          jsonb       NOT NULL DEFAULT '{}'::jsonb,
    correlation_id  uuid        NOT NULL
);
