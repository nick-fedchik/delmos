# DELMOS: домен комплаєнсу — Telecom (Телекомунікації: оператори та обладнання)

Дата: 2026-09-23. Статус: запланований напрямок розширення.
Ліцензія: Apache 2.0.
Контекст: [domains/README.md](README.md), [COMMODITY_NETWORKS_WIFI.md](COMMODITY_NETWORKS_WIFI.md) (суміжний споживчий сегмент),
[INFORMATION_SECURITY_CYBERSECURITY.md](INFORMATION_SECURITY_CYBERSECURITY.md).

---

## 1. Огляд домену

Телекомунікаційний домен охоплює операторів ліцензованого спектра та постачальників мережевого обладнання операторського класу (базові станції, ядро мережі 4G/5G). На відміну від [COMMODITY_NETWORKS_WIFI.md](COMMODITY_NETWORKS_WIFI.md) (нерегульований/легко ліцензований споживчий спектр), тут діють вимоги ліцензування спектра, законного перехоплення (Lawful Interception) та звітності про аварійні простої мережі перед регулятором.

## 2. Ключові стандарти

| Стандарт / Орган | Область застосування |
| --- | --- |
| 3GPP (усі релізи) | Технічні специфікації мобільних мереж (4G LTE, 5G NR) |
| ETSI EN 300 / 301 (серії) | Гармонізовані стандарти радіообладнання ЄС |
| ITU-T Рекомендації | Міжнародні телекомунікаційні стандарти взаємодії мереж |
| EU Radio Equipment Directive (RED) 2014/53/EU | Директива ЄС про радіообладнання (маркування CE, суттєві вимоги) |
| FCC Part 22 / 24 / 27 | Ліцензовані послуги мобільного зв'язку в США (на відміну від Part 15 для нелізензованого спектра) |
| ETSI TS 101 671 / 3GPP TS 33.107 (ЄС), CALEA (США) | Вимоги законного перехоплення (Lawful Interception) |
| FCC NORS (Network Outage Reporting System) | Обов'язкова звітність операторів про аварійні простої мережі (США) |
| GSMA NESAS (Network Equipment Security Assurance Scheme) | Схема оцінки безпеки мережевого обладнання операторського класу |

## 3. Модуль(і) комплаєнсу

### Плановий модуль: `compliance.telecom_network_operator`
- **Внесок у Project Plan:** розділ `Spectrum Licensing & Network Assurance Plan`.
- **Словники:** `SPECTRUM_LICENSE_TYPE`, `LAWFUL_INTERCEPT_JURISDICTION`.
- **Майлстоуни:** `network_outage_report_filing`, `nesas_security_assessment`.
- **Артефакти:** `telecom:type_approval_certificate` (відповідність RED), `telecom:lawful_intercept_capability_record`, `telecom:network_outage_report`.
- **Правила:** `rule.telecom.outage_report_deadline`: запис `network_outage_report` фіксує обов'язковий регуляторний дедлайн подання (наприклад, години з моменту виявлення інциденту).

## 4. Напрямок розширення

* Задіяні провайдери: `WPTypeProvider` (`type_approval_certificate`, `network_outage_report`), `DictionaryProvider` (типи ліцензій спектра, юрисдикції законного перехоплення), `MilestoneProvider` (оцінка NESAS), `TriggerRuleProvider` (дедлайн звітності про простій).
* Наскрізний архітектурний патерн: «дедлайн обов'язкового регуляторного подання» (`network_outage_report`, SAR у [FINANCIAL.md](FINANCIAL.md), сповіщення про витік у [PERSONAL_DATA_PROTECTION.md](PERSONAL_DATA_PROTECTION.md)) повторюється у трьох доменах — кандидат на узагальнений `RegulatoryFilingDeadline` провайдер ядра замість окремої реалізації в кожному модулі.

## 5. Посилання

* [COMMODITY_NETWORKS_WIFI.md](COMMODITY_NETWORKS_WIFI.md) — суміжний домен нелізензованого споживчого спектра.
* [INFORMATION_SECURITY_CYBERSECURITY.md](INFORMATION_SECURITY_CYBERSECURITY.md) — наскрізний мінімум кібербезпеки, доповнений NESAS.
* [STANDARD_MAPPING_TEMPLATE.md](../STANDARD_MAPPING_TEMPLATE.md) — шаблон формального мапування.
