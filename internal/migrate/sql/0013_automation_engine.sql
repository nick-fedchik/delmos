-- 0013: рушій автоматизації (Events, Triggers, Rules, Workflow FSM, Scheduler)
-- за ADR-007, EVENTS_TRIGGERS_RULES.md, SPEC-04-EVENTS-TRIGGERS-RULES-ENGINE.md.
-- audit_events (0002) лишається окремим незмінним журналом доказу (ADR-010) —
-- ці таблиці є лише технічним станом доставки, не аудиторським журналом.

-- Реєстр відомих типів подій ядра та модулів.
CREATE TABLE core.event_definitions (
    event_key       text        PRIMARY KEY,
    name            text        NOT NULL,
    description     text        NOT NULL DEFAULT '',
    owner_module    text        NOT NULL DEFAULT 'core',
    payload_schema  jsonb       NOT NULL DEFAULT '{}'::jsonb,
    created_at      timestamptz NOT NULL DEFAULT now()
);

-- Транзакційний outbox: конверт події пишеться в тій самій транзакції, що
-- й бізнес-зміна; видимий іншим лише після commit.
CREATE TABLE core.event_outbox (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    event_key       text        NOT NULL REFERENCES core.event_definitions (event_key),
    project_id      uuid        NULL REFERENCES core.projects (id) ON DELETE RESTRICT,
    envelope        jsonb       NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    dispatched_at   timestamptz NULL
);

CREATE INDEX event_outbox_undispatched_idx ON core.event_outbox (created_at)
    WHERE dispatched_at IS NULL;

-- Реєстр тригерів: коли й за яких обставин оцінюються правила.
CREATE TABLE core.trigger_definitions (
    trigger_key     text        PRIMARY KEY,
    phase           text        NOT NULL CHECK (phase IN ('before', 'after')),
    input_kind      text        NOT NULL CHECK (input_kind IN ('command', 'event')),
    execution_mode  text        NOT NULL CHECK (execution_mode IN ('in_transaction', 'post_commit')),
    owner_module    text        NOT NULL DEFAULT 'core',
    created_at      timestamptz NOT NULL DEFAULT now()
);

-- Маршрутизація "after"-подій до тригерів-підписників для генерації доставок.
CREATE TABLE core.trigger_subscriptions (
    id                  uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    trigger_key         text        NOT NULL REFERENCES core.trigger_definitions (trigger_key),
    event_key           text        NOT NULL REFERENCES core.event_definitions (event_key),
    subscriber_module   text        NOT NULL DEFAULT 'core',
    routing_generation  int         NOT NULL DEFAULT 1,
    active              boolean     NOT NULL DEFAULT true,
    UNIQUE (trigger_key, event_key)
);

-- Технічний стан доставки (не аудиторський журнал, ADR-010): лізингова
-- оренда повідомлень (Lease Fencing) з ідемпотентним підтвердженням.
CREATE TABLE core.event_deliveries (
    id                  uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    outbox_event_id     uuid        NOT NULL REFERENCES core.event_outbox (id) ON DELETE RESTRICT,
    subscription_id     uuid        NOT NULL REFERENCES core.trigger_subscriptions (id) ON DELETE RESTRICT,
    status              text        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'leased', 'delivered', 'failed', 'dead_letter')),
    lease_token         uuid        NULL,
    lease_until         timestamptz NULL,
    delivery_attempts   int         NOT NULL DEFAULT 0,
    max_attempts        int         NOT NULL DEFAULT 5,
    last_error          text        NULL,
    completed_at        timestamptz NULL,
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX event_deliveries_pending_idx ON core.event_deliveries (status, lease_until)
    WHERE status IN ('pending', 'leased');

-- Декларативні правила: розділяють застосовність (when), інваріант (assert)
-- та дію при порушенні (on_failure). Предикати типізовані й реєструються
-- платформою (SWR-26) — жодного довільного SQL/JS у when/assert.
CREATE TABLE core.rule_definitions (
    rule_key            text        PRIMARY KEY,
    trigger_key         text        NOT NULL REFERENCES core.trigger_definitions (trigger_key),
    owner_module        text        NOT NULL DEFAULT 'core',
    priority            int         NOT NULL DEFAULT 100,
    enforcement_level   text        NOT NULL CHECK (enforcement_level IN ('MANDATORY_VETO', 'ADVISORY_WARNING')),
    when_condition      jsonb       NOT NULL DEFAULT '{"predicate_key": "always_true"}'::jsonb,
    assert_condition    jsonb       NOT NULL,
    on_failure          jsonb       NOT NULL DEFAULT '[]'::jsonb,
    active              boolean     NOT NULL DEFAULT true,
    created_at          timestamptz NOT NULL DEFAULT now()
);

-- Декларативна FSM-схема життєвого циклу (стани/переходи/охоронці/ефекти).
CREATE TABLE core.workflow_definitions (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    key             text        NOT NULL,
    version         text        NOT NULL,
    name            text        NOT NULL,
    target_family   text        NOT NULL,
    definition      jsonb       NOT NULL,
    is_published    boolean     NOT NULL DEFAULT false,
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (key, version)
);

-- Секундний планувальник: атомарний запис факту настання разом з
-- постановкою завдання в outbox у тій самій транзакції.
CREATE TABLE core.schedule_definitions (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    key             text        NOT NULL UNIQUE,
    mode            text        NOT NULL CHECK (mode IN ('once', 'fixed_rate', 'fixed_delay', 'calendar')),
    spec            jsonb       NOT NULL,
    active          boolean     NOT NULL DEFAULT true,
    next_run_at     timestamptz NULL,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE core.schedule_occurrences (
    id                  uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    schedule_id         uuid        NOT NULL REFERENCES core.schedule_definitions (id) ON DELETE CASCADE,
    occurrence_time_utc timestamptz NOT NULL,
    status              text        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'completed', 'failed')),
    created_at          timestamptz NOT NULL DEFAULT now(),
    completed_at        timestamptz NULL,
    UNIQUE (schedule_id, occurrence_time_utc)
);

-- Базові тригери й реєстрація подій ядра, що вже емітуються з v1.0.17.
INSERT INTO core.event_definitions (event_key, name, owner_module) VALUES
    ('wp.revision_committed', 'Зафіксовано нову ревізію work product', 'core'),
    ('trace.link_created', 'Створено зв''язок простежуваності', 'core'),
    ('plan.applied', 'Ревізію плану введено в дію', 'core'),
    ('schedule.occurrence_due', 'Настав час запланованого завдання', 'core');

INSERT INTO core.trigger_definitions (trigger_key, phase, input_kind, execution_mode, owner_module) VALUES
    ('trigger.core.before_wp_transition', 'before', 'command', 'in_transaction', 'core'),
    ('trigger.core.after_revision_committed', 'after', 'event', 'post_commit', 'core');

INSERT INTO core.trigger_subscriptions (trigger_key, event_key, subscriber_module) VALUES
    ('trigger.core.after_revision_committed', 'wp.revision_committed', 'core');

-- Замінює попередню жорстко закодовану перевірку "obsolete WP не
-- редагується" на декларативне обов'язкове правило рушія (доводить
-- MANDATORY_VETO-конвеєр, WORK_PRODUCTS.md §4).
INSERT INTO core.rule_definitions (rule_key, trigger_key, priority, enforcement_level, when_condition, assert_condition) VALUES
    ('rule.core.no_revision_when_obsolete', 'trigger.core.before_wp_transition', 100, 'MANDATORY_VETO',
     '{"predicate_key": "always_true"}'::jsonb,
     '{"predicate_key": "not", "sub_conditions": [{"predicate_key": "field_equals", "params": {"field": "status", "value": "obsolete"}}]}'::jsonb);

-- Право спостереження за станом рушія автоматизації (dead-letter, доставки)
-- для системного адміністратора (RUNBOOK.md, ADM-007).
UPDATE core.role_definitions
SET permission_keys = ARRAY['identities.manage', 'access.grant', 'access.revoke', 'system.observe']
WHERE key = 'system.administrator';

-- Демонстраційна FSM-схема життєвого циклу WP (усі 4 стани описані;
-- реально підключений лише перехід draft->obsolete через RetireWorkProduct,
-- submit/review/approve — окрема майбутня фіча CORE-CONTRACT-001).
INSERT INTO core.workflow_definitions (key, version, name, target_family, definition, is_published) VALUES
    ('core:generic_work_product_lifecycle', '1', 'Життєвий цикл Work Product', 'work_product',
     '{
        "initial_state": "draft",
        "states": [
          {"id": "draft", "label": "Чернетка", "category": "draft"},
          {"id": "in_review", "label": "На рецензуванні", "category": "review"},
          {"id": "approved", "label": "Затверджено", "category": "approved"},
          {"id": "obsolete", "label": "Застаріло", "category": "obsolete"}
        ],
        "transitions": [
          {"id": "retire", "from": "draft", "to": "obsolete", "action_name": "Вивести з експлуатації", "guards": []},
          {"id": "retire_reviewed", "from": "in_review", "to": "obsolete", "action_name": "Вивести з експлуатації", "guards": []},
          {"id": "retire_approved", "from": "approved", "to": "obsolete", "action_name": "Вивести з експлуатації", "guards": []}
        ]
      }'::jsonb, true);
