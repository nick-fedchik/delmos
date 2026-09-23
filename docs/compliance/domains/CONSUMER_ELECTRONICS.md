# DELMOS: домен комплаєнсу — Consumer Electronics (Побутові електронні пристрої)

Дата: 2026-09-23. Статус: запланований напрямок розширення.
Ліцензія: Apache 2.0.
Контекст: [domains/README.md](README.md), [IOT_CYBERSECURITY.md](IOT_CYBERSECURITY.md) (кібербезпека),
[COMMODITY_NETWORKS_WIFI.md](COMMODITY_NETWORKS_WIFI.md) (радіочастотна сертифікація).

---

## 1. Огляд домену

Домен побутової електроніки охоплює фізичну безпеку, електромагнітну сумісність (EMC) та екологічний слід пристрою впродовж усього життєвого циклу (виробництво → продаж → утилізація). Це доповнює, а не дублює [IOT_CYBERSECURITY.md](IOT_CYBERSECURITY.md) (кіберзахист) та [COMMODITY_NETWORKS_WIFI.md](COMMODITY_NETWORKS_WIFI.md) (радіочастоти): тут предметом є електробезпека, хімічний склад матеріалів та енергоефективність.

## 2. Ключові стандарти

| Стандарт / Регламент | Область застосування |
| --- | --- |
| IEC 62368-1 | Безпека аудіо/відео, ІТ та комунікаційного обладнання (замінив IEC 60950-1/60065) |
| EU Low Voltage Directive (LVD) 2014/35/EU + EMC Directive 2014/30/EU | Маркування CE: електробезпека та електромагнітна сумісність |
| RoHS Directive 2011/65/EU | Обмеження небезпечних речовин у складі електронного обладнання |
| WEEE Directive 2012/19/EU | Утилізація та переробка відходів електронного обладнання |
| REACH Regulation (EC) 1907/2006 | Реєстрація та контроль хімічних речовин у складі виробу |
| Ecodesign (ErP) Directive + EU Energy Label | Вимоги до енергоефективності та маркування споживання енергії |
| IEC 62133 / UN 38.3 | Безпека літієвих акумуляторів та вимоги до їх транспортування |
| UL 62368-1 / ETL | Північноамериканська сертифікація безпеки (альтернатива/доповнення до CE) |

## 3. Модуль(і) комплаєнсу

### Плановий модуль: `compliance.consumer_electronics_safety`
- **Внесок у Project Plan:** розділ `Product Safety, Chemical & Environmental Compliance Plan`.
- **Словники:** `SAFETY_CERTIFICATION_MARK` (CE, UL, CSA, PSE), `HAZARDOUS_SUBSTANCE_DECLARATION`.
- **Майлстоуни:** `ce_conformity_declaration`, `weee_registration`, `battery_transport_certification`.
- **Артефакти:** `consumer_electronics:declaration_of_conformity`, `consumer_electronics:rohs_test_report`, `consumer_electronics:battery_un38_3_test_report`.
- **Правила:** `rule.consumer_electronics.doc_required_before_market_release`: артефакт випуску на ринок блокується (`veto`) без чинної декларації відповідності (`declaration_of_conformity`).

## 4. Напрямок розширення

* Задіяні провайдери: `WPTypeProvider` (`declaration_of_conformity`, `rohs_test_report`), `DictionaryProvider` (реєстр сертифікаційних знаків, небезпечних речовин), `MilestoneProvider` (реєстрація WEEE), `TriggerRuleProvider` (обов'язковість декларації відповідності).
* Прогалина: контроль хімічного складу матеріалів (RoHS/REACH) вимагає трасування на рівні компонентів BOM (Bill of Materials), аналогічно SBOM для програмного забезпечення в [IOT_CYBERSECURITY.md](IOT_CYBERSECURITY.md) — природний спільний патерн «реєстр складових з декларацією відповідності на кожен компонент».

## 5. Посилання

* [IOT_CYBERSECURITY.md](IOT_CYBERSECURITY.md) — кібербезпека підключеного компонента та SBOM-патерн.
* [COMMODITY_NETWORKS_WIFI.md](COMMODITY_NETWORKS_WIFI.md) — радіочастотна сертифікація.
* [STANDARD_MAPPING_TEMPLATE.md](../STANDARD_MAPPING_TEMPLATE.md) — шаблон формального мапування.
