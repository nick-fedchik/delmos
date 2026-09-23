# DELMOS: домен комплаєнсу — IoT & Consumer Cybersecurity (Побутова електроніка та інтернет речей)

Дата: 2026-09-23. Статус: запланований напрямок розширення.
Ліцензія: Apache 2.0.
Контекст: [domains/README.md](README.md), [каталог модулів](../../architecture/MODULE_CATALOG.md).

---

## 1. Огляд домену

Побутова електроніка та підключені пристрої (Consumer IoT, розумний дім, носима електроніка) є наймолодшим доменом обов'язкового регуляторного комплаєнсу: вимоги до кібербезпеки «за замовчуванням» стають обов'язковими для виходу на ринок ЄС (Radio Equipment Directive delegated act, спираючись на ETSI EN 303 645) та США (маркування NIST/FTC Cyber Trust Mark). На відміну від важкої промисловості, тут акцент — на безпечних значеннях за замовчуванням, керуванні вразливостями та життєвому циклі оновлень «по повітрю» (OTA).

## 2. Ключові стандарти

| Стандарт | Область застосування |
| --- | --- |
| ETSI EN 303 645 | Базові вимоги кібербезпеки для споживчих IoT-пристроїв (13 положень: відсутність дефолтних паролів, керування вразливостями тощо) |
| IEC 62443 (серія) | Кібербезпека промислових автоматизованих систем керування (частково застосовна й до розумних пристроїв на межі OT/IoT) |
| NISTIR 8259 | Базові можливості кібербезпеки для виробників IoT-пристроїв |
| EU Cyber Resilience Act (CRA) | Вимоги до кібербезпеки протягом усього життєвого циклу продуктів з цифровими елементами на ринку ЄС |

## 3. Модуль(і) комплаєнсу

### Плановий модуль: `compliance.iot_cybersecurity`
- **Внесок у Project Plan:** розділ `Product Cybersecurity & Vulnerability Disclosure Plan`.
- **Словники:** `EN303645_PROVISION` (реєстр 13 базових положень), `VULNERABILITY_DISCLOSURE_STATUS` (`reported`, `triaged`, `patched`, `disclosed`).
- **Майлстоуни:** `security_baseline_review`, `ota_update_capability_verification`.
- **Артефакти:** `iot_cybersec:vulnerability_disclosure_policy`, `iot_cybersec:software_bill_of_materials` (SBOM), `iot_cybersec:security_baseline_compliance_record`.
- **Правила:** `rule.en303645.no_universal_default_passwords`: артефакт типу `security_baseline_compliance_record` блокується (`veto`), якщо позначено використання універсального дефолтного пароля.

## 4. Напрямок розширення

* Задіяні провайдери: `WPTypeProvider` (`vulnerability_disclosure_policy`, `software_bill_of_materials`), `DictionaryProvider` (реєстр 13 положень EN 303 645), `MilestoneProvider` (Security Baseline Review), `TriggerRuleProvider` (veto на дефолтні паролі), `IntegrationConnection` (автоматичний імпорт SBOM зі сканерів залежностей).
* Прогалина: платформа поки не має нативного типу артефакту SBOM (Software Bill of Materials, формат SPDX/CycloneDX) — потрібне розширення `WPTypeProvider` новим типом та парсером формату для автоматичного заповнення `metadata`.

## 5. Посилання

* [MODULE_CATALOG.md](../../architecture/MODULE_CATALOG.md) — зразок структури галузевого модуля.
* [security/SECURITY_SPECIFICATION.md](../../security/SECURITY_SPECIFICATION.md) — базові принципи безпеки, релевантні для дефолтних значень і секретів.
* [STANDARD_MAPPING_TEMPLATE.md](../STANDARD_MAPPING_TEMPLATE.md) — шаблон формального мапування.
