# DELMOS: настанови і критерії готовності до активного кодування

Дата: 2026-09-23. Статус: робочий чек-лист планування; підтвердження в таблиці ще не отримані.
Ліцензія: Apache 2.0.
Контекст: [архітектура](ARCHITECTURE.md), [межі реалізації](IMPLEMENTATION_GUARDRAILS.md), [архітектурні шаблони](IMPLEMENTATION_PATTERNS.md), [вимоги](../requirements/README.md), [тестова стратегія](../testing/TEST_STRATEGY.md).

---

## Що вже є, а чого бракує

| Напрям | Наявна база | Передкодингова робота / доказ |
| --- | --- | --- |
| Продуктовий фокус | [VISION](../guides/VISION.md), [MEMO-004](../memos/MEMO-004-product-concept-validation-strategy.md), SHR/SWR, дорожня карта | Один початковий сценарій і ролі, критерій успіху та 3-5 ризикованих припущень з дешевою перевіркою, а не одночасна реалізація всіх галузей. |
| Межі модулів і даних | [ARCHITECTURE](ARCHITECTURE.md), [MODULES](MODULES.md), [guardrails](IMPLEMENTATION_GUARDRAILS.md) | Для першого вертикального зрізу назвати власника кожного запису, команду, транзакцію, read model і допустиму залежність пакетів. |
| API й події | [CORE-CONTRACT-001](../specifications/CORE-CONTRACT-001-ROLE-WORKFLOW.md), [OpenAPI](../api/openapi.core.v1.yaml), [схема подій](../api/events.core.v1.schema.json) | Машинозчитувані цільові контракти першого зрізу підготовлено; перевірено структуру YAML/JSON Schema, але контрактні тести проти сервера й гарантована поведінка ідемпотентності лишаються до виконання. |
| Міграції та життєвий цикл даних | [вимоги до БД](../requirements/SYSTEM_REQUIREMENTS.md), [ADR-010](decisions/ADR-010-audit-evidence-and-outbox-retention.md), [upgrade/rollback](../operations/UPGRADE_AND_ROLLBACK.md) | FK без каскаду вже визначені; до першої міграції перевірити поведінку FK й архівування, а до production — строк retention, legal hold, backup і відновлення. |
| Безпека і приватність | [ACCESS_CONTROL](ACCESS_CONTROL.md), [SECURITY_SPECIFICATION](../security/SECURITY_SPECIFICATION.md), [privacy](../compliance/domains/PERSONAL_DATA_PROTECTION.md) | Threat model (актори, активи, trust boundaries, abuse cases), класифікація полів/експортів, способи тестування SoD, CSRF, секретів і redaction. Для даних фізичних осіб потрібен власник retention і правового аналізу. |
| Якість і продуктивність | [TEST_STRATEGY](../testing/TEST_STRATEGY.md), [метрики](METRICS.md) | Реальні fixtures й перевірка конкурентності, повторів подій, розміру графа, векторної вибірки й permission filtering; бюджети затримки та розмірів задавати за вимірюванням, не вгадувати. |
| Web GUI / i18n | [GUI-реєстр](gui/README.md), [локалізація](gui/LOCALIZATION.md) | Узгодити стартові локалі й перевірити перший guided-flow з новачком; форма не обходить серверний контракт. |
| OSS і постачання | [CONTRIBUTING](../../CONTRIBUTING.md), [SECURITY](../../SECURITY.md), [GOVERNANCE](../../GOVERNANCE.md) | Перед публічним анонсом: ліцензії залежностей/активів, dependency update/scanning, SBOM-план, відтворювана збірка, контакт приватних security reports. |

## Мінімальний gate до першої доменної функції

1. **Один доказовий вертикальний зріз.** Обрати, наприклад, `Project + PLAN-001 → WP draft → revision → trace → review → baseline` із дозволами; зафіксувати, які кроки справді входять у P0/P1, а які залишаються наступною ітерацією. Актори й негативні сценарії наведені у [каталозі базових ролей](../use-cases/README.md), вимоги до доступу — [SHR-14](../requirements/stakeholder/SHR-14-core-access-and-assurance.md) / [SWR-42..48](../requirements/software/SWR-10-core-access-assurance.md), чернетка перевірки — [UAT-004](../uat/UAT-004-core-roles-review-approval.md). Для кожного кроку потрібні відповідна SHR/SWR, сценарій перевірки й спостережуваний результат; use cases не замінюють вимоги. Вимоги не змінюються без оновлення traceability.
2. **Межі та рішення.** Для таблиць першого зрізу є власник, команди й інваріанти; питання, що лишилися, зафіксовано в [IMPLEMENTATION_GUARDRAILS.md](IMPLEMENTATION_GUARDRAILS.md). [ADR-009](decisions/ADR-009-core-review-and-approval-policy.md) і [ADR-010](decisions/ADR-010-audit-evidence-and-outbox-retention.md) визначають approval і retention; їх потрібно перевірити тестами **до** відповідних міграцій. Нові невирішені протоколозмінні питання оформлюються Proposed ADR, не Accepted за замовчуванням.
3. **Контракти.** [Перший Core-зріз](../specifications/CORE-CONTRACT-001-ROLE-WORKFLOW.md) має OpenAPI, JSON Schema подій, статуси помилок і негативні fixture-кандидати; перед виконанням `make validate` їх ще потрібно перетворити на контрактні тести сервера (валідні/недійсні payload, конкуренція, повтори, два проєкти). Не вимагати всю майбутню OpenAPI-поверхню до першого коду.
4. **Security baseline.** Провести невеликий threat-model review для першого зрізу; сервер перевіряє scope, SoD, CSRF та `classification`, а тести негативних сценаріїв не розкривають сусідній проєкт або `restricted` поля.
5. **Відтворювана перевірка.** Налаштувати реальні команди build/test/lint/doc links/міграційний smoke у репозиторії. Згаданий у документах `make validate` є **вимогою до майбутнього репозиторію**, доки реалізовану команду не перевірено. Тестова БД із початковим і вже мігрованим станом, rollback транзакції й запуск під непривілейованою роллю мають проходити автоматично.

Gate є критерієм **для першого вертикального зрізу**, а не забороною експериментувати прототипами. Якщо рішення неможливо перевірити дешево, обмежити scope і записати припущення; не декларувати платформену гарантію з одного PoC.

## До першого публічного preview

Додатково потрібні: демонстраційний guided-flow без приватних даних, перевірений backup/restore на реальній схемі, матриця сумісності (Go/PostgreSQL/pgvector/браузери/версії API), UX-тест на нових користувачах, мінімальні SLO/діагностика, документація інсталяції й чесна позначка `alpha`. Підключити перевірку залежностей, ліцензій і секретів, опублікувати security contact; довести на прикладі, що зовнішній provider недоступний, а локальне ядро й офлайн-довідка лишаються доступними. Не заявляти сертифікацію чи нормативну відповідність тільки через існування шаблонів у [доменно-комплаєнсному реєстрі](../compliance/domains/README.md).

## Що свідомо відкласти

Повне покриття 21 галузевого домену, універсальний workflow builder, зовнішні мікросервіси, невиміряні проєкції та масштабні AI-функції не є передумовами початку. Після базового зрізу повторити ризикові експерименти й вибір пріоритетів за [MEMO-004](../memos/MEMO-004-product-concept-validation-strategy.md). Для кожного нового модуля перезапускати gate тільки в частині його власних контрактів, прав, даних та UX, а не переписувати всі документи платформи.