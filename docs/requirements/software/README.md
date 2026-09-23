# Реєстр системних вимог DELMOS (Software / System Requirements)

Дата оновлення: 2026-09-23.  
Платформа: **DELMOS** (Discovery, Engineering & Lifecycle Management Operating System).  
Ліцензія: Apache 2.0.

---

## 1. Структура системних вимог за підсистемами

| Підсистема | Документ вимог | Діапазон вимог (SWR) | Батьківські вимоги (SHR) |
| --- | --- | --- | --- |
| **Ядро плану та управління проєктом** | [SWR-01: Core & Project Plan Engine](SWR-01-core-project-plan.md) | SWR-01..05 | SHR-01, SHR-02, SHR-03 |
| **Динамічні поля та модель контенту** | [SWR-02: Dynamic Fields & Content Models](SWR-02-dynamic-fields-content.md) | SWR-06..08, SWR-28..30 | SHR-04, SHR-09 |
| **Віхи, шлюзи та словники** | [SWR-03: Milestones, Gates & Vocabularies](SWR-03-milestones-vocabularies.md) | SWR-09..11, SWR-31..32 | SHR-05, SHR-10 |
| **Події, тригери, правила та планувальник** | [SWR-04: Events, Triggers, Rules & Scheduler](SWR-04-events-triggers-rules.md) | SWR-12..14, SWR-16, SWR-19..20, SWR-23..27 | SHR-06 |
| **Реєстр метрик та вимірювань** | [SWR-05: Metrics Registry](SWR-05-metrics-registry.md) | SWR-21..22 | SHR-08 |
| **Модульна система та UI Host** | [SWR-06: Modules & UI Host](SWR-06-modules-ui-host.md) | SWR-15, SWR-17..18 | SHR-02, SHR-03, SHR-07 |
| **Композитні специфікації (SPEC-01..10)** | [SWR-07: Composite Specifications](SWR-07-composite-specifications.md) | SWR-33..35 | SHR-11 |
| **Векторні дані, графи та візуалізація** | [SWR-08: Vectors, Graphs & Visuals](SWR-08-vectors-graphs-visuals.md) | SWR-36..38 | SHR-12 |
| **Проєктна економіка та ресурси** | [SWR-09: Project Economics & Resources](SWR-09-project-economics-resources.md) | SWR-39..41 | SHR-13 |
| **Доступ і незалежне погодження ядра** | [SWR-10: Core Access & Assurance](SWR-10-core-access-assurance.md) | SWR-42..48 | SHR-14 |

---

## 2. Нормативне покриття

Усі 48 системних вимог деталізують 14 стейкхолдерських вимог. Для `SWR-42..48` сценарій перевірки — [UAT-004](../../uat/UAT-004-core-roles-review-approval.md); рішення щодо кворуму і станів мають бути синхронізовані з архітектурним контрактом перед реалізацією.
