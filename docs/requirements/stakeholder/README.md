# Реєстр стейкхолдерських вимог DELMOS (Stakeholder Requirements)

Дата оновлення: 2026-09-23.  
Платформа: **DELMOS** (Discovery, Engineering & Lifecycle Management Operating System).  
Ліцензія: Apache 2.0.

---

## 1. Реєстр вимог стейкхолдерів (SHR)

| Ідентифікатор | Назва вимоги стейкхолдера | Декомпозиція (SWR) | Архітектурне рішення / Специфікація |
| --- | --- | --- | --- |
| [SHR-01](SHR-01-generic-project-plan.md) | Обов'язкова наявність Generic Project Plan для кожного проєкту | SWR-01, SWR-02, SWR-03 | [ADR-004](../../architecture/decisions/ADR-004-two-tier-module-governance-generic-plan.md); [PROJECT_MODEL.md](../../architecture/PROJECT_MODEL.md) |
| [SHR-02](SHR-02-module-system-policy.md) | Глобальне керування політиками модулів адміністратором | SWR-04, SWR-18 | [ADR-004](../../architecture/decisions/ADR-004-two-tier-module-governance-generic-plan.md); [MODULES.md](../../architecture/MODULES.md) |
| [SHR-03](SHR-03-project-module-activation.md) | Вибір і налаштування модулів менеджером у проєктному плані | SWR-05, SWR-17 | [PROJECT_PLAN_ENGINE.md](../../architecture/PROJECT_PLAN_ENGINE.md); [PROJECT_MANAGER.md](../../use-cases/PROJECT_MANAGER.md) |
| [SHR-04](SHR-04-dynamic-content-fields.md) | Конструктор структури контенту та динамічних полів | SWR-06, SWR-07, SWR-08 | [WORK_PRODUCTS.md](../../architecture/WORK_PRODUCTS.md); [MODULES.md](../../architecture/MODULES.md) |
| [SHR-05](SHR-05-milestones-gates-catalog.md) | Модель майлстоунів як керований каталог типів віх і правил | SWR-09, SWR-10, SWR-11 | [PROJECT_MODEL.md](../../architecture/PROJECT_MODEL.md); [MODULE_CATALOG.md](../../architecture/MODULE_CATALOG.md) |
| [SHR-06](SHR-06-events-triggers-rules.md) | Першокласні сутності ядра Events, Triggers, Rules | SWR-12..14, SWR-16, SWR-19..20, SWR-23..27 | [ADR-007](../../architecture/decisions/ADR-007-transactional-outbox-event-scheduler.md); [EVENTS_TRIGGERS_RULES.md](../../architecture/EVENTS_TRIGGERS_RULES.md) |
| [SHR-07](SHR-07-engineering-artifacts-extensions.md) | Модулі розширення інженерних артефактів | SWR-15, SWR-17, SWR-18 | [MODULE_CATALOG.md](../../architecture/MODULE_CATALOG.md); [MODULES.md](../../architecture/MODULES.md) |
| [SHR-08](SHR-08-metrics-and-measurements.md) | Метрики, формули та вимірювані критерії якості | SWR-21, SWR-22 | [METRICS.md](../../architecture/METRICS.md); [EVENTS_TRIGGERS_RULES.md](../../architecture/EVENTS_TRIGGERS_RULES.md) |
| [SHR-09](SHR-09-work-item-and-work-product.md) | Однозначне розмежування роботи (WorkItem) та результатів (WorkProduct) | SWR-28, SWR-29, SWR-30 | [ADR-002](../../architecture/decisions/ADR-002-strict-workitem-workproduct-separation.md); [ITEMS_AND_CONTENT.md](../../architecture/ITEMS_AND_CONTENT.md) |
| [SHR-10](SHR-10-taxonomy-and-reference-data.md) | Керована класифікація, таксономії та довідкові дані | SWR-31, SWR-32 | [TAXONOMY.md](../../architecture/TAXONOMY.md); [ACCESS_CONTROL.md](../../architecture/ACCESS_CONTROL.md) |
| [SHR-11](SHR-11-composite-specifications.md) | Специфікації та композитні документи з пінуванням ревізій | SWR-33, SWR-34, SWR-35 | [ADR-003](../../architecture/decisions/ADR-003-composite-specification-manifests.md); [WORK_PRODUCTS.md](../../architecture/WORK_PRODUCTS.md) |
| [SHR-12](SHR-12-vector-graph-hybrid-analysis.md) | Семантичний пошук та графова візуалізація зв'язків у PostgreSQL | SWR-36, SWR-37, SWR-38 | [ADR-001](../../architecture/decisions/ADR-001-all-in-postgresql-storage.md); [ADR-008](../../architecture/decisions/ADR-008-svg-vector-visualization-for-compliance.md); [VECTOR_AND_GRAPH_DATA.md](../../architecture/VECTOR_AND_GRAPH_DATA.md) |
| [SHR-13](SHR-13-project-economics-resources.md) | Облік ресурсів, трудовитрат та проєктна економіка (EVM, P&L) | SWR-39, SWR-40, SWR-41 | [ADR-006](../../architecture/decisions/ADR-006-project-economics-resource-accounting.md); [METRICS.md](../../architecture/METRICS.md); [MODULE_CATALOG.md](../../architecture/MODULE_CATALOG.md) |
| [SHR-14](SHR-14-core-access-and-assurance.md) | Базові ролі, ізоляція проєктів, незалежне погодження й аудиторська видимість | SWR-42..48 | [ACCESS_CONTROL.md](../../architecture/ACCESS_CONTROL.md); [WORK_PRODUCTS.md](../../architecture/WORK_PRODUCTS.md) |
