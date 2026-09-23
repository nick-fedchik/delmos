# DELMOS: домен комплаєнсу — Commodity Networks & Wi-Fi (Побутові мережі Інтернет та Wi-Fi)

Дата: 2026-09-23. Статус: запланований напрямок розширення.
Ліцензія: Apache 2.0.
Контекст: [domains/README.md](README.md), [TELECOM.md](TELECOM.md) (суміжний ліцензований сегмент),
[IOT_CYBERSECURITY.md](IOT_CYBERSECURITY.md).

---

## 1. Огляд домену

Побутове мережеве обладнання (Wi-Fi роутери, кабельні/оптичні модеми, mesh-системи) працює переважно в нелізензованому спектрі й регулюється радіочастотними та кібербезпековими вимогами, а не ліцензуванням оператора (на відміну від [TELECOM.md](TELECOM.md)). Із серпня 2025 року делегований акт до EU Radio Equipment Directive прямо поширив обов'язкові вимоги кібербезпеки на все радіообладнання зі здатністю підключення до інтернету, включно з побутовими роутерами — це напряму пов'язує домен із [IOT_CYBERSECURITY.md](IOT_CYBERSECURITY.md).

## 2. Ключові стандарти

| Стандарт / Програма | Область застосування |
| --- | --- |
| IEEE 802.11 (усі поправки) | Технічна база стандартів Wi-Fi (не регуляторний сам собою, але основа сертифікації) |
| Wi-Fi Alliance Certification (Wi-Fi CERTIFIED 6/6E/7, WPA3) | Програма сертифікації сумісності та безпеки Wi-Fi-пристроїв |
| FCC Part 15 | Нелізензовані навмисні радіовипромінювачі (роутери, Wi-Fi, Bluetooth) у США |
| EN 300 328 (2,4 ГГц) / EN 301 893 (5 ГГц) | Гармонізовані стандарти ЄС для широкосмугової передачі даних у нелізензованому спектрі |
| DOCSIS (CableLabs Certification) | Сертифікація кабельних модемів для широкосмугового доступу |
| EU RED delegated act (Art. 3(3)(d)/(e)/(f)) | Обов'язкова кібербезпекова базова лінія для радіообладнання з підключенням до мережі (з 2025 р.) |
| UL 2900-1 | Кібербезпека мережевих пристроїв, застосовна до побутового CPE-обладнання |

## 3. Модуль(і) комплаєнсу

### Плановий модуль: `compliance.commodity_network_equipment`
- **Внесок у Project Plan:** розділ `RF Type Approval & Wi-Fi Security Baseline Plan`.
- **Словники:** `WIFI_CERTIFICATION_PROGRAM` (Wi-Fi CERTIFIED 6/6E/7, WPA3), `RF_BAND_APPROVAL` (`2.4GHz`, `5GHz`, `6GHz`).
- **Майлстоуни:** `rf_type_approval`, `wifi_alliance_certification`.
- **Артефакти:** `commodity_net:rf_type_approval_certificate`, `commodity_net:wifi_alliance_certification_report`.
- **Правила:** `rule.commodity_net.red_cybersecurity_baseline_required`: пристрій не проходить типове схвалення без підтвердженої базової лінії кібербезпеки, еквівалентної ETSI EN 303 645.

## 4. Напрямок розширення

* Задіяні провайдери: `WPTypeProvider` (`rf_type_approval_certificate`), `DictionaryProvider` (програми сертифікації Wi-Fi Alliance, діапазони RF), `MilestoneProvider` (типове схвалення RF), `TriggerRuleProvider` (обов'язкова базова лінія кібербезпеки).
* Цей домен фактично є **композицією** двох горизонтальних вимог (радіочастотна сертифікація за зразком [TELECOM.md](TELECOM.md) + кібербезпека за зразком [IOT_CYBERSECURITY.md](IOT_CYBERSECURITY.md)) — окремих нових прогалин у предметній моделі не виявлено, повторно використовуються ті самі провайдери.

## 5. Посилання

* [TELECOM.md](TELECOM.md) — суміжний ліцензований сегмент та шаблон RF-сертифікації.
* [IOT_CYBERSECURITY.md](IOT_CYBERSECURITY.md) — базова лінія кібербезпеки підключеного пристрою.
* [STANDARD_MAPPING_TEMPLATE.md](../STANDARD_MAPPING_TEMPLATE.md) — шаблон формального мапування.
