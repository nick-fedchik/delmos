# DELMOS: журнал змін (Changelog)

Формат ґрунтується на [RELEASE-NOTES-TEMPLATE.md](docs/templates/RELEASE-NOTES-TEMPLATE.md).
Версії `v0.y.z` не дають гарантій сумісності контрактів (див. [docs/VERSIONING.md §2.1](docs/VERSIONING.md)).

## 1.0.19 — Unreleased

### Змінено

- ADR-011: серію ISO 21500 (ISO 21502, ISO/TR 21506, ISO 21508, ISO 21511) прийнято за єдину нормативну базу керування проєктами та проєктної економіки. PMBOK, SWEBOK, ASPICE MAN.3 і IAS 38 понижено до довідкових джерел.
- Термін EVM приведено до ДСТУ ISO 21508:2022: «освоєний обсяг» замінено на **«управління здобутою цінністю»** в усій документації (GLOSSARY, METRICS, DOMAIN_MODEL, MODULE_CATALOG, SWOT_ANALYSIS, ADR-006, SHR-13, SWR-39..41, SPEC-03, MEMO-001, MEMO-002, docs/README). Позначення BCWS/BCWP/ACWP збережено довідково.
- GLOSSARY: додано розділ про серію ISO 21500 та термін «ієрархічна структура робіт» (WBS) за ДСТУ ISO 21511:2022.
- Машинні ідентифікатори (таблиці, колонки, ключі метрик) лишаються англомовними за англійським терміном стандарту — мовна редактура відокремлена від зміни контракту даних.

## 1.0.18 — Unreleased

### Додано

- Рушій автоматизації: події, тригери, правила, FSM і планувальник (Етап 4, повна SPEC-04, ADR-007).
  - Міграція `0013_automation_engine.sql`: `core.event_definitions`, `core.event_outbox` (транзакційний outbox), `core.trigger_definitions`, `core.trigger_subscriptions`, `core.event_deliveries` (Lease Fencing), `core.rule_definitions` (типізований DSL `when`/`assert`, без довільного SQL/JS — SWR-26), `core.workflow_definitions` (декларативна FSM), `core.schedule_definitions`/`core.schedule_occurrences`.
  - `internal/automation`: `EmitEvent` (транзакційний outbox), `EnforceRules` (before-фаза MANDATORY_VETO/ADVISORY_WARNING з таблицею рішень not_applicable/passed/violated/evaluation_error), `Engine` з `DispatchPending`/`ClaimAndProcess` (Lease Fencing `FOR UPDATE SKIP LOCKED`, dead-letter після `max_attempts`), `LoadWorkflow`/`ApplyTransition` (FSM-гарди та ефекти `emit_event`), `TickScheduler` (режими `once`/`fixed_rate`/`fixed_delay`/`calendar`; `calendar` — мінімальний 5-польовий cron, свідоме звуження обсягу без повного RFC 5545).
  - `wp.revision_committed` тепер публікується в outbox замість синхронного розрахунку — ембедінги (VEC-01) та каскадне поширення `is_suspect` (GRP-03) виконуються асинхронно через `trigger.core.after_revision_committed` (SWR-36.2).
  - Жорстко закодована перевірка «obsolete WP не редагується» замінена декларативним правилом `rule.core.no_revision_when_obsolete` (MANDATORY_VETO) через `trigger.core.before_wp_transition`.
  - `GET /api/v1/system/automation/status`: адміністративне спостереження (лічильники доставок за статусом, dead-letter список) під новим правом `system.observe` (system.administrator).
  - Диспетчер і планувальник працюють у тому самому процесі, що й HTTP-сервер (`cmd/delmos/main.go`), керовані `configs/delmos.yaml` (`automation.poll_interval`/`lease_seconds`/`batch_size`).
  - Тести: атомарний запис outbox, наскрізна доставка, dead-letter після max_attempts, lease fencing під конкурентним навантаженням без подвійної доставки, таблиця рішень правил, dedup планувальника (`schedule_id`, `occurrence_time_utc`), FSM-гарди.
  - `docs/api/openapi.core.v1.yaml`: `GET /system/automation/status` + схеми `AutomationStatus`/`DeadLetterEntry`.
- MEMO-005: концептуальний меморандум щодо модуля UML (`tooling.uml_modeling`) для документування вимог, сутностей, відносин і структур у нотації OMG UML 2.5.1 — діаграма як проєкція наявних сутностей ядра, без нового джерела істини.

## 1.0.17 — Unreleased

### Додано

- Векторний та графовий шар (Етап 3, ROADMAP.md P3): pgvector, `wp_embeddings`, Change Impact Analysis.
  - Міграція `0012_wp_embeddings.sql`: таблиця `core.wp_embeddings` (`vector(1536)`) з HNSW-індексом косинусної відстані (`vector_cosine_ops`), прив'язана до точної пари `(work_product_id, revision_id, model_name, model_version)`.
  - `internal/project/embedding.go`: локальний провайдер ембедінгів (`local-feature-hash`) без зовнішніх мережевих викликів чи API-ключів (feature hashing мішка слів, L2-нормалізація); реальна модель підключається пізніше через той самий `EmbeddingProvider` без зміни схеми (VEC-03).
  - Вектор рахується синхронно в тій самій транзакції, що й ревізія (`wp.create`/`wp.revise`), лише для `requirement` і `test_spec` — тимчасове відхилення від SWR-36.2 (асинхронно через outbox) до появи черги подій/автоматизації (Етап 4).
  - `GET /api/v1/projects/{project_id}/work-products/{work_product_id}/similar`: гібридний семантичний пошук з фільтрами проєкту і моделі в одному SQL-запиті (VEC-01..03).
  - Каскадне поширення прапорця `is_suspect` (Change Impact Analysis, GRP-03): нова ревізія артефакту позначає підозрілими всі прямі й опосередковані ребра графа trace_links рекурсивним SQL в тій самій транзакції.
  - `GET /api/v1/projects/{project_id}/trace-links` (аудиторський список перегляду з `?suspect=true`) та `POST .../trace-links/{id}/acknowledge` (явна дія рецензента для зняття `is_suspect`).
  - `TestTraverseTraceabilityDetectsCycle` (GRP-01/02), `TestReviseWorkProductPropagatesSuspectFlag` (GRP-03), `TestFindSimilarWorkProductsHybridSearch` (VEC-01/02), `TestFindSimilarWorkProductsIgnoresOtherModelVectors` (VEC-03).

## 1.0.16 — Unreleased

### Виправлено

- Секція Git-сховища більше не справляє враження, ніби прив'язка потрібна для створення артефактів: заголовок позначено як необов'язковий, а опис називає джерелом істини базу даних DELMOS (PROJECT_MODEL.md §2, ADR-005).
- Повідомлення про відсутню прив'язку пояснює, що це нічого не блокує, а контекстна довідка огляду проєкту називає Git експортним каналом, а не сховищем артефактів.

## 1.0.15 — Unreleased

### Додано

- Довідка з підключення Git-сховища (`GitSetupHelp.vue`) у стилі порожнього репозиторію GitLab/GitHub: готові команди з кнопкою копіювання замість опису словами.
  - До прив'язки — два варіанти: локальне bare-сховище на сервері DELMOS (`git init --bare`, або автоматичне створення) та порожнє сховище на зовнішньому хостингу зі зразками HTTPS/SSH-адрес.
  - Після прив'язки — блоки `git clone` і `Надіслати наявну теку` з підставленою адресою та гілкою проєкту.
  - Пояснено модель доступу (DELMOS не зберігає токенів; використовує credential helper або SSH-агент сервера) та формат експорту (`<КОД>.md`, коміт `Export <КОД> from DELMOS`).

## 1.0.14 — Unreleased

### Виправлено

- Верхній бар більше не дублює нижній статус-бар: постійний індикатор «Готово» замінено на алерт, що з'являється лише при деградації або втраті зв'язку та веде до діагностики (CORE_SHELL.md §2).
- Нижній статус-бар показує одну версію продукта (`DELMOS 1.0.14`) замість окремих «UI» і «API»: фронтенд і сервер збираються в один бінарник.
- Абстрактне «Готово» замінено на явне «Компоненти системи справні / деградація (N) / перевірка», а підказка перелічує конкретні компоненти.
- З верхнього бара видалено кнопку-гамбургер і кнопку «?», яка неочевидно закривала праву панель.
- Обидві бічні панелі керуються однаковими здвоєними шевронами внизу панелі, видимими й у згорнутому, й у розгорнутому стані.

## 1.0.13 — Unreleased

### Додано

- Повна оболонка застосунку за [CORE_SHELL.md](docs/architecture/gui/CORE_SHELL.md): верхній бар, ліва навігаційна панель, робоча область, права контекстна панель і нижній статус-бар.
  - Ліва панель (`AppLeftSidebar.vue`) показує групи «Система» та контекст проєкту з кодом і назвою замість UUID; згортається до смуги значків із підказками, на вузькому екрані стає накладною з поверненням фокуса.
  - Права панель (`AppRightSidebar.vue`) має режим офлайн-довідки (для чого, що потрібно до початку, що зробити далі, пошук) без залежності від зовнішнього сервісу та режим діагностики сторінки.
  - Нижній статус-бар (`AppStatusBar.vue`) показує версії UI/API, агрегований стан компонентів, лічильники помилок і попереджень сторінки та копіює безпечний контекст підтримки без токенів і секретів.
  - Скорочення клавіатури `[`, `]`, `?` реєструються централізовано, не спрацьовують у полях вводу; довідка зі скорочень доступна з інтерфейсу (`KeyboardShortcutsModal.vue`).
- Значки Pajamas зі спрайта `@gitlab/svgs` (`PIcon.vue`) замість емодзі; спрайт вбудовано в бінарник і доступний офлайн.
- Діагностика сторінки (`stores/diagnostics.ts`): збої API потрапляють до статус-бара й правої панелі; зберігаються лише безпечні дані (метод, шлях, статус, код, повідомлення для користувача).

### Змінено

- Опитування `GET /api/v1/system/boot-status` винесено в єдиний композабл `useSystemHealth` із захистом від паралельних повторів; верхній бар і статус-бар читають спільний стан замість власних таймерів.
- Прокручується робоча область, а не весь viewport разом з панелями; додано посилання «Перейти до вмісту» та перенесення фокуса на основний вміст при зміні маршруту.

## 1.0.12 — Unreleased

### Додано

- Повна підтримка життєвого циклу планування та операційних реєстрів за CORE-CONTRACT-002/003:
  - Реалізовано команду `plan.apply` (`POST /api/v1/projects/{project_id}/plan/apply`) для введення затвердженого плану в дію з інкрементом покоління конфігурації (`config_generation`) та проєкцією фаз і віх.
  - Додано міграцію `0011_plan_apply_and_operational_registers.sql`: таблиці `core.project_plan_applications`, `core.project_phases`, `core.project_milestones`, `core.project_stakeholders`, `core.project_risks`.
  - Реалізовано HTTP API та екранні представлення для реєстру стейкхолдерів (`ProjectStakeholdersView.vue`) та реєстру ризиків (`ProjectRisksView.vue`).
  - Впроваджено уніфікований лейаут додатку (APMS Layout & Core Shell): розгортана ліва навігаційна панель (Sidebar) із контекстом проєкту, індикатор Boot Health у шапці та зручне перемикання між оглядом, планом, стейкхолдерами та ризиками.

## 1.0.11 — Unreleased

### Додано

- Повна синхронізація проєктних прав (Scoped RBAC) між сервером та Web GUI: `GET /projects/{id}`, `GET /plan`, `GET /work-products/{id}` повертають список дозволів актора у проєкті (`permissions`), що автоматично розблоковує форми редагування та створення для власників проєктів.
- Поліпшено взаємодію на екранах `ProjectDetailView` та `WorkProductDetailView`: контекстні кнопки створення, адаптивне управління сховищем та ревізіями.

## 1.0.10 — Unreleased

### Додано

- Повний редизайн Web GUI на основі рекомендацій GitLab Pajamas та APMS:
  - Впроваджено ієрархічні хлібні крихти (Breadcrumbs) для всіх проєктних та артефактних екранів.
  - Оновлено картки проєктів та детальний екран проєкту (`ProjectDetailView.vue`): пласка структура без надлишкової вкладеності, статус-бейджики за стандартами Pajamas, інтеграція з Git-сховищем, таблиця Work Products.
  - Оновлено редактор плану (`ProjectPlanView.vue`) та редактор артефакту (`WorkProductDetailView.vue`): динамічний підрахунок незбережених змін (`isDirty`), виділення кнопки збереження amber-кольором (`.btn-unsaved`) та бейдж «Є незбережені зміни».
  - Покращено семантичну палітру дизайн-токенів у `style.css` (surface, text, border, status, 8px spacing grid, radii, elevation).

## 1.0.9 — Unreleased

### Виправлено

- Відновлення CSRF-токена при оновленні сторінки в браузері: `GET /api/v1/auth/session` передає валідний токен у заголовку `X-CSRF-Token`, що запобігає помилці розбіжності токена після перевантаження сторінки.

## 1.0.8 — Unreleased

### Додано

- Оновлено інтерфейс сторінки проєктів за канонічним паттерном GitLab Pajamas Empty State: додано інженерну ілюстрацію, зрозумілий опис без сирих специфікаційних посилань та пряму кнопку створення.
- Додано панель адміністративного налаштування ролей з можливістю самопризначення ролі Project Manager для швидкого старту.
- Оновлено верхній заголовок додатка: додано аватар користувача, бейдж поточної ролі та хлібні крихти навігації.

## 1.0.7 — Unreleased

### Додано

- Підтримка PID-файлу (`-pid-file`, `server.pid_file`, `DELMOS_PID_FILE`) для надійного локального контролю процесу `delmos`.
- Команди `make start`, `make stop`, `make restart` та `make status` для детермінованого перезапуску локального бекенду.

## 1.0.6 — Unreleased

### Додано

- Діагностичний статус-бар Boot Health на екрані входу з колірними індикаторами стану компонентів (ядро, PostgreSQL, міграції, Git-сховище).
- Публічний діагностичний ендпоінт `GET /api/v1/system/boot-status` (та `/boot-status`) для моніторингу завантаження й інтеграцій системи.

## 1.0.5 — Unreleased

### Виправлено

- Уніфіковано відображення SemVer у Web GUI з версією продукту; номер більше не дублюється вручну в компоненті.
- Повідомлення про недоступність автентифікації пояснює користувачеві, що облікові дані не перевірялися, без деталей реалізації чи хостингу.
- Vue SPA вбудовується у binary `delmos` і віддається разом із API через один Gin HTTP listener.

## v1.0.0 — 2026-09-23

### Основні зміни

Перший стабільний реліз: наскрізний сценарій MVP від входу адміністратора до
експорту артефакту в Git-сховище через REST API та мінімальний Web GUI.
REST-контракт `docs/api/openapi.v1.yaml` заморожено й покрито контрактними
тестами (`internal/server/contract_test.go`).

### Додано

- Локальний вхід/вихід, HttpOnly+Secure+SameSite=Strict сесії, CSRF, rate limiting,
  мінімальний Scoped RBAC (`system.administrator`, `project.manager`, `project.owner`).
- CRUD проєкту з атомарним обов'язковим `Generic Project Plan` (`PLAN-001`).
- CRUD базового Work Product (6 типів ядра) з незмінними ревізіями, `payload_hash`/`content_hash`
  та оптимістичним блокуванням (`row_version`).
- `RepositoryProvider` для простого Git (локальний bare або HTTPS): прив'язка проєкту,
  ручний експорт ревізії у файл через `go-git`.
- Мінімальний Web GUI (Vue 3 + Vite + TypeScript, без дизайн-системи Pajamas).
- Заморожений REST-контракт `docs/api/openapi.v1.yaml` з автоматизованими контрактними тестами.

### Відомі обмеження v1.0.0

- Review/Approval workflow (`wp.submit`/`wp.review`/`wp.approve`, `plan.apply`) не реалізовано —
  цільовий контракт лишається в `docs/api/openapi.core.v1.yaml` для `v1.x`.
- Композитні специфікації, векторний/графовий пошук, події/правила/планувальник,
  проєктна економіка, адаптери GitHub/GitLab/Azure DevOps/Bitbucket/Forgejo — поза межами `v1.0.0`.
- SSH-автентифікація `RepositoryProvider` не реалізована (лише локальний bare і HTTPS).

### Нотатки з міграції

Перше розгортання: виконати `docs/guides/developer/LOCAL_SETUP.md` (розробка) або
`docs/operations/RUNBOOK.md` + `docs/requirements/SYSTEM_REQUIREMENTS.md` (продакшн).
Попередніх версій зі зворотною сумісністю немає.

---

## v0.1.0 … v0.6.0 — 2026-09-23 (передрелізні ітерації)

Опубліковані послідовно в межах підготовки `v1.0.0`; без окремих гарантій сумісності
(див. [docs/VERSIONING.md §2.1](docs/VERSIONING.md)).

- **v0.1.0** — фундамент рантайму: Go-модуль, `Makefile`/`make validate`, конфігураційний шар,
  forward-only міграції з advisory lock і SHA-256, HTTP-скелет (`/healthz`, `/readyz`).
- **v0.2.0** — автентифікація адміністратора, сесії, CSRF, rate limiting, мінімальний RBAC.
- **v0.3.0** — CRUD проєкту та атомарний `Generic Project Plan` (`PLAN-001`).
- **v0.4.0** — CRUD базового Work Product з незмінними ревізіями.
- **v0.5.0** — просте Git-сховище через `RepositoryProvider`.
- **v0.6.0** — мінімальний Web GUI на Vue 3 + Vite.
