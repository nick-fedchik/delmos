# Технічні специфікації підсистем DELMOS (Technical Specifications)

Дата оновлення: 2026-09-23.  
Платформа: **DELMOS** (Discovery, Engineering & Lifecycle Management Operating System).  
Ліцензія: Apache 2.0.

---

## 1. Реєстр технічних специфікацій

| Специфікація | Призначення | Область системи | Пов'язані вимоги |
| --- | --- | --- | --- |
| [SPEC-01: Composite Specifications Engine](SPEC-01-COMPOSITE-SPECIFICATIONS-ENGINE.md) | Формальна специфікація структури композитного документа, маніфесту входжень, канонічного хешування та верифікації SPEC-01..10 | Специфікації, Артефакти, Ядро | SWR-33..35, ADR-003 |
| [SPEC-02: Repository Provider Interface](SPEC-02-REPOSITORY-PROVIDER-INTERFACE.md) | Формальний контракт інтерфейсу `RepositoryProvider`, внутрішнього сховища та адаптерів (GitHub, GitLab, Azure Repos, Bitbucket, Gitea) | Сховища, Інтеграції, Docs-as-Code | SWR-01..02, ADR-005 |
| [SPEC-03: Project Economics & EVM Models](SPEC-03-PROJECT-ECONOMICS-EVM-MODELS.md) | Схеми даних `CostBaseline`, `BudgetLine`, `LaborRate`, `ExpenseRecord`, `WorkRecord` та математичні формули EVM і P&L | Економіка, Ресурси, Метрики | SWR-39..41, ADR-006 |
| [SPEC-04: Events, Triggers & Rules Engine](SPEC-04-EVENTS-TRIGGERS-RULES-ENGINE.md) | Специфікація конверта подій, схеми таблиць Transactional Outbox, протоколу оренди повідомлень (Lease Fencing) та FSM-правил | Автоматизація, Черга, Планувальник | SWR-12..14, SWR-23..27, ADR-007 |
| [CORE-CONTRACT-001: базовий доступ і погодження](CORE-CONTRACT-001-ROLE-WORKFLOW.md) | Постумови першого вертикального зрізу, негативні тести й машинні контракти API/подій | Ядро, доступ, ревізії, workflow | SHR-14, SWR-42..48, ADR-009, ADR-010 |
| [CORE-CONTRACT-002: Generic Project Plan](CORE-CONTRACT-002-GENERIC-PROJECT-PLAN.md) | Структурований нейтральний план, інваріанти, lifecycle `plan.apply` та межа модульних розширень | Ядро, планування, модулі | PM-001, PM-008..010, PM-013..016 |
| [CORE-CONTRACT-003: Generic Plan Domain and Automation](CORE-CONTRACT-003-GENERIC-PLAN-DOMAIN-AND-AUTOMATION.md) | PMBOK/SWEBOK-обґрунтована модель базових сутностей, Work Products та event/trigger/rule handlers | Ядро, планування, автоматизація | SHR-01, SHR-06, SWR-01..04, SWR-12..27 |

---

## 2. Призначення специфікацій

Технічні специфікації описують точні інтерфейси, структури даних, SQL-схеми таблиць та математичні алгоритми функціонування підсистем DELMOS для безпосередньої реалізації інженерами в коді на Go та Vue 3.

Позначення `SPEC-01..04` для історично створених **технічних файлів** перетинається з ідентифікаторами приймальних сценаріїв `SPEC-01..10` у [WORK_PRODUCTS.md](../architecture/WORK_PRODUCTS.md). Тому посилатися на них треба повним шляхом/назвою; новий контракт першого зрізу має однозначний префікс `CORE-CONTRACT-001`, а новим технічним документам не призначається голий `SPEC-NN` без окремого рішення про нумерацію.
