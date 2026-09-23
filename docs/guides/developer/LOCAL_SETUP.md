# DELMOS: локальне середовище розробки (Local Setup)

Дата: 2026-09-23. Статус: цільова інструкція для майбутнього вихідного коду, не перевірена процедура запуску.
Ліцензія: Apache 2.0.
Контекст: [вимоги до середовища розробки](../../requirements/SYSTEM_REQUIREMENTS.md#2-вимоги-до-середовища-розробки-development-environment).

---

Зараз DELMOS містить документацію, але не містить `go.mod`, `web/`, `cmd/delmos`, `configs/delmos.dev.yaml` і Makefile. Команди нижче задають очікуваний шлях розробника **після** появи відповідних файлів; наразі їх не можна виконати в цьому каталозі.

## 1. Передумови

* Ubuntu 24.04 LTS / 26.04 LTS (або сумісний дистрибутив Linux).
* Go 1.22+ (рекомендовано 1.26).
* Node.js LTS для інструментарію фронтенду (Vite 6.x, Vue 3.5+).
* Локально встановлений PostgreSQL 16+ (рекомендовано 18.6) із розширеннями `pgvector` та `ltree`.

## 2. Клонування та збірка backend

```bash
git clone <URL-репозиторію> delmos
cd delmos
go build ./...
```

## 3. Підготовка бази даних для розробки

```bash
sudo -u postgres createuser --no-superuser --no-createdb --no-createrole delmos_dev
sudo -u postgres createdb --owner=delmos_dev delmos_dev
sudo -u postgres psql -d delmos_dev -c "CREATE EXTENSION IF NOT EXISTS vector;"
sudo -u postgres psql -d delmos_dev -c "CREATE EXTENSION IF NOT EXISTS ltree;"
```

## 4. Запуск локального сервера

```bash
go run ./cmd/delmos --config ./configs/delmos.dev.yaml
```

## 5. Frontend (Vue 3 / Vite)

```bash
cd web
npm install
npm run dev
```

## 6. Перевірка перед комітом

```bash
make validate
```

Після реалізації команда повинна виконувати лінтери, перевірку типів, юніт-тести backend і frontend та перевірку цілісності документації (посилання, заголовки H1, закриті блоки коду).

## 7. Дивіться також

* [CONTRIBUTING_WORKFLOW.md](CONTRIBUTING_WORKFLOW.md) — стиль коду, конвенції комітів, чек-лист Merge Request.
