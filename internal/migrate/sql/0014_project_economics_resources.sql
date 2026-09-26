-- 0014: проєктна економіка та облік ресурсів (фаза P5)
-- за ADR-006, ADR-011 (нормативна база — серія ISO 21500), SHR-13, SWR-39..41,
-- SPEC-03-PROJECT-ECONOMICS-EVM-MODELS.md.
--
-- Термінологія: ISO 21508 («управління здобутою цінністю») та ISO 21511
-- («ієрархічні структури робіт»). Машинні ідентифікатори англомовні за
-- англійським терміном стандарту (ADR-011 §4).
--
-- Грошові суми — виключно NUMERIC. Жодного float: втрата точності в
-- кошторисі неприйнятна для аудиту.
--
-- Усі таблиці лишаються в схемі core: ADR-001 (All-in-PostgreSQL) і наявна
-- конвенція міграцій 0001-0013. Окремі схеми на модуль не заводяться.

-- Потрібне для EXCLUDE-констрейнта ставок: поєднує рівність uuid/text
-- із перетином daterange в одному GiST-індексі.
CREATE EXTENSION IF NOT EXISTS btree_gist;

-- Виробничий календар: основа номінальної місткості ресурсу (SWR-39.3).
-- Часовий пояс зберігається явно — місткість рахується в календарних днях
-- проєкту, а не в UTC-днях сервера.
CREATE TABLE core.working_calendars (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id        uuid REFERENCES core.projects(id) ON DELETE CASCADE,
    key               text NOT NULL,
    name              text NOT NULL,
    timezone          text NOT NULL DEFAULT 'Europe/Kyiv',
    -- ISO 8601: 1=понеділок .. 7=неділя. Типово — робочий тиждень Пн-Пт.
    work_days         smallint[] NOT NULL DEFAULT ARRAY[1, 2, 3, 4, 5],
    hours_per_day     numeric(4, 2) NOT NULL DEFAULT 8.00,
    created_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT working_calendars_hours_check
        CHECK (hours_per_day > 0 AND hours_per_day <= 24),
    CONSTRAINT working_calendars_work_days_check
        CHECK (work_days <@ ARRAY[1, 2, 3, 4, 5, 6, 7]::smallint[])
);

-- project_id IS NULL означає системний календар за замовчуванням, тому
-- унікальність будується двома частковими індексами, а не UNIQUE-констрейнтом
-- (у SQL NULL не дорівнює NULL, тож звичайний UNIQUE не спрацював би).
CREATE UNIQUE INDEX working_calendars_project_key_uniq
    ON core.working_calendars (project_id, key) WHERE project_id IS NOT NULL;
CREATE UNIQUE INDEX working_calendars_system_key_uniq
    ON core.working_calendars (key) WHERE project_id IS NULL;

-- Винятки календаря: свята та неробочі дні, що зменшують номінальну місткість.
CREATE TABLE core.calendar_exceptions (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    calendar_id       uuid NOT NULL REFERENCES core.working_calendars(id) ON DELETE CASCADE,
    exception_date    date NOT NULL,
    is_working_day    boolean NOT NULL DEFAULT false,
    hours_override    numeric(4, 2),
    reason            text NOT NULL,
    UNIQUE (calendar_id, exception_date),
    CONSTRAINT calendar_exceptions_hours_check
        CHECK (hours_override IS NULL OR (hours_override >= 0 AND hours_override <= 24))
);

-- Планова зайнятість ресурсу у фазі (SWR-39.2). Зберігається у FTE;
-- години виводяться з календаря, а не дублюються тут другим джерелом істини.
CREATE TABLE core.resource_allocations (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id        uuid NOT NULL REFERENCES core.projects(id) ON DELETE CASCADE,
    user_id           uuid NOT NULL REFERENCES core.users(id) ON DELETE RESTRICT,
    phase_key         text NOT NULL,
    role_key          text NOT NULL,
    allocated_fte     numeric(4, 3) NOT NULL,
    allocation_start  date NOT NULL,
    allocation_finish date NOT NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, user_id, phase_key),
    FOREIGN KEY (project_id, phase_key)
        REFERENCES core.project_phases (project_id, phase_key) ON DELETE CASCADE,
    CONSTRAINT resource_allocations_fte_check
        CHECK (allocated_fte > 0 AND allocated_fte <= 1.000),
    CONSTRAINT resource_allocations_period_check
        CHECK (allocation_finish >= allocation_start)
);

CREATE INDEX resource_allocations_user_period_idx
    ON core.resource_allocations (user_id, allocation_start, allocation_finish);

-- Табель обліку праці (SWR-39.1). Базова одиниця фактичних трудовитрат.
--
-- Свідоме відхилення від SPEC-03 §1.1: замість нетипізованого polymorphic
-- subject_id заведено явний FK на work_products. Сутності WorkItem у ядрі ще
-- немає (ADR-002 розділяє їх, але таблиці немає), а UUID без FK — це діра в
-- цілісності, яку СУБД не контролює. Коли зʼявиться WorkItem, додається
-- окрема колонка з власним FK і CHECK на взаємну виключність.
CREATE TABLE core.work_records (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id        uuid NOT NULL REFERENCES core.projects(id) ON DELETE CASCADE,
    user_id           uuid NOT NULL REFERENCES core.users(id) ON DELETE RESTRICT,
    work_product_id   uuid REFERENCES core.work_products(id) ON DELETE RESTRICT,
    phase_key         text NOT NULL,
    role_key          text NOT NULL,
    work_date         date NOT NULL,
    duration_hours    numeric(6, 2) NOT NULL,
    work_category     text NOT NULL DEFAULT 'development',
    comment           text,
    created_at        timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (project_id, phase_key)
        REFERENCES core.project_phases (project_id, phase_key) ON DELETE RESTRICT,
    CONSTRAINT work_records_duration_check
        CHECK (duration_hours > 0 AND duration_hours <= 24),
    CONSTRAINT work_records_category_check
        CHECK (work_category IN ('design', 'development', 'review', 'testing', 'management'))
);

CREATE INDEX work_records_project_date_idx ON core.work_records (project_id, work_date);
CREATE INDEX work_records_user_date_idx ON core.work_records (user_id, work_date);
CREATE INDEX work_records_phase_idx ON core.work_records (project_id, phase_key);

-- Версіонована ставка собівартості ролі (SWR-40.2). Перекриття періодів для
-- однієї ролі виключається EXCLUDE-констрейнтом, а не перевіркою в коді:
-- інакше дві конкурентні транзакції створили б дві чинні ставки на ту саму
-- дату, і розрахунок AC став би недетермінованим.
CREATE TABLE core.labor_rates (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id        uuid REFERENCES core.projects(id) ON DELETE CASCADE,
    role_key          text NOT NULL,
    hourly_rate       numeric(10, 2) NOT NULL,
    currency          text NOT NULL DEFAULT 'EUR',
    valid_from        date NOT NULL,
    valid_to          date,
    created_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT labor_rates_rate_check CHECK (hourly_rate >= 0),
    CONSTRAINT labor_rates_currency_check CHECK (currency ~ '^[A-Z]{3}$'),
    CONSTRAINT labor_rates_period_check CHECK (valid_to IS NULL OR valid_to > valid_from),
    EXCLUDE USING gist (
        coalesce(project_id, '00000000-0000-0000-0000-000000000000'::uuid) WITH =,
        role_key WITH =,
        daterange(valid_from, valid_to, '[)') WITH &&
    )
);

CREATE INDEX labor_rates_lookup_idx ON core.labor_rates (role_key, valid_from);

-- Базовий кошторис (SWR-40.1). Версіонований; затверджена версія незмінна.
CREATE TABLE core.cost_baselines (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id        uuid NOT NULL REFERENCES core.projects(id) ON DELETE CASCADE,
    version           integer NOT NULL,
    name              text NOT NULL,
    currency          text NOT NULL DEFAULT 'EUR',
    status            text NOT NULL DEFAULT 'draft',
    approved_by       uuid REFERENCES core.users(id) ON DELETE RESTRICT,
    approved_at       timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, version),
    CONSTRAINT cost_baselines_version_check CHECK (version > 0),
    CONSTRAINT cost_baselines_currency_check CHECK (currency ~ '^[A-Z]{3}$'),
    CONSTRAINT cost_baselines_status_check
        CHECK (status IN ('draft', 'pending_approval', 'approved', 'superseded')),
    -- Затвердження неможливе без указаного затверджувача й часу: це доказ
    -- для аудиту (ADR-010), а не декоративні поля.
    CONSTRAINT cost_baselines_approval_check CHECK (
        (status = 'approved' AND approved_by IS NOT NULL AND approved_at IS NOT NULL)
        OR (status <> 'approved' AND approved_by IS NULL AND approved_at IS NULL)
    )
);

-- Лише одна затверджена версія кошторису на проєкт: BAC має бути однозначним.
CREATE UNIQUE INDEX cost_baselines_single_approved_idx
    ON core.cost_baselines (project_id) WHERE status = 'approved';

-- Стаття кошторису за фазою та категорією витрат (SWR-40.1).
-- Сума planned_amount усіх рядків затвердженої версії дає BAC.
CREATE TABLE core.budget_lines (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    cost_baseline_id  uuid NOT NULL REFERENCES core.cost_baselines(id) ON DELETE CASCADE,
    project_id        uuid NOT NULL REFERENCES core.projects(id) ON DELETE CASCADE,
    phase_key         text NOT NULL,
    cost_category     text NOT NULL,
    planned_amount    numeric(14, 2) NOT NULL,
    funding_limit     numeric(14, 2) NOT NULL,
    UNIQUE (cost_baseline_id, phase_key, cost_category),
    FOREIGN KEY (project_id, phase_key)
        REFERENCES core.project_phases (project_id, phase_key) ON DELETE CASCADE,
    CONSTRAINT budget_lines_planned_check CHECK (planned_amount >= 0),
    CONSTRAINT budget_lines_limit_check CHECK (funding_limit >= planned_amount),
    CONSTRAINT budget_lines_category_check CHECK (cost_category IN (
        'labor', 'hardware_prototypes', 'tooling_nre',
        'software_licenses', 'testing_services', 'contingency'
    ))
);

CREATE INDEX budget_lines_phase_idx ON core.budget_lines (project_id, phase_key);

-- Фактичні нетрудові витрати (SWR-40.4). CapEx/OpEx розрізняються для
-- подальшої капіталізації R&D (IAS 38 — довідково, ADR-011).
CREATE TABLE core.expense_records (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id        uuid NOT NULL REFERENCES core.projects(id) ON DELETE CASCADE,
    phase_key         text NOT NULL,
    cost_category     text NOT NULL,
    expense_type      text NOT NULL,
    amount            numeric(14, 2) NOT NULL,
    currency          text NOT NULL DEFAULT 'EUR',
    invoice_reference text,
    expense_date      date NOT NULL,
    recorded_by       uuid NOT NULL REFERENCES core.users(id) ON DELETE RESTRICT,
    created_at        timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (project_id, phase_key)
        REFERENCES core.project_phases (project_id, phase_key) ON DELETE RESTRICT,
    CONSTRAINT expense_records_amount_check CHECK (amount > 0),
    CONSTRAINT expense_records_currency_check CHECK (currency ~ '^[A-Z]{3}$'),
    CONSTRAINT expense_records_type_check CHECK (expense_type IN ('capex', 'opex')),
    CONSTRAINT expense_records_category_check CHECK (cost_category IN (
        'labor', 'hardware_prototypes', 'tooling_nre',
        'software_licenses', 'testing_services', 'contingency'
    ))
);

CREATE INDEX expense_records_project_date_idx ON core.expense_records (project_id, expense_date);
CREATE INDEX expense_records_phase_idx ON core.expense_records (project_id, phase_key);

-- Привʼязка результату до фази з ваговим коефіцієнтом (ISO 21511).
-- Робить здобуту цінність обчислюваною: без явного звʼязку "фаза -> результат"
-- метод вимірювання EV за ISO 21508 не має на що спиратися.
-- Застосовується правило 0/100: результат зараховується лише після approved.
CREATE TABLE core.phase_deliverables (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id        uuid NOT NULL REFERENCES core.projects(id) ON DELETE CASCADE,
    phase_key         text NOT NULL,
    work_product_id   uuid NOT NULL REFERENCES core.work_products(id) ON DELETE CASCADE,
    weight            numeric(6, 3) NOT NULL DEFAULT 1.000,
    created_at        timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, work_product_id),
    FOREIGN KEY (project_id, phase_key)
        REFERENCES core.project_phases (project_id, phase_key) ON DELETE CASCADE,
    CONSTRAINT phase_deliverables_weight_check CHECK (weight > 0)
);

CREATE INDEX phase_deliverables_phase_idx ON core.phase_deliverables (project_id, phase_key);

-- Системний календар за замовчуванням: 40-годинний тиждень.
INSERT INTO core.working_calendars (project_id, key, name, timezone, work_days, hours_per_day)
VALUES (NULL, 'default', 'Стандартний робочий тиждень (40 год)', 'Europe/Kyiv',
        ARRAY[1, 2, 3, 4, 5]::smallint[], 8.00);

-- Права доступу до економіки. Фінансові дані відокремлені від інженерних:
-- ставки собівартості бачить не кожен, хто читає проєкт (ADR-006, наслідки).
UPDATE core.role_definitions
SET permission_keys = permission_keys || ARRAY['timesheet.log']
WHERE key = 'project.owner';

UPDATE core.role_definitions
SET permission_keys = permission_keys
    || ARRAY['timesheet.log', 'resources.manage', 'economics.read', 'economics.manage']
WHERE key = 'project.manager';

-- Затвердження кошторису — окреме право, відокремлене від economics.manage
-- (SoD: хто складає кошторис, той його не затверджує).
INSERT INTO core.role_definitions (key, name, permission_keys)
VALUES (
    'project.financial_controller',
    'Фінансовий контролер проєкту',
    ARRAY['economics.read', 'economics.approve']
);

-- Тригер before-фази: точка, де рушій правил перевіряє інваріанти переходу
-- фази до її фактичного застосування (SPEC-04, фаза before).
INSERT INTO core.trigger_definitions (trigger_key, phase, input_kind, execution_mode, owner_module) VALUES
    ('trigger.core.before_phase_transition', 'before', 'command', 'in_transaction', 'core');

-- Шлюз ліміту фінансування (SHR-13, критерій 5): фаза не закривається, якщо
-- фактичні витрати перевищили затверджений ліміт фінансування. Предикат
-- within_funding_limit зареєстровано платформою (SWR-26) — довільного
-- SQL чи JS в умові немає.
INSERT INTO core.rule_definitions (rule_key, trigger_key, priority, enforcement_level, when_condition, assert_condition) VALUES
    ('rule.economics.funding_limit_gate', 'trigger.core.before_phase_transition', 100, 'MANDATORY_VETO',
     '{"predicate_key": "field_equals", "params": {"field": "target_status", "value": "completed"}}'::jsonb,
     '{"predicate_key": "within_funding_limit"}'::jsonb);

-- Попередження про відхилення вартості при CPI < 0.85 (MODULE_CATALOG §4.2).
-- ADVISORY_WARNING: фіксується як зауваження, переходу не блокує.
INSERT INTO core.rule_definitions (rule_key, trigger_key, priority, enforcement_level, when_condition, assert_condition) VALUES
    ('rule.economics.cost_variance_alert', 'trigger.core.before_phase_transition', 200, 'ADVISORY_WARNING',
     '{"predicate_key": "field_equals", "params": {"field": "target_status", "value": "completed"}}'::jsonb,
     '{"predicate_key": "cpi_above", "params": {"threshold": 0.85}}'::jsonb);
