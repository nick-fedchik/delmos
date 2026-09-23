# DELMOS: журнал змін (Changelog)

Формат ґрунтується на [RELEASE-NOTES-TEMPLATE.md](docs/templates/RELEASE-NOTES-TEMPLATE.md).
Версії `v0.y.z` не дають гарантій сумісності контрактів (див. [docs/VERSIONING.md §2.1](docs/VERSIONING.md)).

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
