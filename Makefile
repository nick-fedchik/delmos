# Makefile DELMOS: єдиний інтерфейс інженерних операцій (SYSTEM_REQUIREMENTS §2.3).
# Збірка не вбудовує локальні шляхи, імена користувачів чи хостів (-trimpath).

BINARY        := delmos
BUILD_DIR     := bin
PREFIX        ?= /usr/local
CONFIG_DIR    ?= $(PREFIX)/etc/delmos
WEB_ASSET_DIR := internal/webassets/dist
PID_FILE      ?= $(BUILD_DIR)/delmos.pid

VERSION       ?= 1.0.24-dev
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
.PHONY: help build run dev start stop restart status test test-integration fmt fmt-check vet lint docs-links validate migrate db-setup db-test-setup install clean web-install web-lint web-typecheck web-test web-build web-embed

help: ## Показати доступні цілі
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

build: web-embed ## Зібрати бінарник DELMOS із вбудованим Web GUI
	@mkdir -p $(BUILD_DIR)
	$(GO_BUILD) -o $(BUILD_DIR)/$(BINARY) ./cmd/delmos

run: build ## Запустити сервер із локальною конфігурацією та pid-файлом
	./$(BUILD_DIR)/$(BINARY) -config ./configs/delmos.yaml -pid-file $(PID_FILE)

dev: build ## Запустити локальний DELMOS у терміналі (з pid-файлом bin/delmos.pid)
	DELMOS_COOKIE_SECURE=false ./$(BUILD_DIR)/$(BINARY) -config ./configs/delmos.yaml -pid-file $(PID_FILE)

start: build ## Запустити локальний DELMOS у фоні з записом pid-файлу
	@if [ -f $(PID_FILE) ]; then \
		pid=$$(cat $(PID_FILE) 2>/dev/null); \
		if [ -n "$$pid" ] && kill -0 $$pid 2>/dev/null; then \
			echo "DELMOS вже запущено (PID $$pid)"; \
			exit 0; \
		fi; \
	fi; \
	echo "Запуск DELMOS у фоні..."; \
	DELMOS_COOKIE_SECURE=false nohup ./$(BUILD_DIR)/$(BINARY) -config ./configs/delmos.yaml -pid-file $(PID_FILE) > $(BUILD_DIR)/delmos.log 2>&1 & \
	sleep 0.5; \
	if [ -f $(PID_FILE) ]; then \
		pid=$$(cat $(PID_FILE)); \
		echo "DELMOS запущено (PID $$pid, лог: $(BUILD_DIR)/delmos.log)"; \
	else \
		echo "Помилка запуску DELMOS. Останні рядки логу:"; \
		cat $(BUILD_DIR)/delmos.log 2>/dev/null || true; \
		exit 1; \
	fi

stop: ## Зупинити локальний процес DELMOS за pid-файлом
	@if [ -f $(PID_FILE) ]; then \
		pid=$$(cat $(PID_FILE) 2>/dev/null); \
		if [ -n "$$pid" ] && kill -0 $$pid 2>/dev/null; then \
			echo "Зупинка DELMOS (PID $$pid)..."; \
			kill -TERM $$pid 2>/dev/null || true; \
			count=0; \
			while kill -0 $$pid 2>/dev/null; do \
				sleep 0.2; \
				count=$$((count + 1)); \
				if [ $$count -ge 50 ]; then \
					echo "Примусова зупинка (kill -9 $$pid)..."; \
					kill -9 $$pid 2>/dev/null || true; \
					break; \
				fi; \
			done; \
			rm -f $(PID_FILE); \
			echo "DELMOS зупинено"; \
		else \
			echo "Процес не знайдено, видалення застарілого $(PID_FILE)"; \
			rm -f $(PID_FILE); \
		fi; \
	else \
		echo "PID-файл $(PID_FILE) відсутній (DELMOS не запущено)"; \
	fi

restart: stop start ## Перезапустити локальний DELMOS у фоні (stop + start)

status: ## Перевірити статус локального процесу DELMOS за pid-файлом
	@if [ -f $(PID_FILE) ]; then \
		pid=$$(cat $(PID_FILE) 2>/dev/null); \
		if [ -n "$$pid" ] && kill -0 $$pid 2>/dev/null; then \
			echo "DELMOS працює (PID $$pid)"; \
			curl -s http://127.0.0.1:10120/api/v1/system/boot-status || true; \
			echo ""; \
		else \
			echo "DELMOS не працює (застарілий PID-файл $(PID_FILE) з PID $$pid)"; \
		fi; \
	else \
		echo "DELMOS не запущено (PID-файл $(PID_FILE) відсутній)"; \
	fi

test: web-embed ## Модульні тести з перевіркою гонок
	go test -race ./cmd/... ./internal/...

test-integration: web-embed ## Тести, що потребують PostgreSQL (DELMOS_TEST_DSN, за потреби DELMOS_TEST_TEMPLATE)
	@test -n "$(DELMOS_TEST_DSN)" || { echo "Задайте DELMOS_TEST_DSN (див. make db-test-setup)"; exit 1; }
	go test -race -count=1 ./internal/migrate/... ./internal/auth/... ./internal/project/... ./internal/repository/... ./internal/server/...

fmt: ## Відформатувати код
	gofmt -w cmd internal

fmt-check: ## Перевірити форматування без запису
	@unformatted=$$(gofmt -l cmd internal); \
	if [ -n "$$unformatted" ]; then echo "Не відформатовано:"; echo "$$unformatted"; exit 1; fi

vet: ## Статичний аналіз стандартним go vet
	go vet ./cmd/... ./internal/...

lint: ## golangci-lint (якщо встановлено)
	@if command -v golangci-lint >/dev/null 2>&1; then golangci-lint run ./cmd/... ./internal/...; \
	else echo "golangci-lint не встановлено — пропущено (go vet виконується окремо)"; fi

docs-links: ## Перевірити відносні посилання в Markdown-документації
	./scripts/check-doc-links.sh

web-install: ## Встановити npm-залежності фронтенду (один раз, до node_modules)
	@[ -d web/node_modules ] || (cd web && npm ci)

web-lint: web-install ## ESLint фронтенду
	cd web && npm run lint

web-typecheck: web-install ## Перевірка типів Vue/TypeScript (vue-tsc)
	cd web && npm run typecheck

web-test: web-install ## Юніт-тести фронтенду (vitest)
	cd web && npm run test:unit

web-build: web-install ## Скласти фронтенд у web/dist
	cd web && npm run build

web-embed: web-build ## Підготувати Web GUI для вбудовування у бінарник
	rm -rf $(WEB_ASSET_DIR)
	mkdir -p $(WEB_ASSET_DIR)
	cp -R web/dist/. $(WEB_ASSET_DIR)/
	touch $(WEB_ASSET_DIR)/.gitkeep

validate: fmt-check vet lint test docs-links web-lint web-typecheck web-test web-build build ## Повна перевірка якості перед Merge Request
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
	rm -rf $(BUILD_DIR) $(WEB_ASSET_DIR)
	mkdir -p $(WEB_ASSET_DIR)
	touch $(WEB_ASSET_DIR)/.gitkeep
