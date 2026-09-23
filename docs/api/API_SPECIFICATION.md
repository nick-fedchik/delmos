# DELMOS: специфікація REST API

Дата: 2026-09-23. Статус: цільовий контракт REST API (шаблон структури, ендпоінти уточнюються під час реалізації).
Ліцензія: Apache 2.0.
Формат: цільова специфікація; для першого зрізу машинний контракт — [openapi.core.v1.yaml](openapi.core.v1.yaml), решта ендпоінтів цієї сторінки ще не формалізована.
Контекст: [api/README.md](README.md), [ACCESS_CONTROL.md](../architecture/ACCESS_CONTROL.md).

---

## 1. Загальний огляд

Перший зріз (ролі, Work Product, Review/Approval, план і аудит) визначено в [CORE-CONTRACT-001](../specifications/CORE-CONTRACT-001-ROLE-WORKFLOW.md) та його [OpenAPI](openapi.core.v1.yaml). Нижченаведені ресурси поза цим зрізом є напрямком розвитку, не заявою про готовий сервер чи повну машинну специфікацію.

### 1.1. Базова адреса

```text
https://<host>/api/v1
```

### 1.2. Автентифікація

Кожен запит супроводжується серверною сесією (HttpOnly cookie) або, для технічних інтеграцій, короткостроковим токеном `ServicePrincipal` у заголовку `Authorization: Bearer <token>`. Детальніше — [ACCESS_CONTROL.md, розділ 2](../architecture/ACCESS_CONTROL.md#2-ідентичності-сесії-та-автентифікація).

### 1.3. Тип вмісту

Усі запити та відповіді використовують `Content-Type: application/json; charset=utf-8`.

### 1.4. Пагінація

```text
GET /api/v1/projects/{project_id}/work-products?page=1&limit=50&sort=updated_at&order=desc
```

Відповідь містить обгортку:

```json
{
  "items": [],
  "pagination": {
    "page": 1,
    "limit": 50,
    "total_items": 0,
    "total_pages": 0
  }
}
```

### 1.5. Стандартний формат помилки

```json
{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "Посилання на неіснуючий Work Product",
    "details": [
      { "field": "occurrences[2].target_id", "issue": "not_found" }
    ]
  }
}
```

| HTTP-статус | Значення |
| --- | --- |
| `400 Bad Request` | Некоректний синтаксис запиту |
| `401 Unauthorized` | Відсутня або недійсна сесія/токен |
| `403 Forbidden` | Автентифіковано, але недостатньо прав (RBAC/ABAC) |
| `404 Not Found` | Ресурс не існує або недоступний у поточній області видимості |
| `409 Conflict` | Конфлікт версій (наприклад, спроба редагувати застарілу ревізію) |
| `422 Unprocessable Entity` | Порушення бізнес-правила (наприклад, вердикт `MANDATORY_VETO`) |

## 2. Проєкти (Projects)

### 2.1. Створити проєкт

```text
POST /api/v1/programmes/{programme_id}/projects
```

Тіло запиту:

```json
{
  "name": "Назва проєкту",
  "description": "Короткий опис мети",
  "repository_provider": "internal"
}
```

Відповідь `201 Created` повертає створений проєкт разом з ідентифікатором автоматично згенерованого документа `Generic Project Plan` (`PLAN-001`). Побічний ефект: атомарне створення `ProjectPlanBinding` в тій самій транзакції.

### 2.2. Отримати проєкт

```text
GET /api/v1/projects/{project_id}
```

## 3. Артефакти (Work Products)

### 3.1. Список артефактів проєкту

```text
GET /api/v1/projects/{project_id}/work-products?type=requirement&status=in_review
```

### 3.2. Отримати деталі артефакту

```text
GET /api/v1/projects/{project_id}/work-products/{work_product_id}
```

Відповідь `200 OK` містить поточну ревізію, метадані, зв'язки трасованості (`trace_links`) та історію попередніх ревізій.

### 3.3. Створити нову ревізію артефакту

```text
POST /api/v1/projects/{project_id}/work-products/{work_product_id}/revisions
```

Побічні ефекти: обчислення `payload_hash`, запис події в `event_outbox` для подальшої асинхронної обробки (наприклад, перерахунок ембедінгу `pgvector`).

## 4. Композитні специфікації (Specifications)

### 4.1. Створити композитну специфікацію

```text
POST /api/v1/projects/{project_id}/specifications
```

Див. повну схему в [SPEC-01: рушій композитних специфікацій](../specifications/SPEC-01-COMPOSITE-SPECIFICATIONS-ENGINE.md).

## 5. Зв'язки трасованості (Trace Links)

### 5.1. Отримати матрицю трасованості

```text
GET /api/v1/projects/{project_id}/trace-links?from={work_product_id}&direction=downstream&depth=5
```

Реалізація спирається на рекурсивний обхід графа, описаний у [VECTOR_AND_GRAPH_DATA.md](../architecture/VECTOR_AND_GRAPH_DATA.md).

## 6. Дивіться також

* [api/README.md](README.md) — поверхня API, життєвий цикл контракту, правила для нових ендпоінтів.
* [architecture/ACCESS_CONTROL.md](../architecture/ACCESS_CONTROL.md) — модель авторизації, застосовна до кожного запиту.
