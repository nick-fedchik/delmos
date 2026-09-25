# DELMOS: локальне середовище розробки (Local Setup)

Дата: 2026-09-23. Статус: перевірена процедура — усі команди нижче виконано й пройдено на робочому середовищі (Ubuntu, Go 1.26, PostgreSQL 18.6, pgvector 0.8.6).
Ліцензія: Apache 2.0.
Контекст: [вимоги до середовища розробки](../../requirements/SYSTEM_REQUIREMENTS.md#2-вимоги-до-середовища-розробки-development-environment), [вимоги до СУБД](../../requirements/SYSTEM_REQUIREMENTS.md#3-вимоги-до-субд-postgresql-database-requirements).

---

## 1. Передумови

* Linux (Ubuntu 24.04/26.04 LTS або сумісний дистрибутив).
* Go 1.25+ (перевірено на 1.26).
* PostgreSQL 16+ (перевірено на 18.6) із розширеннями `pgvector` (0.8+), `ltree`, `pgcrypto`, встановленими на рівні сервера (пакети ОС), — самі розширення ще потрібно **активувати** в конкретній базі (крок 3).
* `make`, `golangci-lint` (опційно для `make lint`; без нього `make validate` пропускає цей крок з попередженням).

## 2. Клонування та збірка

```bash
git clone <URL-репозиторію> delmos
cd delmos
go build ./...
```

## 3. Підготовка PostgreSQL

### 3.1. Чому потрібен суперкористувач саме тут

`pgvector` **не** позначений як `trusted` розширення (`vector.control` без `trusted = true`), тому `CREATE EXTENSION vector` виконує лише суперкористувач СУБД. Робітнича роль застосунку (`delmos`) і роль мігратора (`delmos_migrator`) навмисно **не** мають цього права (SYSTEM_REQUIREMENTS.md §3.3) — розширення активується один раз під час підготовки бази, а не при кожному запуску міграцій.

### 3.2. Основна база застосунку (одноразово, від суперкористувача)

```bash
sudo -u postgres make db-setup
```

Виконує [scripts/sql/bootstrap.sql](../../../scripts/sql/bootstrap.sql): створює ролі `delmos`/`delmos_migrator`, базу `delmos`, активує розширення. Параметри (`DB_NAME`, `DB_APP_ROLE`, `DB_MIG_ROLE`) можна перевизначити через змінні `make`.

### 3.3. Тестовий шаблон для інтеграційних тестів

Інтеграційні тести (`internal/testsupport`) створюють окрему тимчасову базу на кожен тест. Щоб не потребувати прав суперкористувача під час самих тестів, тимчасові бази клонуються з наперед підготовленого шаблону, де розширення вже активовані:

```bash
sudo -u postgres make db-test-setup DB_TEST_ROLE="$(whoami)"
```

Це створює:
* роль PostgreSQL з іменем **вашого поточного ОС-користувача** та правом `CREATEDB` — завдяки цьому локальне з'єднання по UNIX-сокету проходить стандартну `peer`-автентифікацію (`local all all peer` у `pg_hba.conf`) без пароля і без додаткових налаштувань;
* базу-шаблон `delmos_test_template` з активованими `vector`/`ltree`/`pgcrypto`, позначену `IS_TEMPLATE`.

> Якщо потрібна саме спільна (не персональна) назва ролі — наприклад, `delmos_test` для CI-подібного відтворення локально, — додайте маппінг ідентичності в `pg_ident.conf` (`<map_name> <ваш_ОС_користувач> delmos_test`) і відповідний рядок `local all delmos_test peer map=<map_name>` у `pg_hba.conf` **перед** загальним рядком `local all all peer`, потім `sudo systemctl reload postgresql`. Це системна зміна — вносить лише DBA/оператор хоста.

### 3.4. Запуск тестів

```bash
export DELMOS_TEST_DSN="postgres://$(whoami)@/postgres?host=/var/run/postgresql"
export DELMOS_TEST_TEMPLATE=delmos_test_template
make test-integration   # тести, що потребують PostgreSQL
go test -race ./...     # повний набір: і юніт-, і (за наявності DSN) інтеграційні тести
```

Без `DELMOS_TEST_DSN` тести, що потребують БД, автоматично пропускаються (`t.Skip`), а не падають — це не помилка.

## 4. Запуск локального сервера

```bash
make migrate                                      # застосувати міграції схеми
DELMOS_BOOTSTRAP_PASSWORD='<локальний-пароль>' ./bin/delmos -config ./configs/delmos.yaml -bootstrap-admin admin
make dev                                          # локальний сервер у терміналі (PID-файл bin/delmos.pid)
# або фоновий запуск та перезапуск:
make start                                        # запуск у фоні з записом bin/delmos.pid
make status                                       # перевірка стану за PID-файлом та /boot-status
make stop                                         # зупинка процесу за PID-файлом
make restart                                      # перезапуск (stop + start)
```

`make dev` і `make start` автоматично записують PID-файл у `bin/delmos.pid` і встановлюють
`DELMOS_COOKIE_SECURE=false` лише для свого процесу, тому
HttpOnly-сесії працюють через `http://127.0.0.1`. Ця змінна не змінює
`configs/delmos.yaml` і не впливає на `delmos.service`.

Щоб вручну запустити binary без TLS-термінації, встановіть:

```yaml
server:
  cookie_secure: false   # лише для http://localhost; за реверс-проксі з TLS лишати true (типове значення)
```

або встановіть `DELMOS_COOKIE_SECURE=false` в оточенні процесу.

## 5. Web GUI

```bash
make dev         # http://127.0.0.1:10120, Vue SPA та API в одному binary
```

`make build` збирає Vue SPA та вбудовує її у `delmos`. Vite використовується лише
як інструмент складання і не є runtime-процесом. `make dev` не встановлює binary
у `/usr/local` і не використовує `systemctl`.

Мінімальний UI (`v0.6.0`) без дизайн-системи Pajamas — вхід, список/створення проєктів, перегляд плану, CRUD базового Work Product, прив'язка Git-сховища та експорт. Повна дизайн-система та вбудовування активів у Go-бінарник — наступні ітерації.

## 6. Перевірка перед комітом

```bash
make validate
```

Виконує `gofmt`, `go vet`, `golangci-lint` (якщо встановлено), `go test -race` (лише `cmd`/`internal`, без `web/node_modules`), перевірку відносних Markdown-посилань і збірку Go-бінарника без CGO (`-trimpath`, тому в артефакті немає локальних шляхів чи імені хоста). Для `web/` також виконує ESLint, `vue-tsc`, `vitest` та продакшн-збірку (`web/dist`). Інтеграційні тести з реальною БД в `make validate` не входять — запускайте їх окремо (крок 3.4) перед відкриттям Merge Request, якщо змінили `internal/migrate`, `internal/auth` чи `internal/server`.

## 7. Дивіться також

* [CONTRIBUTING_WORKFLOW.md](CONTRIBUTING_WORKFLOW.md) — стиль коду, конвенції комітів, чек-лист Merge Request.
* [scripts/sql/bootstrap.sql](../../../scripts/sql/bootstrap.sql), [scripts/sql/test-template.sql](../../../scripts/sql/test-template.sql) — SQL, що виконує `db-setup`/`db-test-setup`.
