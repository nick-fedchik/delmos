# DELMOS: домен комплаєнсу — Power Generation & Distribution (Енергетика: генерація та розподіл)

Дата: 2026-09-23. Статус: запланований напрямок розширення.
Ліцензія: Apache 2.0.
Контекст: [domains/README.md](README.md), [каталог модулів](../../architecture/MODULE_CATALOG.md),
[INDUSTRIAL_AUTOMATION.md](INDUSTRIAL_AUTOMATION.md) (спільна генерична основа IEC 61508).

---

## 1. Огляд домену

Енергетична галузь (генерація, передача та розподіл електроенергії, smart grid, відновлювані джерела) поєднує функціональну безпеку систем автоматизації підстанцій з обов'язковим захистом критичної інфраструктури від кібератак. На відміну від атомної енергетики (окремий домен [NUCLEAR_ENERGY.md](NUCLEAR_ENERGY.md)), тут регулятором у Північній Америці є NERC (North American Electric Reliability Corporation) з юридично зобов'язальними стандартами CIP, а в ЄС — мережеві кодекси ENTSO-E та директива NIS2.

## 2. Ключові стандарти

| Стандарт | Область застосування |
| --- | --- |
| IEC 61850 (усі частини) | Комунікаційні протоколи та архітектура автоматизації підстанцій електропередачі |
| IEC 62351 (усі частини) | Кібербезпека протоколів керування енергосистемами (доповнює IEC 61850/60870-5/61970) |
| NERC CIP-002 … CIP-014 | Обов'язкові стандарти захисту критичної інфраструктури електроенергетики США/Канади (North American Electric Reliability Corporation Critical Infrastructure Protection) |
| IEEE 1547 | Вимоги до підключення розподіленої генерації (сонячні, вітрові станції) до мережі |
| EU NIS2 Directive | Директива ЄС про кібербезпеку операторів критично важливих секторів (включно з енергетикою) |
| IEC 61508 (генеричний) | Функціональна безпека систем захисту генераторів/турбін — див. [INDUSTRIAL_AUTOMATION.md](INDUSTRIAL_AUTOMATION.md) |

## 3. Модуль(і) комплаєнсу

### Плановий модуль: `compliance.nerc_cip`
- **Внесок у Project Plan:** розділ `Critical Infrastructure Protection Plan`.
- **Словники:** `CIP_STANDARD` (`CIP-002`..`CIP-014`), `BES_CYBER_ASSET_CATEGORY` (`High`, `Medium`, `Low` impact).
- **Майлстоуни:** `cip_senior_manager_approval`, `annual_cip_audit`.
- **Артефакти:** `nerc_cip:bes_cyber_system_categorization`, `nerc_cip:security_awareness_program_record`, `nerc_cip:incident_response_plan`.
- **Правила:** `rule.nerc_cip.high_impact_requires_quarterly_review`: активи категорії `High impact` вимагають щоквартального перегляду контролів доступу.

### Плановий модуль: `compliance.iec61850_62351`
- **Словники:** `SUBSTATION_AUTOMATION_LEVEL`, `IEC62351_SECURITY_MEASURE`.
- **Артефакти:** `iec61850:scd_configuration_record` (System Configuration Description), `iec62351:security_assessment_report`.
- **Правила:** `rule.iec62351.goose_message_authentication_required`: конфігурація протоколу GOOSE не приймається без увімкненої автентифікації повідомлень.

## 4. Напрямок розширення

* Задіяні провайдери: `WPTypeProvider` (`bes_cyber_system_categorization`, `scd_configuration_record`), `DictionaryProvider` (реєстр стандартів CIP, рівні впливу BES Cyber Asset), `MilestoneProvider` (щорічний аудит CIP), `TriggerRuleProvider` (обов'язковість автентифікації GOOSE, періодичність перегляду контролів).
* Перетин із наскрізним доменом кібербезпеки: базові вимоги NERC CIP і IEC 62351 спираються на той самий каркас керування ризиками, що й [INFORMATION_SECURITY_CYBERSECURITY.md](INFORMATION_SECURITY_CYBERSECURITY.md) (ISO/IEC 27001, NIST CSF) — цей домен додає лише галузево-специфічні контролі понад горизонтальний мінімум.
* Прогалина: облік активів категорії `BES Cyber Asset` потребує окремого реєстру фізичних активів (сервери, RTU, IED), відсутнього в поточній предметній моделі — можливе розширення `DOMAIN_MODEL.md` новою сутністю `PhysicalAsset`.

## 5. Посилання

* [MODULE_CATALOG.md](../../architecture/MODULE_CATALOG.md) — зразок структури галузевого модуля.
* [INDUSTRIAL_AUTOMATION.md](INDUSTRIAL_AUTOMATION.md) — спільна генерична основа функціональної безпеки (IEC 61508).
* [INFORMATION_SECURITY_CYBERSECURITY.md](INFORMATION_SECURITY_CYBERSECURITY.md) — наскрізний мінімум кібербезпеки.
* [STANDARD_MAPPING_TEMPLATE.md](../STANDARD_MAPPING_TEMPLATE.md) — шаблон формального мапування.
