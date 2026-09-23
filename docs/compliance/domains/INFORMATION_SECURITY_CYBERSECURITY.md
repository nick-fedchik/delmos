# DELMOS: наскрізний стандарт — Information Security & Cybersecurity (Загальна інформаційна та кібербезпека)

Дата: 2026-09-23. Статус: наскрізний (горизонтальний) напрямок розширення, застосовний до будь-якого галузевого домену.
Ліцензія: Apache 2.0.
Контекст: [domains/README.md](README.md), [security/SECURITY_SPECIFICATION.md](../../security/SECURITY_SPECIFICATION.md),
[ACCESS_CONTROL.md](../../architecture/ACCESS_CONTROL.md).

---

## 1. Чому цей стандарт не є галузевим доменом

На відміну від файлів у розділі «Вертикальні галузеві домени» ([domains/README.md](README.md)), інформаційна безпека та кібербезпека є **горизонтальною вимогою**: вона застосовується до кожного проєкту незалежно від галузі (Automotive, Aviation, Medicine, Financial тощо). Тому цей документ описує мінімальний наскрізний каркас, який окремі галузеві домени лише доповнюють власними контролями (наприклад, [POWER_GENERATION_DISTRIBUTION.md](POWER_GENERATION_DISTRIBUTION.md) додає NERC CIP/IEC 62351 понад цей мінімум).

## 2. Ключові стандарти

| Стандарт | Область застосування |
| --- | --- |
| ISO/IEC 27001:2022 | Система менеджменту інформаційної безпеки (ISMS) — вимоги до сертифікації |
| ISO/IEC 27002:2022 | Каталог контролів безпеки (доповнює 27001) |
| NIST Cybersecurity Framework (CSF) 2.0 | Функціональна модель управління кіберризиком (Govern, Identify, Protect, Detect, Respond, Recover) |
| NIST SP 800-53 Rev. 5 | Каталог контролів безпеки та приватності для федеральних інформаційних систем США |
| SOC 2 (Type I / Type II) | Аудиторський звіт довіри постачальника послуг (Trust Services Criteria: Security, Availability, Confidentiality) |
| IEC 62443 (усі частини) | Кібербезпека промислових автоматизованих систем керування (Industrial Automation & Control Systems) |
| CIS Controls v8 | Практичний пріоритезований каталог технічних контролів безпеки |

## 3. Модуль(і) комплаєнсу

### Плановий модуль: `compliance.iso27001`
- **Внесок у Project Plan:** розділ `Information Security Management System (ISMS) Plan`.
- **Словники:** `ISO27001_CONTROL` (реєстр контролів Annex A), `RISK_TREATMENT_DECISION` (`accept`, `mitigate`, `transfer`, `avoid`).
- **Майлстоуни:** `internal_isms_audit`, `certification_surveillance_audit`.
- **Артефакти:** `iso27001:statement_of_applicability`, `iso27001:risk_treatment_plan`, `iso27001:internal_audit_report`.
- **Правила:** `rule.iso27001.soa_control_justification_required`: кожен контроль у `statement_of_applicability`, позначений як «не застосовний», вимагає письмового обґрунтування.

### Плановий модуль: `compliance.nist_csf`
- **Словники:** `NIST_CSF_FUNCTION` (`Govern`, `Identify`, `Protect`, `Detect`, `Respond`, `Recover`), `MATURITY_TIER` (`Partial`..`Adaptive`).
- **Артефакти:** `nist_csf:profile_current`, `nist_csf:profile_target`, `nist_csf:gap_analysis`.

## 4. Напрямок розширення

* Задіяні провайдери: `WPTypeProvider` (`statement_of_applicability`, `risk_treatment_plan`, `nist_csf:profile_current`), `DictionaryProvider` (реєстр контролів ISO 27001 Annex A, функцій NIST CSF), `MilestoneProvider` (внутрішній аудит ISMS, сертифікаційний наглядовий аудит), `TriggerRuleProvider` (обов'язковість обґрунтування для контролів поза межами застосування).
* Зв'язок із наявною специфікацією платформи: цей горизонтальний стандарт формалізує та розширює вже задокументовані практики DELMOS — шифрування ([security/SECURITY_SPECIFICATION.md](../../security/SECURITY_SPECIFICATION.md)), журнал аудиту ([operations/LOGGING_AND_OBSERVABILITY.md](../../operations/LOGGING_AND_OBSERVABILITY.md)) та модель авторизації ([ACCESS_CONTROL.md](../../architecture/ACCESS_CONTROL.md)) — а не вводить нову архітектуру з нуля.
* Прогалина: відсутній єдиний реєстр «застосовних контролів» (`Statement of Applicability`) як самостійна сутність — наразі відповідність описується прозово в окремих архітектурних документах, без формального трасування контроль → доказ.

## 5. Посилання

* [security/SECURITY_SPECIFICATION.md](../../security/SECURITY_SPECIFICATION.md) — шифрування, секрети, безпека вебхуків.
* [architecture/ACCESS_CONTROL.md](../../architecture/ACCESS_CONTROL.md) — модель авторизації RBAC/ABAC/SoD.
* [PERSONAL_DATA_PROTECTION.md](PERSONAL_DATA_PROTECTION.md) — суміжний наскрізний стандарт захисту персональних даних.
* [STANDARD_MAPPING_TEMPLATE.md](../STANDARD_MAPPING_TEMPLATE.md) — шаблон формального мапування.
