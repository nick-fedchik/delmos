# SWR-01: Системні вимоги до ядра проєктного плану та модульного вибору

Дата: 2026-09-23. Статус: нормативні вимоги підсистеми Core & Plan Engine.  
Ліцензія: Apache 2.0.  
Батьківські вимоги: [SHR-01](../stakeholder/SHR-01-generic-project-plan.md), [SHR-02](../stakeholder/SHR-02-module-system-policy.md), [SHR-03](../stakeholder/SHR-03-project-module-activation.md).  
Пов'язані архітектурні рішення: [ADR-004](../../architecture/decisions/ADR-004-two-tier-module-governance-generic-plan.md).

---

#### SWR-01: Автоматична генерація первинного плану при створенні проєкту
При виклику команди створення проєкту сервіс у межах однієї транзакції зобов'язаний:
1. Створити запис у таблиці `projects` зі статусом `initializing`.
2. Згенерувати обов'язковий Work Product `PLAN-001` (`type: plan`, `profile: core:project_plan`).
3. Створити першу immutable ревізію плану з базовою описовою структурою за замовчуванням.
4. Зареєструвати запис у `project_plan_bindings` із посиланням на створений WP.
5. Перевести проєкт у статус `active`.

#### SWR-02: Канонічна описова структура Generic Project Plan
Базова схема Project Plan (`core:project_plan`) повинна містити обов'язкові структурні блоки:
1. `purpose_and_charter`: мета, бізнес-обґрунтування, замовники та спонсори.
2. `scope_and_deliverables`: межі робіт, перелік ключових результатів (deliverables).
3. `team_and_raci`: матриця ролей та відповідальності.
4. `methodology_and_governance`: базовий підхід (agile, waterfall, hybrid), правила прийняття рішень.
5. `phases`: перелік фаз із запланованими датами початку та завершення.
6. `milestones`: контрольні точки/віхи з прив'язкою до фаз і критеріїв приймання.
7. `budget_and_economics`: базовий кошторис та ліміти фінансування.
8. `modules`: секція активованих модулів проєкту та їхньої конфігурації.

#### SWR-03: Версіонування та життєвий цикл Project Plan
1. Зміна параметрів проєкту (ввімкнення модулів, зміна фаз, зміна правил) здійснюється виключно через створення нової draft-ревізії Project Plan.
2. Затвердження плану використовує спільний workflow WP, але вимагає окремого дозволу `plan.approve`, позитивної рецензії та незалежності автора; Approval фіксує точний `revision_id` і `payload_hash` ([ADR-009](../../architecture/decisions/ADR-009-core-review-and-approval-policy.md)).
3. Застосування плану (`plan.apply`) атомарно оновлює проекції проєкту в БД (`effective_plan_revision_id`, `config_generation`).

#### SWR-04: Реєстр системних модулів (Module Definition Registry)
1. Кожен модуль описується маніфестом: `id` із префіксом (`methodology.*`, `compliance.*`, `management.*`, `tooling.*`), `name`, `description`, `version` (SemVer), `category_id`, версії схем та `artifact_digest`.
2. Системний адміністратор змінює `ModuleSystemPolicy` (`available`, `blocked_for_new_activation`, `suspended`).
3. `blocked_for_new_activation` забороняє нові активації без зупинки чинних; `suspended` блокує виконання модуля в чинних проєктах.

#### SWR-05: Сторінка керування модулями в проєкті (Project Plan Modules Page)
1. В інтерфейсі редагування проєктного плану виділена секція `Модулі та розширення`.
2. Для нового вибору доступні лише globally available модулі. Вже використані blocked/suspended модулі залишаються видимими read-only з причиною.
3. При виборі модуля інтерфейс динамічно відображає його конфігураційну форму, валідовану за наданою модулем JSON Schema.
4. Нова ревізія стає effective лише після авторизованого погодження та виконання `plan.apply`.
