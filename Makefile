# Makefile DELMOS: єдиний інтерфейс інженерних операцій (SYSTEM_REQUIREMENTS §2.3).
# Збірка не вбудовує локальні шляхи, імена користувачів чи хостів (-trimpath).

BINARY        := delmos
BUILD_DIR     := bin
PREFIX        ?= /usr/local
CONFIG_DIR    ?= $(PREFIX)/etc/delmos

VERSION       ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo 0.1.0-dev)
COMMIT        ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)
# Час збірки — UTC із дати коміту: локальний час і часовий пояс не потрапляють у бінарник.
BUILD_DATE    ?= $(shell date -u -d "@$$(git log -1 --format=%ct 2>/dev/null || date +%s)" +%Y-%m-%dT%H:%M:%SZ)

PKG           := delmos/internal/version
LDFLAGS       := -s -w -X $(PKG).Version=$(VERSION) -X $(PKG).Commit=$(COMMIT) -X $(PKG).BuildDate=$(BUILD_DATE)
GO_BUILD      := CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)"

PSQL          ?= psql
DB_ADMIN_DSN  ?= postgres:///postgres
DB_NAME       ?= delmos
DB_APP_ROLE   ?= delmos
DB_MIG_ROLE   ?= delmos_migrator
DB_TEST_ROLE  ?= delmos_test

.DEFAULT_GOAL := help
.PHONY: help build run test test-integration fmt fmt-check vet lint docs-links validate migrate db-setup db-test-setup install clean

help: ## Показати доступні цілі
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

build: ## Зібрати бінарник у bin/delmos
	@mkdir -p $(BUILD_DIR)
	$(GO_BUILD) -o $(BUILD_DIR)/$(BINARY) ./cmd/delmos

run: build ## Запустити сервер із локальною конфігурацією
	./$(BUILD_DIR)/$(BINARY) -config ./configs/delmos.yaml

test: ## Модульні тести з перевіркою гонок
	go test -race ./...

test-integration: ## Тести, що потребують PostgreSQL (DELMOS_TEST_DSN, за потреби DELMOS_TEST_TEMPLATE)
	@test -n "$(DELMOS_TEST_DSN)" || { echo "Задайте DELMOS_TEST_DSN (див. make db-test-setup)"; exit 1; }
	go test -race -count=1 ./internal/migrate/... ./internal/auth/... ./internal/server/...

fmt: ## Відформатувати код
	gofmt -w cmd internal

fmt-check: ## Перевірити форматування без запису
	@unformatted=$$(gofmt -l cmd internal); \
	if [ -n "$$unformatted" ]; then echo "Не відформатовано:"; echo "$$unformatted"; exit 1; fi

vet: ## Статичний аналіз стандартним go vet
	go vet ./...

lint: ## golangci-lint (якщо встановлено)
	@if command -v golangci-lint >/dev/null 2>&1; then golangci-lint run; \
	else echo "golangci-lint не встановлено — пропущено (go vet виконується окремо)"; fi

docs-links: ## Перевірити відносні посилання в Markdown-документації
	./scripts/check-doc-links.sh

validate: fmt-check vet lint test docs-links build ## Повна перевірка якості перед Merge Request
	@echo "make validate: усі перевірки пройдено"

migrate: build ## Застосувати міграції схеми та завершити роботу
	./$(BUILD_DIR)/$(BINARY) -config ./configs/delmos.yaml -migrate-only

db-setup: ## Створити ролі, базу та розширення (потрібні права суперкористувача)
	$(PSQL) -v ON_ERROR_STOP=1 -v db_name=$(DB_NAME) -v app_role=$(DB_APP_ROLE) -v migrator_role=$(DB_MIG_ROLE) \
		-d "$(DB_ADMIN_DSN)" -f scripts/sql/bootstrap.sql

db-test-setup: ## Підготувати тестову роль і шаблон бази для інтеграційних тестів
	$(PSQL) -v ON_ERROR_STOP=1 -v test_role=$(DB_TEST_ROLE) \
		-d "$(DB_ADMIN_DSN)" -f scripts/sql/test-template.sql

install: build ## Встановити бінарник і приклад конфігурації за FHS
	install -D -m 0755 $(BUILD_DIR)/$(BINARY) $(DESTDIR)$(PREFIX)/bin/$(BINARY)
	install -D -m 0640 configs/delmos.yaml $(DESTDIR)$(CONFIG_DIR)/delmos.yaml
	install -D -m 0644 deploy/delmos.service $(DESTDIR)/etc/systemd/system/delmos.service

clean: ## Видалити артефакти збірки
	rm -rf $(BUILD_DIR)
