# DELMOS: домен комплаєнсу — Financial Services (Фінансові технології)

Дата: 2026-09-23. Статус: запланований напрямок розширення.
Ліцензія: Apache 2.0.
Контекст: [domains/README.md](README.md), [каталог модулів](../../architecture/MODULE_CATALOG.md).

---

## 1. Огляд домену

На відміну від решти доменів цього реєстру, фінансові технології (банкінг, платіжні системи, фінтех) є переважно **software-орієнтованим** доменом без апаратної складової системної безпеки, але з надзвичайно суворими вимогами до цілісності процесу розробки, аудиту змін та захисту даних. Комплаєнс тут зосереджений навколо контролю змін (Change Management), розподілу обов'язків (SoD) та захисту персональних/платіжних даних.

## 2. Ключові стандарти

| Стандарт | Область застосування |
| --- | --- |
| PCI DSS v4.0 | Захист даних платіжних карток (Payment Card Industry Data Security Standard) |
| SOX (Sarbanes-Oxley) Section 404 | Внутрішній контроль над фінансовою звітністю, включно з контролем змін у ПЗ |
| ISO/IEC 27001 | Система менеджменту інформаційної безпеки (ISMS) |
| GDPR / місцеві закони про персональні дані | Захист персональних даних клієнтів |
| MiFID II / PSD2 | Регулювання інвестиційних послуг та платіжних сервісів (ЄС) |

## 3. Модуль(і) комплаєнсу

### Плановий модуль: `compliance.sox_change_control`
- **Внесок у Project Plan:** розділ `Change Management & Segregation of Duties Plan`.
- **Словники:** `CHANGE_RISK_TIER` (`Low`, `Medium`, `High`), `CONTROL_OBJECTIVE` (реєстр контрольних цілей SOX ITGC).
- **Майлстоуни:** `quarterly_control_attestation`.
- **Артефакти:** `sox:change_request_record`, `sox:control_attestation_report`, `sox:segregation_of_duties_matrix`.
- **Правила:** `rule.sox.four_eyes_on_production_change`: будь-яка зміна, позначена як `production-impacting`, вимагає підтвердження щонайменше двома незалежними ролями (розширення вбудованого правила SoD).

### Плановий модуль: `compliance.pci_dss`
- **Словники:** `CARDHOLDER_DATA_SCOPE` (`in-scope`, `out-of-scope`), `PCI_REQUIREMENT` (реєстр 12 вимог PCI DSS).
- **Артефакти:** `pci_dss:scope_definition`, `pci_dss:vulnerability_scan_report`, `pci_dss:penetration_test_report`.
- **Правила:** `rule.pci_dss.quarterly_scan_required`: артефакт `vulnerability_scan_report` не може мати вік понад 90 днів для систем у межах області PCI.

## 4. Напрямок розширення

* Задіяні провайдери: `WPTypeProvider` (`change_request_record`, `vulnerability_scan_report`), `DictionaryProvider` (реєстр вимог PCI DSS/SOX ITGC), `MilestoneProvider` (квартальна атестація контролів), `TriggerRuleProvider` (правило чотирьох очей, обов'язковість актуального сканування вразливостей).
* Прогалина: цей домен майже не потребує апаратних (`test_spec` HW) типів — натомість посилює вимоги до вбудованого журналу аудиту ([LOGGING_AND_OBSERVABILITY.md](../../operations/LOGGING_AND_OBSERVABILITY.md)) як формального доказу контролю змін.

## 5. Посилання

* [MODULE_CATALOG.md](../../architecture/MODULE_CATALOG.md) — зразок структури галузевого модуля.
* [security/SECURITY_SPECIFICATION.md](../../security/SECURITY_SPECIFICATION.md) — шифрування та захист даних, релевантні для PCI DSS/GDPR.
* [STANDARD_MAPPING_TEMPLATE.md](../STANDARD_MAPPING_TEMPLATE.md) — шаблон формального мапування.
