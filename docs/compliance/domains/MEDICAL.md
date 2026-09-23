# DELMOS: домен комплаєнсу — Medicine (Медичні вироби)

Дата: 2026-09-23. Статус: запланований напрямок розширення.
Ліцензія: Apache 2.0.
Контекст: [domains/README.md](README.md), [каталог модулів](../../architecture/MODULE_CATALOG.md).

---

## 1. Огляд домену

Розробка медичних виробів (Medical Device Software, MDSW) поєднує керування ризиком для пацієнта (ISO 14971) з процесним стандартом розробки ПЗ (IEC 62304) та вимогами до електробезпеки апаратної частини (IEC 60601-1). Регуляторні органи (FDA у США, нотифіковані органи ЄС за MDR) вимагають, щоб клас безпеки програмного забезпечення (`Software Safety Class`) визначав інтенсивність верифікації.

## 2. Ключові стандарти

| Стандарт | Область застосування |
| --- | --- |
| IEC 62304 | Життєвий цикл програмного забезпечення медичних виробів (Class A/B/C) |
| ISO 14971 | Керування ризиком для медичних виробів упродовж усього життєвого циклу |
| IEC 60601-1 | Загальні вимоги безпеки та основних експлуатаційних характеристик електромедичного обладнання (HW) |
| IEC 62366-1 | Інженерія юзабіліті (Usability Engineering) для медичних виробів |
| EU MDR 2017/745 | Регламент ЄС про медичні вироби (клінічна оцінка, технічна документація) |
| FDA 21 CFR Part 820 / QSR | Система якості виробника медичних виробів у США |

## 3. Модуль(і) комплаєнсу

### Плановий модуль: `compliance.iec62304`
- **Внесок у Project Plan:** розділ `Software Safety Classification & Development Plan`.
- **Словники:** `SOFTWARE_SAFETY_CLASS` (`A` — не може призвести до травми, `B` — незначна травма, `C` — смерть або серйозна травма).
- **Майлстоуни:** `design_history_file_freeze`, `software_release_for_clinical_use`.
- **Артефакти:** `iec62304:software_development_plan`, `iec62304:software_requirements_specification`, `iec62304:software_architecture`, `iec62304:unit_verification_record`.
- **Правила:** `rule.iec62304.class_c_requires_unit_verification`: для класу C обов'язкова верифікація на рівні одиниці (unit) перед інтеграцією.

### Плановий модуль: `compliance.iso14971`
- **Словники:** `HAZARD_SEVERITY`, `HAZARD_PROBABILITY`, `RISK_ACCEPTABILITY` (`acceptable`, `ALARP`, `unacceptable`).
- **Артефакти:** `iso14971:risk_management_file`, `iso14971:risk_analysis_record`, `iso14971:risk_control_verification`.
- **Правила:** `rule.iso14971.residual_risk_benefit_analysis`: неприйнятний залишковий ризик вимагає формального аналізу співвідношення користь/ризик.

### Плановий модуль: `compliance.iec60601` (апаратна безпека)
- **Артефакти:** `iec60601:electrical_safety_test_report`, `iec60601:essential_performance_verification`.

## 4. Напрямок розширення

* Задіяні провайдери: `WPTypeProvider` (`risk_management_file`, `software_architecture` профіль `iec62304`), `DictionaryProvider` (шкали Safety Class, Severity/Probability), `MilestoneProvider` (Design History File Freeze), `TriggerRuleProvider` (правило обов'язкової верифікації для класу C).
* Прогалина: відсутній вбудований механізм зв'язку `iso14971:risk_control_verification` із конкретними тест-кейсами класу `test_spec` — потребує розширення `TraceLink` типом `mitigates`.

## 5. Посилання

* [MODULE_CATALOG.md](../../architecture/MODULE_CATALOG.md) — зразок структури галузевого модуля.
* [ENTITY_CATALOG.md](../../architecture/ENTITY_CATALOG.md) — типи зв'язків трасованості (`TraceLink`).
* [STANDARD_MAPPING_TEMPLATE.md](../STANDARD_MAPPING_TEMPLATE.md) — шаблон формального мапування.
