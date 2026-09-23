# DELMOS: домен комплаєнсу — Marine & Offshore (Морська та офшорна техніка)

Дата: 2026-09-23. Статус: запланований напрямок розширення.
Ліцензія: Apache 2.0.
Контекст: [domains/README.md](README.md), [каталог модулів](../../architecture/MODULE_CATALOG.md).

---

## 1. Огляд домену

Морська та офшорна галузь (суднобудування, офшорні платформи, морська робототехніка) регулюється класифікаційними товариствами (DNV, Lloyd's Register, ABS, Bureau Veritas) та міжнародною морською організацією (IMO). Функціональна безпека спирається на генеричний стандарт IEC 61508, адаптований класифікаційними правилами до специфіки суднових систем управління.

## 2. Ключові стандарти

| Стандарт | Область застосування |
| --- | --- |
| IEC 61508 (усі частини) | Генерична функціональна безпека електричних/електронних/програмованих електронних систем (SIL 1–4) |
| IACS UR E22 | Вимоги до систем управління на основі програмованого електронного обладнання на суднах |
| DNV-GL / Lloyd's Register Rules | Класифікаційні правила для суднових систем автоматизації та навігації |
| IMO SOLAS Chapter V | Безпека мореплавства, вимоги до навігаційного обладнання |
| IEC 60945 | Загальні вимоги до морського навігаційного та радіозв'язкового обладнання (умови довкілля, EMC) |

## 3. Модуль(і) комплаєнсу

### Плановий модуль: `compliance.iec61508`
- **Внесок у Project Plan:** розділ `Functional Safety Lifecycle Plan` (за V-моделлю IEC 61508 частина 1).
- **Словники:** `SIL` (Safety Integrity Level, `SIL1`..`SIL4`), `SAFETY_LIFECYCLE_PHASE`.
- **Майлстоуни:** `safety_requirements_specification_freeze`, `functional_safety_assessment`.
- **Артефакти:** `iec61508:safety_requirements_specification`, `iec61508:sil_determination_record`, `iec61508:proof_test_procedure`.
- **Правила:** `rule.iec61508.sil_determination_traceable`: рівень SIL повинен мати простежуване обґрунтування (аналіз ризику LOPA/HAZOP).

### Плановий модуль: `compliance.marine_classification`
- **Словники:** `CLASS_SOCIETY` (DNV, LR, ABS, BV), `EQUIPMENT_TYPE_APPROVAL_STATUS`.
- **Артефакти:** `marine:type_approval_certificate`, `marine:sea_trial_report`.
- **Правила:** `rule.marine.type_approval_required_before_installation`: обладнання не може бути позначене як встановлене без дійсного сертифіката типового схвалення.

## 4. Напрямок розширення

* Задіяні провайдери: `WPTypeProvider` (`safety_requirements_specification`, `sil_determination_record`), `DictionaryProvider` (шкала SIL, реєстр класифікаційних товариств), `MilestoneProvider` (Sea Trial, Functional Safety Assessment), `TriggerRuleProvider` (обов'язковість обґрунтування SIL).
* Прогалина: класифікаційні товариства мають власні (частково несумісні) варіації правил — модуль потребує механізму `tailoring` для вибору конкретного класифікаційного товариства в межах одного проєкту (аналогічно `TailoringSystem`, описаному для ASPICE).

## 5. Посилання

* [MODULE_CATALOG.md](../../architecture/MODULE_CATALOG.md) — зразок структури галузевого модуля.
* [STANDARD_MAPPING_TEMPLATE.md](../STANDARD_MAPPING_TEMPLATE.md) — шаблон формального мапування.
