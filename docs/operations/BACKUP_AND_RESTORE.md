# DELMOS: резервне копіювання та відновлення (Backup & Restore)

Дата: 2026-09-23. Статус: нормативний операційний посібник.
Ліцензія: Apache 2.0.
Контекст: [вимоги до СУБД PostgreSQL](../requirements/SYSTEM_REQUIREMENTS.md#3-вимоги-до-субд-postgresql-database-requirements),
[аварійне відновлення](DISASTER_RECOVERY.md).

---

## 1. Огляд

Платформа DELMOS зберігає всі дані (реляційні сутності, графи трасованості, векторні ембедінги `pgvector`) в єдиній базі даних PostgreSQL. Резервне копіювання виконується стандартною утилітою `pg_dump` у кастомному стиснутому форматі, що дозволяє вибіркове відновлення окремих таблиць за потреби.

## 2. Створення резервної копії

```bash
#!/usr/bin/env bash
set -euo pipefail

BACKUP_DIR="/var/backups/delmos"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
BACKUP_FILE="${BACKUP_DIR}/delmos-${TIMESTAMP}.dump"

sudo install -d -m 0750 -o delmos -g delmos "${BACKUP_DIR}"

sudo -u delmos pg_dump \
  --format=custom \
  --file="${BACKUP_FILE}" \
  --dbname="delmos"

sudo -u delmos sha256sum "${BACKUP_FILE}" > "${BACKUP_FILE}.sha256"

echo "==> Резервну копію створено: ${BACKUP_FILE}"
```

## 3. Відновлення з резервної копії

```bash
#!/usr/bin/env bash
set -euo pipefail

BACKUP_FILE="$1"
RESTORE_DB="${2:-delmos_restore_test}"

# Обов'язкова перевірка контрольної суми перед відновленням
cd "$(dirname "${BACKUP_FILE}")"
sha256sum --check "$(basename "${BACKUP_FILE}").sha256"

sudo -u postgres dropdb --if-exists "${RESTORE_DB}"
sudo -u postgres createdb "${RESTORE_DB}"
sudo -u postgres pg_restore \
  --exit-on-error \
  --no-owner \
  --dbname="${RESTORE_DB}" \
  "${BACKUP_FILE}"

echo "==> Відновлено в тестову базу: ${RESTORE_DB}"
```

**Правило безпеки:** відновлення ніколи не виконується безпосередньо у робочу базу `delmos`. Спочатку дані відновлюються в окрему тестову базу (`delmos_restore_test`), перевіряються, і лише після підтвердження цілісності DBA приймає рішення про перемикання робочого трафіку.

## 4. Планування через Cron

```cron
# Щоденне резервне копіювання о 02:00 UTC
0 2 * * * /usr/local/bin/delmos-backup.sh >> /var/log/delmos/backup.log 2>&1
```

## 5. Регулярна перевірка відновлення (Restore Rehearsal)

Репетиція відновлення виконується щотижня для підтвердження придатності резервних копій:

1. Обрати останню резервну копію за поточний тиждень.
2. Виконати сценарій відновлення в тимчасову базу `delmos_restore_test`.
3. Перевірити кількість застосованих міграцій: `SELECT COUNT(*) FROM schema_migrations;`.
4. Перевірити наявність очікуваної кількості проєктів і артефактів.
5. Видалити тимчасову базу після успішної перевірки: `dropdb delmos_restore_test`.

## 6. Дивіться також

* [DISASTER_RECOVERY.md](DISASTER_RECOVERY.md) — повний план аварійного відновлення з цілями RPO/RTO.
* [UPGRADE_AND_ROLLBACK.md](UPGRADE_AND_ROLLBACK.md) — обов'язкове резервне копіювання перед оновленням версії.
