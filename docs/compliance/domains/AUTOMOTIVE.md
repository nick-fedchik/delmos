# DELMOS: домен комплаєнсу — Automotive (Автомобілебудування)

Дата: 2026-09-23. Статус: концептуально описаний напрямок; модулі ще не реалізовані.
Ліцензія: Apache 2.0.
Контекст: [domains/README.md](README.md), [каталог модулів](../../architecture/MODULE_CATALOG.md#3-галузеві-модулі-комплаєнсу-engineering-compliance).

---

## 1. Огляд домену

Автомобільна галузь наразі найдетальніше описана в архітектурному каталозі DELMOS, але її модулі не реалізовані: задум поєднує оцінку зрілості процесів (ASPICE), функціональну безпеку (ISO 26262) та кібербезпеку (ISO 21434) для HW/SW/системного рівня. Для відповідності потрібні конкретні докази трасування від вимоги до тесту на кожному рівні (SYS → SW → HW); сам опис не доводить відповідність.

## 2. Ключові стандарти

| Стандарт | Область застосування | Стан у DELMOS |
| --- | --- | --- |
| ASPICE 4.0 | Оцінка зрілості інженерних процесів (SYS.1–5, SWE.1–6, HWE.1–6) | Описано в каталозі, не реалізовано (`compliance.aspice`) |
| ISO 26262:2018 | Функціональна безпека E/E систем (ASIL A–D) | Описано в каталозі, не реалізовано (`compliance.iso26262`) |
| ISO/SAE 21434 | Інженерія кібербезпеки транспортних засобів (TARA, CAL 1–4) | Описано в каталозі, не реалізовано (`compliance.iso21434`) |
| ISO 21448 (SOTIF) | Безпека передбачуваного використання (для систем з ML/сприйняттям) | Заплановано |
| IATF 16949 | Система менеджменту якості автомобільних постачальників | Заплановано |
| UNECE R155 / R156 | Кібербезпека та керування програмними оновленнями (CSMS/SUMS) на рівні типового затвердження | Заплановано |

## 3. Модуль(і) комплаєнсу

Цільовий опис модулів `compliance.aspice`, `compliance.iso26262`, `compliance.iso21434` — [MODULE_CATALOG.md, розділи 3.1–3.3](../../architecture/MODULE_CATALOG.md#31-модуль-automotive-spice-40-complianceaspice).

### Плановане розширення: `compliance.iso21448` (SOTIF)
- **Словники:** `SOTIF_HAZARD_CATEGORY`, `TRIGGERING_CONDITION_SOURCE`.
- **Артефакти:** `iso21448:sotif_analysis`, `iso21448:known_unsafe_scenario_register`.
- **Правила:** `rule.iso21448.residual_risk_acceptance`: вимагає формального затвердження залишкового ризику перед релізом.

### Плановане розширення: `compliance.iatf16949`
- **Словники:** `APQP_PHASE`, `PPAP_LEVEL` (1–5).
- **Артефакти:** `iatf16949:control_plan`, `iatf16949:ppap_submission`.

## 4. Напрямок розширення

* Задіяні провайдери: `WPTypeProvider` (нові типи `sotif_analysis`, `ppap_submission`), `DictionaryProvider` (шкали SOTIF/APQP), `MilestoneProvider` (SOP — Start of Production), `TriggerRuleProvider` (правила залишкового ризику).
* Прогалина: відсутній формальний зв'язок HWE.1-6 (апаратні процеси ASPICE) з `test_spec` для HW-верифікації — розглядається як частина фази P2/P4 [ROADMAP.md](../../architecture/ROADMAP.md).

## 5. Посилання

* [MODULE_CATALOG.md](../../architecture/MODULE_CATALOG.md) — наявні реалізовані модулі.
* [MEMO-002: стратегія кваліфікації інструменту](../../memos/MEMO-002-tool-qualification-and-compliance-strategy.md).
* [STANDARD_MAPPING_TEMPLATE.md](../STANDARD_MAPPING_TEMPLATE.md) — шаблон для формального мапування нового стандарту.
