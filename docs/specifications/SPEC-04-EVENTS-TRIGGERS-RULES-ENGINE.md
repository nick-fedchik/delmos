# SPEC-04: Технічна специфікація рушія подій, тригерів та правил (Events, Triggers & Rules Engine)

Дата: 2026-09-23. Статус: нормативна технічна специфікація рушія автоматизації.  
Ліцензія: Apache 2.0.  
Контекст: [події, тригери та правила](../architecture/EVENTS_TRIGGERS_RULES.md), [ADR-007](../architecture/decisions/ADR-007-transactional-outbox-event-scheduler.md),
[SWR-12..14, SWR-23..27](../requirements/software/SWR-04-events-triggers-rules.md).

---

## 1. Конверт події (Event Envelope)

Кожна подія системи серіалізується у стандартизовану JSON-структуру:

```json
{
  "event_id": "8408ecb7-2639-45f6-98b4-d628d654b401",
  "event_key": "wp.revision_committed",
  "schema_version": "1.0.0",
  "source": "delmos.core.workproducts",
  "occurred_at": "2026-09-23T14:30:00.000Z",
  "recorded_at": "2026-09-23T14:30:00.012Z",
  "scope": {
    "scope_type": "project",
    "scope_id": "7108ecb7-2639-45f6-98b4-d628d654b200"
  },
  "actor": {
    "user_id": "5108ecb7-2639-45f6-98b4-d628d654b100",
    "role": "engineer"
  },
  "correlation_id": "req-99124-abc",
  "causation_id": "cmd-submit-requirement-01",
  "payload": {
    "work_product_id": "3108ecb7-2639-45f6-98b4-d628d654b500",
    "revision_id": "3108ecb7-2639-45f6-98b4-d628d654b501",
    "code": "REQ-042",
    "type": "requirement",
    "payload_hash": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
  }
}
```

---

## 2. Схема таблиць Transactional Outbox у PostgreSQL

```sql
CREATE TABLE event_definitions (
    event_key VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    owner_module VARCHAR(64) NOT NULL,
    payload_schema JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE event_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_key VARCHAR(64) NOT NULL REFERENCES event_definitions(event_key),
  project_id UUID REFERENCES projects(id) ON DELETE RESTRICT,
    envelope JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    dispatched_at TIMESTAMPTZ
);

CREATE TABLE trigger_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trigger_key VARCHAR(64) NOT NULL,
    event_key VARCHAR(64) NOT NULL REFERENCES event_definitions(event_key),
    subscriber_module VARCHAR(64) NOT NULL,
    routing_generation INT NOT NULL DEFAULT 1,
    active BOOLEAN NOT NULL DEFAULT true
);

CREATE TABLE event_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  outbox_event_id UUID NOT NULL REFERENCES event_outbox(id) ON DELETE RESTRICT,
  subscription_id UUID NOT NULL REFERENCES trigger_subscriptions(id) ON DELETE RESTRICT,
    status VARCHAR(32) NOT NULL DEFAULT 'pending', -- 'pending', 'leased', 'delivered', 'failed', 'dead_letter'
    lease_token UUID,
    lease_until TIMESTAMPTZ,
    delivery_attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 5,
    last_error TEXT,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX idx_deliveries_pending ON event_deliveries(status, lease_until) 
WHERE status IN ('pending', 'leased');
```

Таблиці `event_outbox` і `event_deliveries` є технічним станом доставки, **не** канонічним журналом аудиту. Деактивація підписки не видаляє її невиконані доставки. Очищати delivery можна лише після `delivered` або документованого розслідування `dead_letter`, завершення затвердженого строку, перевірки legal hold та наявності окремого аудиторського доказу; після цього outbox очищається тільки якщо немає невиконаних підписок. Без затвердженої політики retention автоматичне очищення вимкнене ([ADR-010](../architecture/decisions/ADR-010-audit-evidence-and-outbox-retention.md)).

Нижче мінімальна структура незмінного журналу рішень першого зрізу. Для подій без ревізії поля `revision_id` та `payload_hash` порожні; для погоджень вони обов'язкові на рівні прикладної команди. `actor_ref` посилається на чинного користувача або технічний principal; повні payload/секрети не зберігаються:

```sql
CREATE TABLE audit_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID REFERENCES projects(id) ON DELETE RESTRICT,
  actor_ref UUID NOT NULL,
  role_key VARCHAR(64) NOT NULL,
  scope_type VARCHAR(16) NOT NULL,
  scope_id UUID,
  subject_type VARCHAR(64) NOT NULL,
  subject_id UUID,
  action_key VARCHAR(64) NOT NULL,
  decision VARCHAR(32) NOT NULL,
  revision_id UUID REFERENCES work_product_revisions(id) ON DELETE RESTRICT,
  payload_hash CHAR(64),
  correlation_id VARCHAR(128) NOT NULL,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
```

`audit_events` записується в тій самій транзакції, що й істотна бізнес-зміна; для runtime-ролі заборонені `UPDATE`, `DELETE` та `TRUNCATE` на цій таблиці. Додаткові поля аудитора/обґрунтування додаються окремою forward-only міграцією, без редагування історичних подій.

---

## 3. Протокол конкурентної оренди повідомлень (Lease Fencing)

1. **Claiming (Захоплення оренди воркером):**
   ```sql
   UPDATE event_deliveries
   SET status = 'leased',
       lease_token = $worker_token,
       lease_until = clock_timestamp() + INTERVAL '30 seconds',
       delivery_attempts = delivery_attempts + 1
   WHERE id IN (
       SELECT id FROM event_deliveries
       WHERE status = 'pending' OR (status = 'leased' AND lease_until < clock_timestamp())
       ORDER BY created_at ASC
       LIMIT 10
       FOR UPDATE SKIP LOCKED
   )
   RETURNING id, outbox_event_id, lease_token;
   ```
2. **ACK (Підтвердження успішної обробки):**
   Воркер передає отриманий `lease_token`. Оновлення перевіряє токен:
   ```sql
   UPDATE event_deliveries
   SET status = 'delivered', completed_at = clock_timestamp(), lease_token = NULL
   WHERE id = $delivery_id AND lease_token = $worker_token;
   ```
3. **Dead-letter handling:** Якщо `delivery_attempts >= max_attempts`, повідомлення маркується як `dead_letter` і генерує інцидент для моніторингу адміністратора.
