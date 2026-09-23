# DELMOS: REST API

Дата: 2026-09-23. Статус: нормативний реєстр документації API (цільовий контракт, підлягає уточненню під час реалізації P1–P7).
Ліцензія: Apache 2.0.
Контекст: [ядро та архітектура](../architecture/ARCHITECTURE.md), [дорожня карта](../architecture/ROADMAP.md).

---

## 1. Поверхня API

```mermaid
flowchart LR
    UI["Vue 3 SPA"] --> REST["REST API<br/>/api/v1"]
    CLI["CLI-клієнт"] --> REST
    Smoke["Наскрізні smoke-тести"] --> REST
    REST --> Auth["Авторизація (RBAC/ABAC)"]
    REST --> Domain["Доменні сервіси"]
    REST --> DB["PostgreSQL"]
```

## 2. Документи

| Документ | Призначення |
| --- | --- |
| [API_SPECIFICATION.md](API_SPECIFICATION.md) | Людиночитний опис ресурсів REST API з прикладами запитів/відповідей |
| [openapi.core.v1.yaml](openapi.core.v1.yaml) | Машинозчитуваний OpenAPI 3.1 першого зрізу ядра: ролі, проєкт, ревізії, Review/Approval, plan.apply та аудит (цільовий, не реалізовано) |
| [events.core.v1.schema.json](events.core.v1.schema.json) | JSON Schema Draft 2020-12 для конвертів подій Core без сирого вмісту артефактів |
| [CORE-CONTRACT-001](../specifications/CORE-CONTRACT-001-ROLE-WORKFLOW.md) | Постумови команд, негативні сценарії та межі транзакцій першого зрізу |

## 3. Життєвий цикл контракту

```mermaid
sequenceDiagram
    participant Dev as Розробник
    participant Spec as API_SPECIFICATION.md
    participant Code as Реалізація (Go handlers)
    participant Test as Автоматизовані тести

    Dev->>Spec: Узгодити ендпоінт у OpenAPI і людиночитному описі
    Dev->>Code: Реалізувати обробник відповідно до опису
    Dev->>Test: Додати/оновити integration-тест
    Test-->>Dev: make validate проходить
```

## 4. Правила для нових ендпоінтів

* Для першого зрізу канонічний машинний контракт — [openapi.core.v1.yaml](openapi.core.v1.yaml), людиночитна поведінка — [CORE-CONTRACT-001](../specifications/CORE-CONTRACT-001-ROLE-WORKFLOW.md); подальші ендпоінти до або разом із реалізацією вносяться в обидва представлення.
* Усі ендпоінти живуть під префіксом `/api/v1`; зміна формату відповіді в межах `v1` є **зворотносумісною** (додавання полів дозволене, видалення чи перейменування — ні).
* Кожен ендпоінт, що змінює стан, перевіряє права доступу через RBAC/ABAC (див. [ACCESS_CONTROL.md](../architecture/ACCESS_CONTROL.md)) до виконання будь-якої бізнес-логіки.
* Помилки повертаються в єдиному узгодженому форматі (розділ 3 [API_SPECIFICATION.md](API_SPECIFICATION.md)).

## 5. Пов'язані документи

* [architecture/ACCESS_CONTROL.md](../architecture/ACCESS_CONTROL.md) — модель авторизації, що застосовується до кожного запиту.
* [specifications/SPEC-02-REPOSITORY-PROVIDER-INTERFACE.md](../specifications/SPEC-02-REPOSITORY-PROVIDER-INTERFACE.md) — контракт постачальників сховищ, окремий від публічного REST API.
