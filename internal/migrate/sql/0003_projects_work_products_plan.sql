-- 0003: проєкт, ядрові Work Products/ревізії та обов'язковий Generic Project
-- Plan (PLAN-001), розширення RoleBinding на Project scope (PROJECT_MODEL.md,
-- WORK_PRODUCTS.md, ACCESS_CONTROL.md).

CREATE TABLE core.projects (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    code            citext      NOT NULL UNIQUE,
    name            text        NOT NULL,
    description     text        NOT NULL DEFAULT '',
    status          text        NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    created_by      uuid        NOT NULL REFERENCES core.users (id) ON DELETE RESTRICT,
    created_at      timestamptz NOT NULL DEFAULT now(),
    archived_at     timestamptz NULL,
    row_version     bigint      NOT NULL DEFAULT 1
);

-- Шість базових типів артефактів ядра (WORK_PRODUCTS.md §2).
CREATE TABLE core.work_products (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      uuid        NOT NULL REFERENCES core.projects (id) ON DELETE RESTRICT,
    code            text        NOT NULL,
    type            text        NOT NULL CHECK (type IN ('plan', 'requirement', 'architecture', 'test_spec', 'report', 'record')),
    profile         text        NOT NULL,
    title           text        NOT NULL,
    status          text        NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'in_review', 'approved', 'obsolete')),
    classification  text        NOT NULL DEFAULT 'internal' CHECK (classification IN ('internal', 'restricted')),
    created_at      timestamptz NOT NULL DEFAULT now(),
    row_version     bigint      NOT NULL DEFAULT 1,
    UNIQUE (project_id, code)
);

-- Незмінні ревізії: погодження й аудит завжди посилаються на точний revision_id/payload_hash.
CREATE TABLE core.work_product_revisions (
    id                  uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    work_product_id     uuid        NOT NULL REFERENCES core.work_products (id) ON DELETE RESTRICT,
    revision_number     integer     NOT NULL,
    body                text        NOT NULL DEFAULT '',
    metadata            jsonb       NOT NULL DEFAULT '{}'::jsonb,
    payload_hash        bytea       NOT NULL,
    content_hash        bytea       NOT NULL,
    created_by          uuid        NOT NULL REFERENCES core.users (id) ON DELETE RESTRICT,
    created_at          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (work_product_id, revision_number)
);

-- Кореневий інваріант проєкту (PROJECT_MODEL.md §1): нерозривний зв'язок з PLAN-001.
-- effective_plan_revision_id лишається NULL до окремої майбутньої команди plan.apply.
CREATE TABLE core.project_plan_bindings (
    project_id                  uuid    PRIMARY KEY REFERENCES core.projects (id) ON DELETE RESTRICT,
    plan_work_product_id        uuid    NOT NULL REFERENCES core.work_products (id) ON DELETE RESTRICT,
    effective_plan_revision_id  uuid    NULL REFERENCES core.work_product_revisions (id) ON DELETE RESTRICT,
    config_generation           bigint  NOT NULL DEFAULT 0
);

-- Розширення RoleBinding на Project scope (ACCESS_CONTROL.md §3). scope_id навмисно без
-- FK: простір може в майбутньому охоплювати й інші кореневі сутності (Programme).
ALTER TABLE core.role_bindings DROP CONSTRAINT role_bindings_scope_type_check;
ALTER TABLE core.role_bindings ADD CONSTRAINT role_bindings_scope_type_check
    CHECK (
        (scope_type = 'system' AND scope_id IS NULL) OR
        (scope_type = 'project' AND scope_id IS NOT NULL)
    );

CREATE INDEX role_bindings_active_by_scope ON core.role_bindings (scope_type, scope_id) WHERE revoked_at IS NULL;

-- Каталог ролей: створення проєкту (System scope) і власник щойно створеного проєкту (Project scope).
INSERT INTO core.role_definitions (key, name, permission_keys, delegation_ceiling) VALUES
    ('project.manager', 'Project Manager', ARRAY['scopes.manage'], 10),
    ('project.owner', 'Project Owner', ARRAY['project.read'], 0)
ON CONFLICT (key) DO NOTHING;
