# Реєстр архітектурних рішень DELMOS (Architecture Decision Records)

Дата оновлення: 2026-09-23.  
Платформа: **DELMOS** (Discovery, Engineering & Lifecycle Management Operating System).  
Ліцензія: Apache 2.0.

---

## 1. Реєстр рішень (ADR Index)

| Номер | Назва рішення | Статус | Область системи |
| --- | --- | --- | --- |
| [ADR-001](ADR-001-all-in-postgresql-storage.md) | Використання PostgreSQL як єдиного сховища реляційних, графових і векторних даних | Accepted | Ядро, База даних, Вектори, Графи |
| [ADR-002](ADR-002-strict-workitem-workproduct-separation.md) | Суворе семантичне розмежування діяльності (WorkItem) та результату (WorkProduct) | Accepted | Предметна модель, Workflow, SoD |
| [ADR-003](ADR-003-composite-specification-manifests.md) | Композитні специфікації та незмінні маніфести ревізій елементів | Accepted | Артефакти, Специфікації, Версіонування |
| [ADR-004](ADR-004-two-tier-module-governance-generic-plan.md) | Дворівневе керування конфігурацією проєкту та обов'язковий Generic Project Plan | Accepted | Конфігурація, План проєкту, Модулі |
| [ADR-005](ADR-005-multi-provider-repository-abstraction.md) | Мультипровайдерна абстракція сховищ та трекерів (RepositoryProvider) | Accepted | Сховища, Інтеграції, Docs-as-Code |
| [ADR-006](ADR-006-project-economics-resource-accounting.md) | Інтеграція проєктної економіки (Project Economics) та обліку ресурсів | Accepted | Економіка, Ресурси, EVM, P&L |
| [ADR-007](ADR-007-transactional-outbox-event-scheduler.md) | Транзакційна черга подій (Outbox), оренда повідомлень та секундний планувальник | Accepted | Автоматизація, Черга, Планувальник |
| [ADR-008](ADR-008-svg-vector-visualization-for-compliance.md) | Використання векторної графіки (SVG / Vue Flow) для візуалізації графів та аудитів | Accepted | Інтерфейс, RTM, Аудит, Векторна графіка |
| [ADR-009](ADR-009-core-review-and-approval-policy.md) | Базовий кворум незалежних Review/Approval, повернення на зміни та окреме застосування плану | Accepted (цільовий контракт) | Ядро, права, workflow, SoD |
| [ADR-010](ADR-010-audit-evidence-and-outbox-retention.md) | Незмінні аудиторські докази й контрольоване очищення технічної черги без каскадів | Accepted (цільовий контракт) | Ядро, збереження доказів, outbox |
| [ADR-011](ADR-011-iso-21500-series-normative-base.md) | Серія ISO 21500 як нормативна база керування проєктами й проєктної економіки | Accepted | Термінологія, Економіка, WBS, Комплаєнс |

---

## 2. Стандарти та життєвий цикл ADR

Кожне архітектурне рішення фіксує контекст інженерного вибору, альтернативні варіанти, затверджене рішення та його позитивні й негативні наслідки. Рішення верифікуються зв'язками з нормативними вимогами та технічними специфікаціями платформи.
