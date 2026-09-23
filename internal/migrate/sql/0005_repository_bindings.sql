-- 0005: прив'язка проєкту до простого Git-сховища (SPEC-02, ADR-005).
-- v0.5.0: лише provider_type='plain_git' (локальний bare або SSH/HTTPS);
-- корпоративні адаптери (GitHub/GitLab/Azure/Bitbucket/Gitea) — поза межами MVP.
CREATE TABLE core.repository_bindings (
    project_id      uuid        PRIMARY KEY REFERENCES core.projects (id) ON DELETE RESTRICT,
    provider_type   text        NOT NULL DEFAULT 'plain_git' CHECK (provider_type = 'plain_git'),
    remote_url      text        NOT NULL,
    default_branch  text        NOT NULL DEFAULT 'main',
    status          text        NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'error')),
    last_probe_at   timestamptz NULL,
    last_error      text        NULL,
    created_by      uuid        NOT NULL REFERENCES core.users (id) ON DELETE RESTRICT,
    created_at      timestamptz NOT NULL DEFAULT now()
);

-- Керування прив'язкою сховища й ручний експорт WP — дія власника проєкту (SWR-43).
UPDATE core.role_definitions
SET permission_keys = ARRAY['project.read', 'wp.read', 'wp.create', 'wp.edit', 'wp.retire', 'repository.manage']
WHERE key = 'project.owner';
