# DELMOS: домен комплаєнсу — Aviation & Aerospace (Авіація та аерокосмічна галузь)

Дата: 2026-09-23. Статус: запланований напрямок розширення.
Ліцензія: Apache 2.0.
Контекст: [domains/README.md](README.md), [каталог модулів](../../architecture/MODULE_CATALOG.md).

---

## 1. Огляд домену

Цивільна та військова авіація вимагає найсуворішого рівня формальної верифікації серед усіх галузей: рівень забезпечення розробки (`DAL`, Development Assurance Level) визначає обсяг об'єктивних доказів (`objective evidence`) для кожного програмного модуля та апаратного компонента. На відміну від автомобільної галузі, авіаційні стандарти вимагають структурного покриття коду (`MC/DC`) та формального аналізу апаратних несправностей.

## 2. Ключові стандарти

| Стандарт | Область застосування |
| --- | --- |
| DO-178C / ED-12C | Розробка бортового програмного забезпечення (DAL A–E) |
| DO-254 / ED-80 | Розробка бортових складних електронних апаратних компонентів (HW DAL A–E) |
| ARP4754A / ED-79A | Розробка систем та інтеграція на рівні повітряного судна |
| ARP4761 | Методи оцінки безпеки систем (FHA, PSSA, SSA, FTA, FMEA) |
| DO-160 / ED-14 | Умови довкілля та випробування бортового обладнання |
| DO-330 | Кваліфікація інструментів розробки (Tool Qualification Level, TQL) |

## 3. Модуль(і) комплаєнсу

### Плановий модуль: `compliance.do178c`
- **Внесок у Project Plan:** розділ `Software Level & Certification Plan` (вибір DAL A–E, PSAC — Plan for Software Aspects of Certification).
- **Словники:** `DAL` (`A`..`E`), `OBJECTIVE_TYPE` (з таблиць Annex A DO-178C), `STRUCTURAL_COVERAGE_CRITERION` (`Statement`, `Decision`, `MC/DC`).
- **Майлстоуни:** `sois_review` (Stage of Involvement), `far_review` (Final Aircraft Review).
- **Артефакти:** `do178c:psac` (План сертифікації ПЗ), `do178c:software_verification_cases_and_procedures`, `do178c:software_accomplishment_summary`.
- **Правила:** `rule.do178c.mcdc_required_for_level_a`: для DAL A обов'язкове покриття MC/DC перед закриттям верифікаційного WP.

### Плановий модуль: `compliance.do254`
- **Словники:** `HW_DAL` (`A`..`E`), `HW_DESIGN_ASSURANCE_PROCESS`.
- **Артефакти:** `do254:hardware_accomplishment_summary`, `do254:validation_verification_plan`.

### Плановий модуль: `compliance.arp4761`
- **Артефакти:** `arp4761:functional_hazard_assessment`, `arp4761:fault_tree_analysis`, `arp4761:common_cause_analysis`.
- **Правила:** `rule.arp4761.catastrophic_requires_independent_review`: катастрофічні відмови вимагають незалежної рецензії (SoD) на рівні системи.

## 4. Напрямок розширення

* Задіяні провайдери: `WPTypeProvider` (`psac`, `hazard_assessment`, `fault_tree_analysis`), `DictionaryProvider` (шкали DAL/HW DAL), `MilestoneProvider` (Stage of Involvement, Type Certification), `MetricProvider` (відсоток структурного покриття MC/DC).
* Прогалина: платформа поки не має вбудованого механізму імпорту результатів структурного покриття коду (MC/DC) із зовнішніх інструментів статичного аналізу — потребує нового `IntegrationConnection` типу постачальника даних покриття.
* Кваліфікація самого DELMOS як інструмента верифікації (за DO-330 TQL) описується окремо в [MEMO-002](../../memos/MEMO-002-tool-qualification-and-compliance-strategy.md), адаптованим під авіаційний контекст.

## 5. Посилання

* [MODULE_CATALOG.md](../../architecture/MODULE_CATALOG.md) — зразок структури галузевого модуля комплаєнсу.
* [MODULES.md](../../architecture/MODULES.md) — типізовані провайдери розширень.
* [STANDARD_MAPPING_TEMPLATE.md](../STANDARD_MAPPING_TEMPLATE.md) — шаблон формального мапування.
