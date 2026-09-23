# CORE-CONTRACT-001: Контракт першого зрізу ядра з ролями та погодженням

Дата: 2026-09-23. Статус: цільова специфікація; сервер і тести ще не реалізовані.
Ліцензія: Apache 2.0.
Контекст: [SHR-14](../requirements/stakeholder/SHR-14-core-access-and-assurance.md), [SWR-42..48](../requirements/software/SWR-10-core-access-assurance.md), [ADR-009](../architecture/decisions/ADR-009-core-review-and-approval-policy.md), [ADR-010](../architecture/decisions/ADR-010-audit-evidence-and-outbox-retention.md), [UAT-004](../uat/UAT-004-core-roles-review-approval.md), [OpenAPI Core](../api/openapi.core.v1.yaml).

---

## Межа першого зрізу

System Administrator (або уповноважений власник `access.grant`) надає ролі в проєкті; PM створює Project разом із `PLAN-001`; Engineer створює WP типу `requirement`, нову ревізію і подає її; Reviewer фіксує позитивний Review; Approver незалежно погоджує точний хеш; Auditor і Viewer читають лише дозволені результати. Створення міжпроєктних графів, бейзлайнів, модульних стандартів, галузевих полів, зовнішнього Git і UI-асистента не входить у перший контракт. Застосування вже погодженого плану перевіряється окремою командою `plan.apply` в рамках ядра, без активації модулів.

## Команди, права та атомарність

| Операція | Право / роль | Транзакційна постумова |
| --- | --- | --- |
| Створити Project | `scopes.manage` у Programme/System | Одночасно створено Project, WP `PLAN-001` з r1 і `ProjectPlanBinding`; невдалий крок відкочує всю операцію. |
| Надати RoleBinding у Project | `access.grant` у допустимому scope | Користувач/роль/scope/термін/обґрунтування записані разом із append-only audit event; делегування вище стелі заборонене. |
| Відкликати RoleBinding у Project | `access.revoke` у допустимому scope | Прив'язка втрачає чинність для наступних запитів і фонових дій; зміна записана разом із append-only audit event. |
| Створити WP і ревізію | `wp.create` у Project | Стан `draft`, серверний `revision_id`, хеш і `row_version`, outbox та audit event у тій же транзакції. |
| Створити наступну ревізію | `wp.edit` | Попередня ревізія незмінна; атомарно зіставлено `expected_row_version`; повернуто нову ревізію та версію. |
| Подати ревізію | `wp.submit` | Перевірено ревізію/хеш і валідність метаданих; стан `in_review` і ReviewRequest з призначеннями. |
| Фіксувати Review / повернути на зміни | `wp.review` / `wp.request_changes` і призначення | Позитивний Review не змінює стан; мотивований запит змін повертає WP до `draft`, зберігаючи історію. |
| Погодити ревізію | `wp.approve` і призначення | Сервер перевіряє позитивний Review, кворум, незалежність, точний хеш і `row_version`; записує незмінний Approval і стан `approved`. |
| Застосувати погоджений план | `plan.apply` | Лише після `plan.approve` оновлено `effective_plan_revision_id` і `config_generation`; дубль не застосовує план повторно. |
| Читати ревізію / аудит | `wp.read` плюс `wp.history.read` / `audit.read` | Відповідь містить лише дозволені записи з точним revision ref; відсутність права в іншому проєкті не розкриває назву об'єкта. |

Кожен запис суттєвої бізнес-зміни створює append-only `audit_events`, а подія для асинхронної обробки додається до outbox у тій самій транзакції, коли вона потрібна. Не викликати зовнішній сервіс під блокуванням БД. Лише `audit_events` є канонічним доказом рішення, технічні доставки можуть очищатися згідно з [ADR-010](../architecture/decisions/ADR-010-audit-evidence-and-outbox-retention.md).

## Протокол HTTP

Канонічний машинозчитуваний фрагмент — [openapi.core.v1.yaml](../api/openapi.core.v1.yaml); конверт подій — [events.core.v1.schema.json](../api/events.core.v1.schema.json) із полями із [SPEC-04](SPEC-04-EVENTS-TRIGGERS-RULES-ENGINE.md). Базовий шлях `/api/v1`; cookie-сесія та CSRF-заголовок для змін. Кожна команда запису отримує `Idempotency-Key` у межах актора/операції та однаковий request hash: повтор повертає той самий результат, той самий ключ з іншими даними — `409`. Час життя ключа/допустиме вікно повтору визначаються до кодування сервісу, а не припускаються клієнтом. `expected_row_version` разом із `revision_id` і `payload_hash` потрібні для переходів WP/плану. Серверні хеші й `row_version` не обчислюються UI.

| HTTP | Контракт поведінки |
| --- | --- |
| `POST /programmes/{programme_id}/projects` | Створює Project і первинний план атомарно. |
| `POST /projects/{project_id}/role-bindings` | Надає RoleBinding без самопідвищення привілеїв. |
| `DELETE /projects/{project_id}/role-bindings/{binding_id}` | Відкликає прив'язку, не чекаючи завершення сесії користувача. |
| `POST /projects/{project_id}/work-products` | Створює базовий WP і першу ревізію. |
| `POST /projects/{project_id}/work-products/{work_product_id}/revisions` | Створює наступну immutable ревізію з оптимістичною перевіркою. |
| `POST /projects/{project_id}/work-products/{work_product_id}/submit` | Створює ReviewRequest для точної ревізії. |
| `POST /projects/{project_id}/work-products/{work_product_id}/reviews` | Фіксує позитивний Review, не Approved. |
| `POST /projects/{project_id}/work-products/{work_product_id}/request-changes` | Повертає на `draft` з причиною й аудиторським записом. |
| `POST /projects/{project_id}/work-products/{work_product_id}/approvals` | Фіксує незалежний Approval; для WP типу `plan` перевіряє `plan.approve`. |
| `POST /projects/{project_id}/plan/apply` | Застосовує окремо погоджену ревізію PLAN-001. |
| `GET /projects/{project_id}/work-products/{work_product_id}/revisions/{revision_id}` | Повертає лише дозволену конкретну ревізію. |
| `GET /projects/{project_id}/audit-events` | Повертає лише дозволені санітизовані докази. |

Відомий, але невидимий об'єкт і невідомий об'єкт повертають однакову `404` без назв; відсутня сесія — `401`, відсутній дозвіл на видиму дію — `403`, stale version/повтор із зміненим payload — `409`, порушення інваріанта для авторизованого актора (SoD, кворум, стан) — `422`. `error.code` стабільний для клієнта, а пояснення безпечне для поточного актора. У тілі помилки не повертати сирі логи або секрети.

## Перевірні приклади і заборони

1. Два Project A/B; Viewer з доступом тільки до A не отримує дані B ні по ID, ні в списку/пошуку, навіть при прямому запиті.
2. Спроба `wp.approve` автором із додатковою роллю Approver — `422` без Approval; позитивний Review тієї ж особи-автора також не зараховується.
3. Два конкурентні запити до r1 з однаковим `expected_row_version`: лише один створює r2; другий повертає `409` без втрати r1.
4. Повтор тієї ж команди з тим самим `Idempotency-Key` не додає нового підпису чи ревізії; інший request body з тим самим ключем — `409`.
5. Дія Auditor на зміну/експорт без додаткового права — `403`; читання журналу не повертає raw payload чи токенів.
6. Створення Project із помилкою при створенні PLAN-001 не залишає проєкт-сироту; після успіху нова ревізія плану застосовується тільки після окремого Review/Approval та `plan.apply`.

Ці приклади є контрактними fixture-кандидатами для [UAT-004](../uat/UAT-004-core-roles-review-approval.md) і [тестової стратегії](../testing/TEST_STRATEGY.md), а не звітом про їхній успішний запуск.