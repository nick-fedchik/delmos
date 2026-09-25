-- 0010: версіонований шаблон Generic Project Plan, реєстр базових доменних
-- сутностей та структурований маніфест кожної ревізії PLAN-001.

CREATE TABLE core.entity_definitions (
    entity_key      text PRIMARY KEY,
    name            text NOT NULL,
    category        text NOT NULL CHECK (category IN ('configuration', 'operational', 'evidence')),
    owner_module    text NOT NULL DEFAULT 'core',
    description     text NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE core.project_plan_templates (
    template_key    text NOT NULL,
    template_version integer NOT NULL CHECK (template_version > 0),
    name            text NOT NULL,
    description     text NOT NULL,
    manifest_schema jsonb NOT NULL,
    default_manifest jsonb NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (template_key, template_version)
);

CREATE TABLE core.project_plan_manifests (
    revision_id     uuid PRIMARY KEY REFERENCES core.work_product_revisions (id) ON DELETE RESTRICT,
    template_key    text NOT NULL,
    template_version integer NOT NULL CHECK (template_version > 0),
    manifest        jsonb NOT NULL,
    manifest_hash   bytea NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (template_key, template_version)
        REFERENCES core.project_plan_templates (template_key, template_version) ON DELETE RESTRICT
);

INSERT INTO core.entity_definitions (entity_key, name, category, description) VALUES
    ('project_plan', 'Generic Project Plan', 'configuration', 'Єдиний системний PLAN-001 проєкту.'),
    ('plan_application', 'Plan Application', 'configuration', 'Факт застосування ревізії плану.'),
    ('project_objective', 'Project Objective', 'configuration', 'Мета та критерії успіху.'),
    ('scope_item', 'Scope Item', 'configuration', 'Елемент межі in scope або out of scope.'),
    ('assumption', 'Assumption', 'configuration', 'Явне перевірюване припущення.'),
    ('constraint', 'Constraint', 'configuration', 'Обов''язкова межа проєкту.'),
    ('responsibility_assignment', 'Responsibility Assignment', 'configuration', 'Відповідальність у плані без RBAC-повноважень.'),
    ('deliverable', 'Deliverable', 'configuration', 'Результат, що доводиться Work Product.'),
    ('phase', 'Phase', 'configuration', 'Вузол графа плану виконання.'),
    ('milestone', 'Milestone', 'configuration', 'Контрольна точка фази.'),
    ('acceptance_rule', 'Acceptance Rule', 'configuration', 'Типізована умова приймання.'),
    ('stakeholder', 'Stakeholder', 'operational', 'Зацікавлена сторона проєкту.'),
    ('risk', 'Risk', 'operational', 'Невизначеність, що впливає на цілі.'),
    ('issue', 'Issue', 'operational', 'Наявна перешкода для проєкту.'),
    ('change_request', 'Change Request', 'operational', 'Контрольований запит зміни.'),
    ('decision', 'Decision', 'operational', 'Відтворюване управлінське рішення.'),
    ('gate_decision', 'Gate Decision', 'evidence', 'Формальний результат приймання milestone.'),
    ('project_dependency', 'Project Dependency', 'operational', 'Залежність між результатами, фазами або сторонами.')
ON CONFLICT (entity_key) DO NOTHING;

INSERT INTO core.project_plan_templates
    (template_key, template_version, name, description, manifest_schema, default_manifest)
VALUES
    ('generic-project-plan', 1, 'Generic Project Plan',
     'Нейтральний базовий шаблон структурованої конфігурації проєкту.',
     '{"type":"object","required":["objectives","scope_items","assumptions","constraints","responsibility_assignments","deliverables","phases","milestones","acceptance_rules","governance","extensions"]}'::jsonb,
     '{"objectives":[],"scope_items":[],"assumptions":[],"constraints":[],"responsibility_assignments":[],"deliverables":[],"phases":[],"milestones":[],"acceptance_rules":[],"governance":{"change_control_required":true},"extensions":{}}'::jsonb)
ON CONFLICT (template_key, template_version) DO NOTHING;

-- Історичні PLAN-001 були створені до появи структурованого маніфесту. Для
-- них додається детермінований мінімальний екземпляр шаблону без зміни вже
-- зафіксованих Work Product revisions або їхніх hash.
INSERT INTO core.project_plan_manifests
    (revision_id, template_key, template_version, manifest, manifest_hash)
SELECT r.id, 'generic-project-plan', 1, manifest.value,
       digest(convert_to(manifest.value::text, 'UTF8'), 'sha256')
FROM core.project_plan_bindings binding
JOIN core.work_products wp ON wp.id = binding.plan_work_product_id
JOIN core.work_product_revisions r ON r.work_product_id = wp.id
JOIN core.projects project ON project.id = binding.project_id
CROSS JOIN LATERAL (
    SELECT jsonb_build_object(
        'objectives', jsonb_build_array(jsonb_build_object('key', 'OBJ-001', 'statement', 'Deliver ' || project.name, 'success_criteria', '[]'::jsonb)),
        'scope_items', '[]'::jsonb,
        'assumptions', '[]'::jsonb,
        'constraints', '[]'::jsonb,
        'responsibility_assignments', '[]'::jsonb,
        'deliverables', '[]'::jsonb,
        'phases', '[]'::jsonb,
        'milestones', '[]'::jsonb,
        'acceptance_rules', '[]'::jsonb,
        'governance', jsonb_build_object('change_control_required', true),
        'extensions', '{}'::jsonb
    ) AS value
) AS manifest
WHERE wp.type = 'plan' AND wp.profile = 'core:project_plan'
ON CONFLICT (revision_id) DO NOTHING;