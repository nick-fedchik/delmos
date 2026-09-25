-- 0011: plan.apply pipeline, execution projections (phases, milestones)
-- та оперативні реєстри (stakeholders, risks) за CORE-CONTRACT-002/003.

CREATE TABLE core.project_plan_applications (
    id                  uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id          uuid        NOT NULL REFERENCES core.projects (id) ON DELETE RESTRICT,
    plan_revision_id    uuid        NOT NULL REFERENCES core.work_product_revisions (id) ON DELETE RESTRICT,
    config_generation   bigint      NOT NULL,
    applied_by          uuid        NOT NULL REFERENCES core.users (id) ON DELETE RESTRICT,
    applied_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE core.project_phases (
    id                  uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id          uuid        NOT NULL REFERENCES core.projects (id) ON DELETE CASCADE,
    phase_key           text        NOT NULL,
    name                text        NOT NULL,
    planned_start       date        NULL,
    planned_finish      date        NULL,
    status              text        NOT NULL DEFAULT 'not_started' CHECK (status IN ('not_started', 'active', 'completed')),
    depends_on          text[]      NOT NULL DEFAULT '{}',
    config_generation   bigint      NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, phase_key)
);

CREATE TABLE core.project_milestones (
    id                  uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id          uuid        NOT NULL REFERENCES core.projects (id) ON DELETE CASCADE,
    milestone_key       text        NOT NULL,
    phase_key           text        NOT NULL,
    name                text        NOT NULL,
    target_date         date        NULL,
    status              text        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'passed', 'failed', 'waived')),
    deliverable_keys    text[]      NOT NULL DEFAULT '{}',
    acceptance_rule_keys text[]     NOT NULL DEFAULT '{}',
    config_generation   bigint      NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, milestone_key)
);

CREATE TABLE core.project_stakeholders (
    id                  uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id          uuid        NOT NULL REFERENCES core.projects (id) ON DELETE CASCADE,
    kind                text        NOT NULL CHECK (kind IN ('user', 'organization', 'external_party')),
    name                text        NOT NULL,
    contact_ref         text        NOT NULL DEFAULT '',
    interest            text        NOT NULL DEFAULT '',
    created_by          uuid        NOT NULL REFERENCES core.users (id) ON DELETE RESTRICT,
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE core.project_risks (
    id                  uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id          uuid        NOT NULL REFERENCES core.projects (id) ON DELETE CASCADE,
    title               text        NOT NULL,
    description         text        NOT NULL DEFAULT '',
    status              text        NOT NULL DEFAULT 'identified' CHECK (status IN ('identified', 'analyzed', 'mitigated', 'closed')),
    impact              text        NOT NULL DEFAULT 'medium' CHECK (impact IN ('low', 'medium', 'high', 'critical')),
    likelihood          text        NOT NULL DEFAULT 'medium' CHECK (likelihood IN ('low', 'medium', 'high')),
    response_strategy   text        NOT NULL DEFAULT '',
    owner_ref           text        NOT NULL DEFAULT '',
    created_by          uuid        NOT NULL REFERENCES core.users (id) ON DELETE RESTRICT,
    created_at          timestamptz NOT NULL DEFAULT now()
);

-- Надання прав plan.apply та plan.edit ролям ядра
UPDATE core.role_definitions
SET permission_keys = ARRAY[
    'project.read', 'wp.read', 'wp.create', 'wp.edit', 'wp.retire',
    'repository.manage', 'trace.create', 'trace.unlink',
    'plan.apply', 'plan.edit'
]
WHERE key = 'project.owner';

UPDATE core.role_definitions
SET permission_keys = ARRAY['scopes.manage', 'plan.apply', 'plan.edit']
WHERE key = 'project.manager';
