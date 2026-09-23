# DELMOS: домен комплаєнсу — Space (Космічна галузь)

Дата: 2026-09-23. Статус: запланований напрямок розширення.
Ліцензія: Apache 2.0.
Контекст: [domains/README.md](README.md), [каталог модулів](../../architecture/MODULE_CATALOG.md).

---

## 1. Огляд домену

Космічна галузь (супутники, ракети-носії, наземні станції керування) вирізняється неможливістю фізичного ремонту виробу після запуску, тому основний акцент комплаєнсу зроблено на превентивній верифікації, дублюванні (redundancy) та формальному аналізі відмов на найранніших фазах проєкту. Європейська система стандартів ECSS структурована аналогічно ASPICE/ISO 26262 — окремі гілки Q (Quality), E (Engineering), M (Management).

## 2. Ключові стандарти

| Стандарт | Область застосування |
| --- | --- |
| ECSS-E-ST (серія Engineering) | Системна, програмна та апаратна інженерія космічних апаратів |
| ECSS-Q-ST (серія Quality) | Забезпечення якості, надійності, аналіз відмов (FMEA, Worst Case Analysis) |
| ECSS-M-ST (серія Management) | Управління проєктом, фазові огляди (PDR, CDR, QR, AR) |
| NASA-STD-8719 | Стандарти безпеки NASA для пілотованих та безпілотних місій |
| ECSS-Q-ST-80 | Забезпечення якості програмного забезпечення космічних систем |
| CCSDS (Consultative Committee for Space Data Systems) | Протоколи телеметрії, телекомандування та передачі даних між космічним апаратом і наземним сегментом |
| AS9100 | Система менеджменту якості для аерокосмічної та оборонної галузі (доповнює ISO 9001 вимогами простежуваності деталей та конфігураційного контролю) |

**Достатність покриття:** базовий набір ECSS + NASA-STD-8719 покриває системний/програмний/якісний рівень; CCSDS і AS9100 додано для повноти охоплення протоколів наземного сегмента та ланцюга постачання апаратних компонентів.

## 3. Модуль(і) комплаєнсу

### Плановий модуль: `compliance.ecss`
- **Внесок у Project Plan:** розділ `ECSS Tailoring & Milestone Review Plan` (вибір застосовних стандартів серій E/Q/M залежно від класу місії).
- **Словники:** `ECSS_MILESTONE` (`SRR`, `PDR`, `CDR`, `QR`, `AR` — Acceptance Review), `CRITICALITY_CATEGORY` (за ECSS-Q-ST-30, категорії 1–3).
- **Майлстоуни:** `preliminary_design_review`, `critical_design_review`, `qualification_review`, `acceptance_review`.
- **Артефакти:** `ecss:tailoring_matrix`, `ecss:worst_case_analysis`, `ecss:failure_modes_effects_analysis`, `ecss:qualification_test_report`.
- **Правила:** `rule.ecss.category1_requires_redundancy_analysis`: компоненти категорії критичності 1 вимагають формального аналізу резервування перед CDR.

## 4. Напрямок розширення

* Задіяні провайдери: `WPTypeProvider` (`worst_case_analysis`, `failure_modes_effects_analysis`), `DictionaryProvider` (реєстр серій ECSS та тайлорингу), `MilestoneProvider` (повна послідовність оглядів SRR→PDR→CDR→QR→AR), `TriggerRuleProvider` (обов'язковість аналізу резервування).
* Прогалина: ECSS передбачає окрему структуру тайлорингу (вибір застосовних вимог стандарту для конкретного класу місії, аналогічно `TailoringSystem` для ASPICE) — потребує повторного використання того самого архітектурного патерна тайлорингу, узагальненого для довільного стандарту.

## 5. Посилання

* [MODULE_CATALOG.md](../../architecture/MODULE_CATALOG.md) — зразок структури галузевого модуля.
* [architecture/PROJECT_PLAN_ENGINE.md](../../architecture/PROJECT_PLAN_ENGINE.md) — механізм фазових шлюзів (Gate Reviews), придатний для SRR/PDR/CDR/QR/AR.
* [STANDARD_MAPPING_TEMPLATE.md](../STANDARD_MAPPING_TEMPLATE.md) — шаблон формального мапування.
