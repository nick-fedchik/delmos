-- 0015: незалежна рецензія та погодження точної ревізії (CORE-CONTRACT-001)
-- за ADR-009, SHR-14, SWR-45..46.
--
-- Центральний інваріант ADR-009 §3: рішення завжди прив'язане до конкретної
-- пари (revision_id, payload_hash). Створення наступної ревізії потребує
-- нового ReviewRequest; старі рішення ніколи не переносяться на новий хеш.
-- Тому payload_hash дублюється в кожному рядку: це не надмірність, а фіксація
-- того, ЩО саме підписав рецензент на момент рішення.

-- Запит на рецензію точної ревізії.
CREATE TABLE core.review_requests (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id        uuid NOT NULL REFERENCES core.projects(id) ON DELETE CASCADE,
    work_product_id   uuid NOT NULL REFERENCES core.work_products(id) ON DELETE RESTRICT,
    revision_id       uuid NOT NULL REFERENCES core.work_product_revisions(id) ON DELETE RESTRICT,
    payload_hash      bytea NOT NULL,
    status            text NOT NULL DEFAULT 'open',
    requested_by      uuid NOT NULL REFERENCES core.users(id) ON DELETE RESTRICT,
    created_at        timestamptz NOT NULL DEFAULT now(),
    closed_at         timestamptz,
    CONSTRAINT review_requests_status_check
        CHECK (status IN ('open', 'approved', 'changes_requested', 'superseded')),
    CONSTRAINT review_requests_closed_check
        CHECK ((status = 'open') = (closed_at IS NULL)),
    -- Дає змогу прив'язати рішення до запиту РАЗОМ з ревізією одним FK нижче.
    UNIQUE (id, revision_id)
);

-- Один відкритий запит на артефакт: інакше два паралельні подання створили б
-- дві незалежні гілки погодження на різні ревізії одного WP.
CREATE UNIQUE INDEX review_requests_single_open_idx
    ON core.review_requests (work_product_id) WHERE status = 'open';

CREATE INDEX review_requests_project_idx ON core.review_requests (project_id, status);

-- Явне призначення особи на запит (ADR-009 §1). Наявність дозволу без
-- призначення недостатня: право дає змогу діяти, призначення визначає — над чим.
CREATE TABLE core.review_assignments (
    review_request_id uuid NOT NULL REFERENCES core.review_requests(id) ON DELETE CASCADE,
    user_id           uuid NOT NULL REFERENCES core.users(id) ON DELETE RESTRICT,
    assignment_role   text NOT NULL,
    assigned_by       uuid NOT NULL REFERENCES core.users(id) ON DELETE RESTRICT,
    assigned_at       timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (review_request_id, user_id, assignment_role),
    CONSTRAINT review_assignments_role_check
        CHECK (assignment_role IN ('reviewer', 'approver'))
);

-- Незмінний запис рішення. Рядки ніколи не оновлюються й не видаляються:
-- це доказ для аудиту (ADR-010), а не поточний стан.
CREATE TABLE core.review_decisions (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    review_request_id uuid NOT NULL,
    revision_id       uuid NOT NULL,
    payload_hash      bytea NOT NULL,
    decision_kind     text NOT NULL,
    outcome           text NOT NULL,
    decided_by        uuid NOT NULL REFERENCES core.users(id) ON DELETE RESTRICT,
    reason            text,
    operation_key     text,
    decided_at        timestamptz NOT NULL DEFAULT now(),
    -- Складений FK не дає записати рішення на ревізію, відмінну від тієї, на
    -- яку створено запит. Без нього розбіжність довелося б ловити в коді.
    FOREIGN KEY (review_request_id, revision_id)
        REFERENCES core.review_requests (id, revision_id) ON DELETE CASCADE,
    CONSTRAINT review_decisions_kind_check
        CHECK (decision_kind IN ('review', 'approval', 'request_changes')),
    CONSTRAINT review_decisions_outcome_check
        CHECK (outcome IN ('positive', 'changes_requested')),
    -- «Потрібні зміни» без причини не є вмотивованим рішенням (ADR-009 §3).
    CONSTRAINT review_decisions_reason_check
        CHECK (decision_kind <> 'request_changes' OR (reason IS NOT NULL AND reason <> '')),
    -- Одна особа дає щонайбільше одне рішення кожного роду на запит: інакше
    -- повторний виклик додав би другий підпис тієї самої особи до кворуму.
    UNIQUE (review_request_id, decided_by, decision_kind)
);

CREATE INDEX review_decisions_request_idx ON core.review_decisions (review_request_id);

-- Ідемпотентність повторного виклику (ADR-009 §4): той самий ключ операції
-- повертає попередній результат замість другого підпису.
CREATE UNIQUE INDEX review_decisions_operation_key_idx
    ON core.review_decisions (operation_key) WHERE operation_key IS NOT NULL;

-- Права рецензування. Окремі дозволи для review та approve — умова, за якої
-- одна особа може бути і рецензентом, і погоджувачем (ADR-009 §2).
UPDATE core.role_definitions
SET permission_keys = permission_keys || ARRAY['wp.submit']
WHERE key = 'project.owner';

INSERT INTO core.role_definitions (key, name, permission_keys) VALUES
    ('project.reviewer', 'Рецензент проєкту',
     ARRAY['project.read', 'wp.read', 'wp.review', 'wp.request_changes']),
    ('project.approver', 'Погоджувач проєкту',
     ARRAY['project.read', 'wp.read', 'wp.approve', 'wp.request_changes']);

-- Подання на рецензію — окрема подія ядра для рушія автоматизації.
INSERT INTO core.event_definitions (event_key, name, owner_module) VALUES
    ('wp.submitted_for_review', 'Артефакт подано на рецензію', 'core'),
    ('wp.approved', 'Артефакт затверджено', 'core'),
    ('wp.changes_requested', 'Щодо артефакту запитано зміни', 'core');
