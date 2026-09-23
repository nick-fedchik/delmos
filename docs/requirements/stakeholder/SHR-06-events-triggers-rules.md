# SHR-06: Першокласні сутності ядра Events, Triggers, Rules

- **Ідентифікатор:** `SHR-06`
- **Стейкхолдери:** Lead Systems Architect, Process Automation Engineer, Auditor.
- **Формулювання:** Ядро системи повинно мати вбудовані першокласні сутності подій (Events), тригерів (Triggers) та правил (Rules), а функціональні модулі повинні мати можливість реєструвати нові типи подій, тригери та галузеві правила для перевірки переходів станів, зв'язків та бейзлайнів.
- **Обґрунтування (Rationale):** Замість жорстких `if/else` розгалужень у вихідному коді, правила контролю якості, інваріанти трасованості та реакції на події мають бути декларативними, версійованими та розширюваними модулями.
- **Критерії приймання:**
  1. Події мають єдиний незмінний конверт (Event Envelope) із фіксацією часу, актора та скоупу.
  2. Тригери розмежовують синхронні перехоплення команд (`before` guards) та асинхронні реакції (`after`).
  3. Правила підтримують роздільну оцінку застосовності (`when`) та умов (`assert`), а також дії блокування (`veto`) чи аудиторських зауважень (`advisory`).
  4. Надійна черга повідомлень реалізується на базі PostgreSQL (outbox) без сторонніх брокерів.
- **Декомпозиція на системні вимоги:** [SWR-12](../software/SWR-04-events-triggers-rules.md), [SWR-13](../software/SWR-04-events-triggers-rules.md), [SWR-14](../software/SWR-04-events-triggers-rules.md), [SWR-16](../software/SWR-04-events-triggers-rules.md), [SWR-19](../software/SWR-04-events-triggers-rules.md), [SWR-20](../software/SWR-04-events-triggers-rules.md), [SWR-23](../software/SWR-04-events-triggers-rules.md), [SWR-24](../software/SWR-04-events-triggers-rules.md), [SWR-25](../software/SWR-04-events-triggers-rules.md), [SWR-26](../software/SWR-04-events-triggers-rules.md), [SWR-27](../software/SWR-04-events-triggers-rules.md).
- **Пов'язані архітектурні рішення:** [ADR-007](../../architecture/decisions/ADR-007-transactional-outbox-event-scheduler.md).
