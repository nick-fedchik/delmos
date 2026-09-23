# План портування та розбудови платформи DELMOS

Дата: 2026-09-23.  
Статус: Стратегічний та операційний план розбудови нової незалежної платформи.  
Ліцензія: Apache 2.0.

---

## 1. Мета та принципи портування

Метою є створення автономного, незалежного від легасі-систем продукту **DELMOS** (Discovery, Engineering & Lifecycle Management Operating System).

### Ключові принципи:
1. **Повна автономність:** Жодних жорстких прив'язок до сторонніх закритих платформ, легасі-термінології чи специфічних зовнішніх брокерів повідомлень.
2. **Чистота коду та ліцензійна прозорість:** Усі напрацювання переводяться під чисту ліцензію **Apache 2.0**.
3. **Docs-First & Model-Driven:** Архітектура, схеми даних, інваріанти та вимоги мають бути зафіксовані та верифіковані до початку масштабної кодогенерації.
4. **All-in-PostgreSQL:** Уніфікація реляційних, графових і векторних даних у межах єдиної ACID-СУБД на цільовому сервері Ubuntu.

---

## 2. Етапи розбудови (Phases P0 – P7)

### Етап 0: Концептуалізація, повний ребрендинг та перенесення документації (Поточний)
- [x] Ініціалізація автономного каталогу `~/delmos` із файлами `LICENSE` (Apache 2.0), `NOTICE`, `README.md`.
- [ ] Формування повного дерева документації: `docs/requirements/`, `docs/architecture/`, `docs/use-cases/`.
- [ ] Очищення документації від усіх легасі-термінів, перейменування в **DELMOS**.
- [ ] Інтеграція нових архітектурних модулів:
  - Мультипровайдерне сховище (`RepositoryProvider`: Internal, Plain Git, GitHub, GitLab, Azure Repos, Bitbucket, Gitea);
  - Розширений інженерний цикл (Pre-sales, Feasibility, Hardware, Software, Delivery);
  - Проєктна економіка (`management.resources`, `management.economics`, EVM, Cost Baselines);
  - Векторні (`pgvector`) та графові (`CTE CYCLE`, `ltree`) моделі в PostgreSQL, векторна візуалізація (SVG).
- [ ] Повна перевірка валідності посилань та синтаксису Markdown.

### Етап 1: Базовий рантайм та ядро даних (Core Scaffolding & Native Runtime)
- Створення Go-модуля `delmos` (`go.mod`).
- Нативний `Makefile` для збірки, тестів, лінтування та FHS-встановлення у систему (`/usr/local/bin/delmos`, `/etc/delmos/`).
- Початкові міграції PostgreSQL:
  - Схема сесій, облікових записів і ролей (Scoped RBAC);
  - Таблиці програм, проєктів і обов'язкового зв'язку `project_plan_bindings`;
  - Базові 6 типів Work Products (`plan`, `requirement`, `architecture`, `test_spec`, `report`, `record`).
- Механізм автоматичної ініціалізації: створення проєкту атомарно генерує обов'язковий `Generic Project Plan` (`PLAN-001`).

### Етап 2: Специфікації артефактів та композитні документи (Work Products & Specifications)
- Сервіс валідації метаданих через JSON Schema Draft 2020-12.
- Незмінні ревізії артефактів (`work_product_revisions`) із розрахунком детермінованого `payload_hash`.
- Модель композитних документів:
  - Збереження структури розділів і впорядкованих входжень (occurrences);
  - Фіксація точних маніфестів ревізій компонентів;
  - Незалежний життєвий цикл і погодження специфікації та її складових елементів.
- Проходження 10 автоматизованих сценаріїв приймання композиції (**SPEC-01..10**).

### Етап 3: Векторна підсистема та рекурсивні графи (Vectors & Graphs in PostgreSQL)
- Активація розширення `pgvector` та створення таблиці `wp_embeddings` із HNSW-індексацією.
- Асинхронний розрахунок ембедінгів для ревізій вимог і тест-кейсів через подієву чергу.
- Гібридний семантичний пошук дублікатів та рекомендація зв'язків (Smart Link Suggestion) з урахуванням прав доступу.
- Реляційний граф `trace_links` із рекурсивним обходом `WITH RECURSIVE` та нативним захистом `CYCLE target_id SET is_cycle USING path`.
- Автоматичний розрахунок і поширення прапорця підозрілості (`is_suspect`) при оновленні джерел (Change Impact Analysis).
- Проходження тестів **VEC-01..03** та **GRP-01..03**.

### Етап 4: Події, тригери, правила та планувальник (Automation Engine)
- Реєстр сутностей ядра: `event_definitions`, `trigger_definitions`, `rule_definitions`.
- Секундний планувальник завдань (`ScheduleDefinition` / `ScheduleOccurrence`) із захистом від дрейфу часу та повторного запуску.
- Надійна черга повідомлень на базі PostgreSQL (`inbox`, `outbox`, `event_deliveries`) із механізмом оренди (leases) та ідемпотентною обробкою.
- Декларативний FSM-рушій робочих процесів (`workflow_definitions`) із перевіркою guards та емісією побічних подій.

### Етап 5: Проєктна економіка та облік ресурсів (Economics & Resources)
- Модуль `management.resources`:
  - Табельний облік трудовитрат (`WorkRecord`);
  - Планування місткості (`ResourceAllocation`, FTE);
  - Робочі календарі (`WorkingCalendar`).
- Модуль `management.economics`:
  - Базовий кошторис (`CostBaseline`) та ліміти фінансування;
  - Внутрішні ставки собівартості ролей (`LaborRate`);
  - Облік матеріальних витрат (`ExpenseRecord`, CapEx vs OpEx);
  - Розрахунок показників Earned Value Management ($PV, AC, EV, CPI, SPI, EAC$);
  - Комерційний P&L аналіз (Contract Revenue vs Cost).

### Етап 6: Мультипровайдерне сховище (Storage & Tracker Adapters)
- Реалізація інтерфейсу `RepositoryProvider`:
  - *Internal Provider:* збереження Docs-as-Code безпосередньо у внутрішній БД або локальному bare Git-репозиторії;
  - *Generic Git Provider:* робота з віддаленими репозиторіями по SSH/HTTPS;
  - *GitLab Adapter:* REST-інтеграція з проектами, MR та CI-пайплайнами;
  - *GitHub Adapter:* інтеграція з репозиторіями GitHub та GitHub Actions;
  - *Azure DevOps Adapter:* інтеграція з Azure Repos та Azure Pipelines;
  - *Bitbucket & Gitea/Forgejo Adapters.*

### Етап 7: Вебінтерфейс та векторна графіка (UI & Vector Visualization)
- SPA-інтерфейс на базі Vue 3 та дизайн-системи GitLab Pajamas.
- Модульний UI Host для динамічного підключення форм та віджетів розширень.
- Інтерактивне векторне полотно візуалізації графів на базі **SVG** (матриця RTM, дерево залежностей, FSM-схеми, Roadmap).
- Підтримка апаратного zoom/pan, мінімапи, підсвічування шляхів і фільтрації за правами.
- Декларативне вбудовування діаграм Mermaid та прямий експорт графічних звітів у векторні формати SVG і PDF для аудиторів.

---

## 3. Матриця відповідності та перенесення артефактів

| Оригінальний концепт | Новий артефакт у DELMOS | Статус перенесення |
|---|---|---|
| `MODULAR_PROJECT_SYSTEM.md` | `docs/requirements/SYSTEM_REQUIREMENTS.md` | Переноситься з ребрендингом, 13 SHR, 41 SWR |
| `ARCHITECTURE.md` | `docs/architecture/ARCHITECTURE.md` | Повний ребрендинг під DELMOS |
| `DOMAIN_MODEL.md` | `docs/architecture/DOMAIN_MODEL.md` | Додано сутності ресурсів та економіки |
| `WORK_PRODUCTS.md` | `docs/architecture/WORK_PRODUCTS.md` | Включає специфікації та тести SPEC-01..10 |
| `VECTOR_AND_GRAPH_DATA.md` | `docs/architecture/VECTOR_AND_GRAPH_DATA.md` | Повний опис pgvector, CTE CYCLE та SVG |
| `MODULE_CATALOG.md` | `docs/architecture/MODULE_CATALOG.md` | Додано модулі resources та economics |
| `METRICS.md` | `docs/architecture/METRICS.md` | Додано розрахунок показників EVM та PnL |
| `SWOT_ANALYSIS.md` | `docs/architecture/SWOT_ANALYSIS.md` | Стратегічний аналіз архітектури DELMOS |
| `ACCESS_CONTROL.md` | `docs/architecture/ACCESS_CONTROL.md` | Scoped RBAC, перевірка незалежності SoD |
| `PROJECT_MODEL.md` | `docs/architecture/PROJECT_MODEL.md` | Модель проекту, мультипровайдерне сховище |
| `ITEMS_AND_CONTENT.md` | `docs/architecture/ITEMS_AND_CONTENT.md` | Розмежування робіт і артефактів |
| `TAXONOMY.md` | `docs/architecture/TAXONOMY.md` | Поліієрархічні словники та класифікатори |
| `EVENTS_TRIGGERS_RULES.md` | `docs/architecture/EVENTS_TRIGGERS_RULES.md` | Подієвий рушій, черга та планувальник |
| `PROJECT_PLAN_ENGINE.md` | `docs/architecture/PROJECT_PLAN_ENGINE.md` | Дворівневе керування конфігурацією |
| `MODULES.md` | `docs/architecture/MODULES.md` | Контракт маніфестів і UI Host |
| `CONTENT_PROJECT_PATTERNS.md` | `docs/architecture/CONTENT_PROJECT_PATTERNS.md` | Порівняльний аналіз архітектурних патернів |
| `ADMINISTRATOR.md` | `docs/use-cases/ADMINISTRATOR.md` | Сценарії адміністрування системи |
| `PROJECT_MANAGER.md` | `docs/use-cases/PROJECT_MANAGER.md` | Сценарії ведення проекту та економіки |
