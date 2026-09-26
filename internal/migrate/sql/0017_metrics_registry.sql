-- 0017: типізований реєстр метрик та незмінні вимірювання (SWR-21, SWR-22).
--
-- Ключова вимога SWR-21.1: «Заборонено довільний нетипізований JSON як
-- значення метрики». Тому значення зберігається в колонці, що відповідає
-- оголошеному типу, а відповідність типу тримає складений зовнішній ключ
-- (metric_key, value_type) — не код. Помилка в застосунку не запише
-- десяткове значення в метрику, оголошену як boolean.
--
-- Одиниці: SWR-21.2 перелічує count, percent, ms, currency:EUR, currency:USD.
-- Додано `ratio` — CPI та SPI безрозмірні, і METRICS.md уже оголошує їх так;
-- позначати індекс як percent було б хибно за змістом. Форму currency:XXX
-- узято з вимоги, вона ж вирішує багатовалютність.

CREATE TABLE core.metric_definitions (
    metric_key        text PRIMARY KEY,
    name              text NOT NULL,
    value_type        text NOT NULL,
    unit              text NOT NULL,
    owner_module      text NOT NULL DEFAULT 'core',
    enum_values       text[],
    -- Поріг застарівання. NULL означає, що вимірювання не застаріває з часом.
    max_age_seconds   integer,
    -- Чи блокує застаріле/невалідне вимірювання фазовий шлюз (SWR-22.3).
    required_for_gate boolean NOT NULL DEFAULT false,
    created_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT metric_definitions_value_type_check
        CHECK (value_type IN ('integer', 'decimal', 'duration', 'boolean', 'enum', 'distribution')),
    CONSTRAINT metric_definitions_unit_check
        CHECK (unit IN ('count', 'percent', 'ratio', 'ms') OR unit ~ '^currency:[A-Z]{3}$'),
    CONSTRAINT metric_definitions_enum_check
        CHECK (value_type <> 'enum' OR (enum_values IS NOT NULL AND cardinality(enum_values) > 0)),
    CONSTRAINT metric_definitions_max_age_check
        CHECK (max_age_seconds IS NULL OR max_age_seconds > 0),
    -- Ціль складеного зовнішнього ключа з observations.
    UNIQUE (metric_key, value_type)
);

-- Незмінне вимірювання (SWR-22.2). Рядки ніколи не оновлюються: нове
-- обчислення створює новий рядок, історія показників лишається доказом для
-- аудиту фазових шлюзів.
CREATE TABLE core.metric_observations (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    metric_key         text NOT NULL,
    value_type         text NOT NULL,
    project_id         uuid NOT NULL REFERENCES core.projects(id) ON DELETE CASCADE,
    -- NULL означає рівень проєкту, інакше — конкретна фаза.
    phase_key          text,
    quality            text NOT NULL,
    value_integer      bigint,
    value_decimal      numeric(20, 6),
    value_duration     interval,
    value_boolean      boolean,
    value_enum         text,
    value_distribution jsonb,
    -- Причина для no_data та error: без неї стан неможливо розслідувати.
    detail             text,
    computed_at        timestamptz NOT NULL DEFAULT now(),
    correlation_id     uuid,
    created_at         timestamptz NOT NULL DEFAULT now(),

    FOREIGN KEY (metric_key, value_type)
        REFERENCES core.metric_definitions (metric_key, value_type) ON DELETE RESTRICT,

    CONSTRAINT metric_observations_quality_check
        CHECK (quality IN ('valid', 'stale', 'no_data', 'error')),

    -- Для no_data та error значення не існує. Записати його туди означало б
    -- видати відсутність даних за виміряний нуль (METRICS.md §6).
    CONSTRAINT metric_observations_absent_value_check CHECK (
        quality IN ('valid', 'stale')
        OR (value_integer IS NULL AND value_decimal IS NULL AND value_duration IS NULL
            AND value_boolean IS NULL AND value_enum IS NULL AND value_distribution IS NULL)
    ),
    CONSTRAINT metric_observations_detail_check
        CHECK (quality IN ('valid', 'stale') OR (detail IS NOT NULL AND detail <> '')),

    -- Значення лежить рівно в тій колонці, що відповідає оголошеному типу.
    CONSTRAINT metric_observations_typed_value_check CHECK (
        quality NOT IN ('valid', 'stale')
        OR (
            (value_type = 'integer' AND value_integer IS NOT NULL AND value_decimal IS NULL
                AND value_duration IS NULL AND value_boolean IS NULL AND value_enum IS NULL AND value_distribution IS NULL)
         OR (value_type = 'decimal' AND value_decimal IS NOT NULL AND value_integer IS NULL
                AND value_duration IS NULL AND value_boolean IS NULL AND value_enum IS NULL AND value_distribution IS NULL)
         OR (value_type = 'duration' AND value_duration IS NOT NULL AND value_integer IS NULL
                AND value_decimal IS NULL AND value_boolean IS NULL AND value_enum IS NULL AND value_distribution IS NULL)
         OR (value_type = 'boolean' AND value_boolean IS NOT NULL AND value_integer IS NULL
                AND value_decimal IS NULL AND value_duration IS NULL AND value_enum IS NULL AND value_distribution IS NULL)
         OR (value_type = 'enum' AND value_enum IS NOT NULL AND value_integer IS NULL
                AND value_decimal IS NULL AND value_duration IS NULL AND value_boolean IS NULL AND value_distribution IS NULL)
         OR (value_type = 'distribution' AND value_distribution IS NOT NULL AND value_integer IS NULL
                AND value_decimal IS NULL AND value_duration IS NULL AND value_boolean IS NULL AND value_enum IS NULL)
        )
    )
);

CREATE INDEX metric_observations_lookup_idx
    ON core.metric_observations (project_id, metric_key, phase_key, computed_at DESC);

-- Незмінність на рівні СУБД: вимірювання є доказом, а доказ не редагують.
CREATE OR REPLACE FUNCTION core.metric_observations_immutable()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'metric_observations є незмінними (SWR-22.2): операція % заборонена', TG_OP;
END;
$$;

CREATE TRIGGER metric_observations_no_update
    BEFORE UPDATE OR DELETE ON core.metric_observations
    FOR EACH ROW EXECUTE FUNCTION core.metric_observations_immutable();

-- Реєстр метрик здобутої цінності (METRICS.md §5). Валюта EUR відповідає
-- типовій валюті кошторису; проєкт в іншій валюті отримує власні визначення.
INSERT INTO core.metric_definitions
    (metric_key, name, value_type, unit, owner_module, max_age_seconds, required_for_gate) VALUES
    ('economics.pv',  'Планова вартість (Planned Value)',      'decimal', 'currency:EUR', 'core', 86400, true),
    ('economics.ac',  'Фактична вартість (Actual Cost)',       'decimal', 'currency:EUR', 'core', 86400, true),
    ('economics.ev',  'Здобута цінність (Earned Value)',       'decimal', 'currency:EUR', 'core', 86400, true),
    ('economics.cv',  'Відхилення вартості (Cost Variance)',   'decimal', 'currency:EUR', 'core', 86400, false),
    ('economics.sv',  'Відхилення розкладу (Schedule Variance)','decimal', 'currency:EUR', 'core', 86400, false),
    ('economics.cpi', 'Індекс виконання бюджету (CPI)',        'decimal', 'ratio',        'core', 86400, false),
    ('economics.spi', 'Індекс дотримання розкладу (SPI)',      'decimal', 'ratio',        'core', 86400, false);

-- Обчислення метрик виконує фоновий обробник, а не шлях запиту (SWR-22.1).
-- Подія економіки потрібна саме для розриву кола: якби метрики оновлювалися
-- переходом фази, шлюз блокував би перехід, який єдиний їх би й оновив.
INSERT INTO core.event_definitions (event_key, name, owner_module) VALUES
    ('economics.inputs_changed', 'Змінилися вхідні дані проєктної економіки', 'core');

INSERT INTO core.trigger_definitions (trigger_key, phase, input_kind, execution_mode, owner_module) VALUES
    ('trigger.core.after_economics_changed', 'after', 'event', 'post_commit', 'core');

INSERT INTO core.trigger_subscriptions (trigger_key, event_key, subscriber_module) VALUES
    ('trigger.core.after_economics_changed', 'economics.inputs_changed', 'core'),
    -- Завершення фази теж змінює картину: наступна фаза стає активною.
    ('trigger.core.after_economics_changed', 'phase.transitioned', 'core');

-- Застарілі чи невалідні вимірювання блокують обов'язковий фазовий шлюз
-- шлюз (SWR-22.3). Правило застосовне лише там, де економічний контроль
-- справді запроваджено: проєкт без затвердженого кошторису не має чого
-- вимірювати, і вічне блокування було б хибою, а не суворістю.
INSERT INTO core.rule_definitions
    (rule_key, trigger_key, priority, enforcement_level, when_condition, assert_condition) VALUES
    ('rule.core.gate_requires_fresh_metrics', 'trigger.core.before_phase_transition', 50, 'MANDATORY_VETO',
     '{"predicate_key": "and", "sub_conditions": [
         {"predicate_key": "field_equals", "params": {"field": "target_status", "value": "completed"}},
         {"predicate_key": "field_equals", "params": {"field": "economics_applicable", "value": true}}
       ]}'::jsonb,
     '{"predicate_key": "metrics_fresh"}'::jsonb);
