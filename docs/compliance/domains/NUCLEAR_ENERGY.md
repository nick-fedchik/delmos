# DELMOS: домен комплаєнсу — Nuclear Energy (Атомна енергетика)

Дата: 2026-09-23. Статус: запланований напрямок розширення.
Ліцензія: Apache 2.0.
Контекст: [domains/README.md](README.md), [каталог модулів](../../architecture/MODULE_CATALOG.md).

---

## 1. Огляд домену

Системи керування атомними електростанціями (Instrumentation & Control, I&C) вимагають найвищого рівня доказовості серед усіх цивільних доменів: категорія важливості для безпеки (`Safety Category`) визначає інтенсивність незалежної верифікації та валідації (IV&V), а регуляторний нагляд здійснюється національними органами (у координації з МАГАТЕ, IAEA).

## 2. Ключові стандарти

| Стандарт | Область застосування |
| --- | --- |
| IEC 61513 | Загальні вимоги до систем I&C, важливих для безпеки атомних станцій |
| IEC 60880 | Вимоги до програмного забезпечення систем захисту атомних станцій (Safety Category A) |
| IEC 62138 | Вимоги до ПЗ систем I&C категорій B та C (менш критичні за 60880) |
| IAEA SSG-39 | Настанова МАГАТЕ з питань безпеки програмного забезпечення для систем I&C |
| IEC 60987 | Вимоги до апаратного забезпечення систем I&C, важливих для безпеки |

## 3. Модуль(і) комплаєнсу

### Плановий модуль: `compliance.iec61513`
- **Внесок у Project Plan:** розділ `Safety I&C Categorization & V&V Plan`.
- **Словники:** `SAFETY_CATEGORY` (`A`, `B`, `C` — за спадною критичністю), `IVV_INDEPENDENCE_LEVEL`.
- **Майлстоуни:** `design_verification_review`, `licensing_submission_freeze`, `commissioning_acceptance`.
- **Артефакти:** `iec61513:safety_classification_record`, `iec60880:software_verification_and_validation_plan`, `iec61513:independent_verification_report`.
- **Правила:** `rule.iec61513.category_a_requires_diverse_ivv_team`: для категорії A незалежна команда верифікації не повинна мати спільних учасників з командою розробки (посилений SoD).

## 4. Напрямок розширення

* Задіяні провайдери: `WPTypeProvider` (`safety_classification_record`, `verification_and_validation_plan`), `DictionaryProvider` (шкала Safety Category), `MilestoneProvider` (Licensing Submission, Commissioning Acceptance), `TriggerRuleProvider` (посилена незалежність IV&V для категорії A).
* Прогалина: цей домен вимагає найсуворішої моделі SoD серед усіх — потрібне явне розширення правила SoD полем «diverse team» (відсутність спільних учасників між двома ролями), що виходить за межі базової перевірки «автор ≠ затверджувач» в [ACCESS_CONTROL.md](../../architecture/ACCESS_CONTROL.md).

## 5. Посилання

* [MODULE_CATALOG.md](../../architecture/MODULE_CATALOG.md) — зразок структури галузевого модуля.
* [ACCESS_CONTROL.md](../../architecture/ACCESS_CONTROL.md) — базова модель розподілу обов'язків (SoD), що потребує посилення.
* [STANDARD_MAPPING_TEMPLATE.md](../STANDARD_MAPPING_TEMPLATE.md) — шаблон формального мапування.
