# DELMOS: домен комплаєнсу — Railway (Залізничний транспорт)

Дата: 2026-09-23. Статус: запланований напрямок розширення.
Ліцензія: Apache 2.0.
Контекст: [domains/README.md](README.md), [каталог модулів](../../architecture/MODULE_CATALOG.md).

---

## 1. Огляд домену

Залізничний транспорт (сигналізація, системи керування рухом поїздів, гальмівні системи) регулюється трьома взаємопов'язаними європейськими стандартами CENELEC, що формують повний V-model цикл: від системного рівня надійності (RAMS) до конкретної реалізації безпечного програмного забезпечення (SIL 0–4).

## 2. Ключові стандарти

| Стандарт | Область застосування |
| --- | --- |
| EN 50126 (частини 1–2) | Специфікація та демонстрація надійності, готовності, ремонтопридатності та безпеки (RAMS) на системному рівні |
| EN 50128 | Програмне забезпечення для залізничних систем керування та сигналізації (SIL 0–4) |
| EN 50129 | Безпечні електронні системи для сигналізації (апаратна частина, докази безпеки) |
| EN 50159 | Безпечний зв'язок у системах передачі даних для залізниці |

## 3. Модуль(і) комплаєнсу

### Плановий модуль: `compliance.en5012x`
- **Внесок у Project Plan:** розділ `RAMS & Safety Case Plan`.
- **Словники:** `SIL` (`SIL0`..`SIL4`), `RAMS_ATTRIBUTE` (`Reliability`, `Availability`, `Maintainability`, `Safety`).
- **Майлстоуни:** `preliminary_safety_case_review`, `safety_case_acceptance` (перед введенням в експлуатацію).
- **Артефакти:** `en50126:rams_plan`, `en50128:software_safety_case`, `en50129:safety_case_hardware`, `en50128:software_quality_assurance_plan`.
- **Правила:** `rule.en50128.sil4_requires_independent_verification_body`: для SIL4 незалежна верифікація виконується органом, не залученим до розробки (посилена перевірка SoD).

## 4. Напрямок розширення

* Задіяні провайдери: `WPTypeProvider` (`rams_plan`, `software_safety_case`, `safety_case_hardware`), `DictionaryProvider` (шкала SIL, атрибути RAMS), `MilestoneProvider` (Safety Case Acceptance), `TriggerRuleProvider` (посилена незалежність верифікації для SIL4).
* Прогалина: потрібна композитна структура «Safety Case» (система + ПЗ + HW доказ разом) — природно моделюється як `CompositionManifest` (SPEC-01), що посилається на ревізії `rams_plan`, `software_safety_case` та `safety_case_hardware` одночасно.

## 5. Посилання

* [MODULE_CATALOG.md](../../architecture/MODULE_CATALOG.md) — зразок структури галузевого модуля.
* [specifications/SPEC-01-COMPOSITE-SPECIFICATIONS-ENGINE.md](../../specifications/SPEC-01-COMPOSITE-SPECIFICATIONS-ENGINE.md) — механізм композитних Safety Case.
* [STANDARD_MAPPING_TEMPLATE.md](../STANDARD_MAPPING_TEMPLATE.md) — шаблон формального мапування.
