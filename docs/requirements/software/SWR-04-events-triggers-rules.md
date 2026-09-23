# SWR-04: Системні вимоги до подій, тригерів, правил та планувальника

Дата: 2026-09-23. Статус: нормативні вимоги підсистеми Events, Triggers & Rules Engine.  
Ліцензія: Apache 2.0.  
Батьківські вимоги: [SHR-06](../stakeholder/SHR-06-events-triggers-rules.md).  
Пов'язані архітектурні рішення: [ADR-007](../../architecture/decisions/ADR-007-transactional-outbox-event-scheduler.md).

---

#### SWR-12: Системні точки перевірки та події (Event Triggers)
1. Системні точки перехоплення команд (`before` guards): `wp.before_create`, `wp.before_update`, `wp.before_transition`, `trace.before_link`, `milestone.before_accept`, `baseline.before_freeze`.
2. Асинхронні події ядра (`after` committed events): `wp.created`, `wp.revision_committed`, `wp.status_changed`, `trace.link_created`, `milestone.accepted`, `plan.applied`, `baseline.frozen`.

#### SWR-13: Оцінка умов (Conditions Evaluation)
1. Правило чітко розділяє `when` (застосовність), `assert` (інваріант) та `on_failure` (veto або warning).
2. Якщо `when = false`, результат `not_applicable`, операція не блокується. Якщо `when = true` і `assert = false`, виконується дія при порушенні. Помилка обчислення обов'язкового правила блокує дію.

#### SWR-14: Дії правил (Actions & Guards)
1. Блокуюча дія (`MANDATORY_VETO`): відкат транзакції з кодом 422 та зрозумілим повідомленням.
2. Аудиторська дія (`ADVISORY_WARNING`): запис зауваження в аудит без зупинки дії.
3. Автоматичний розрахунок метаданих (`derive_value`): детермінований розрахунок залежних полів (наприклад, матриця ASIL Table 4).

#### SWR-16: Першокласні сутності Events, Triggers, Rules у ядрі
1. Ядро містить таблиці `event_definitions`, `event_instances`, `trigger_definitions`, `rule_definitions` з унікальними строковими ключами.

#### SWR-19: Каталог операцій та джерел подій
1. Успішні команди мутації атомарно записують зміну сутності, запис у журнал подій та повідомлення у вихідну чергу (outbox).

#### SWR-20: Керування RuleSet та scoped bindings
1. Ефективний список правил формується з обов'язкових правил ядра, системних політик та правил модулів, активованих у проєктному плані.

#### SWR-23: Конверт подій ядра (Event Envelope)
1. Конверт події містить ID, тип, версію схеми, джерело, часові мітки (UTC), проектний скоуп, актора, а також `correlation_id` та `causation_id`.

#### SWR-24: Секундний планувальник завдань
1. Планувальник підтримує режими `once`, `fixed_rate`, `fixed_delay`, `calendar` із точністю до секунди та захистом від повторного запуску через унікальний композитний ключ `(schedule_id, occurrence_time_utc)`.

#### SWR-25: Надійна черга повідомлень у PostgreSQL
1. Реалізація черги на базі таблиць `inbox`, `outbox`, `event_deliveries` із механізмом конкурентної оренди (Lease Fencing) та обов'язковим контролем ідемпотентності.

#### SWR-26: Реєстр предикатів та правил
1. Предикати умов типізовані, мають фіксований бюджет виконання та реєструються платформою. Заборонено виконання довільного SQL чи JS з тексту плану.

#### SWR-27: Відмовостійкість подієвої підсистеми
1. Захист від перевантаження (Backpressure), обмеження повторних спроб (Dead-letter queue) та глибини ланцюжка викликів.
