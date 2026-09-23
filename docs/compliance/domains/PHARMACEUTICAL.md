# DELMOS: домен комплаєнсу — Pharmaceutical (Фармацевтика)

Дата: 2026-09-23. Статус: запланований напрямок розширення.
Ліцензія: Apache 2.0.
Контекст: [domains/README.md](README.md), [BIOLOGY_RESEARCH.md](BIOLOGY_RESEARCH.md) (спільний патерн валідації комп'ютеризованих систем),
[MEDICAL.md](MEDICAL.md).

---

## 1. Огляд домену

Фармацевтичний домен (розробка, виробництво, дистрибуція лікарських засобів) є одним із найсуворіше регульованих серед усіх — парасольковий термін **GxP** (Good x Practice: GMP, GLP, GCP, GDP) вимагає, щоб кожна комп'ютеризована система, залучена до виробництва чи контролю якості, проходила формальну валідацію з незмінним аудиторським слідом. Це напряму перетинається з базовими інваріантами самого DELMOS (незмінні ревізії, `payload_hash`, SoD), що робить платформу природним кандидатом для ведення доказів GxP-валідації.

## 2. Ключові стандарти

| Стандарт / Настанова | Область застосування |
| --- | --- |
| FDA 21 CFR Part 210/211 | Поточна належна виробнича практика (cGMP) для лікарських засобів (США) |
| FDA 21 CFR Part 11 | Електронні записи та електронні підписи для регульованих систем |
| EU GMP Annex 11 | Вимоги до комп'ютеризованих систем у виробництві лікарських засобів (ЄС) |
| ICH Q7–Q10 | Настанови якості: GMP для АФІ (Q7), управління ризиком якості (Q9), фармацевтична система якості (Q10) |
| GAMP 5 (ISPE) | Методологія валідації автоматизованих виробничих систем на основі категорій ризику ПЗ (1–5) |
| US DSCSA (Drug Supply Chain Security Act) | Серіалізація та простежуваність постачання лікарських засобів (США) |
| EU FMD (Falsified Medicines Directive 2011/62/EU) | Обов'язкова 2D-серіалізація упаковки лікарських засобів (ЄС) |
| ICH E2 (серія) / EudraVigilance | Фармаконагляд — звітність про побічні реакції |
| EU GDP (2013/C 343/01) | Належна дистриб'юторська практика лікарських засобів |

## 3. Модуль(і) комплаєнсу

### Плановий модуль: `compliance.gxp_computerized_systems`
- **Внесок у Project Plan:** розділ `GxP Computerized System Validation (CSV) Plan`.
- **Словники:** `GXP_CATEGORY` (`GMP`, `GLP`, `GCP`, `GDP`), `GAMP5_SOFTWARE_CATEGORY` (`1`..`5`), `VALIDATION_RISK_CLASS`.
- **Майлстоуни:** `iq_oq_pq_completion` (Installation/Operational/Performance Qualification), `periodic_review`.
- **Артефакти:** `gxp:validation_master_plan`, `gxp:iq_oq_pq_protocol`, `gxp:audit_trail_review_record`, `gxp:electronic_signature_record` (21 CFR Part 11).
- **Правила:** `rule.gxp.audit_trail_immutable_and_reviewed`: аудиторський слід підлягає обов'язковому періодичному перегляду; `rule.gxp.electronic_signature_requires_two_factor`: електронний підпис вимагає двофакторної автентифікації.

### Плановий модуль: `compliance.serialization_track_trace`
- **Словники:** `SERIALIZATION_FORMAT` (GS1 DataMatrix), `MARKET_JURISDICTION` (`US_DSCSA`, `EU_FMD`).
- **Артефакти:** `pharma:serial_number_registry_record`, `pharma:aggregation_record`.

## 4. Напрямок розширення

* Задіяні провайдери: `WPTypeProvider` (`validation_master_plan`, `electronic_signature_record`, `serial_number_registry_record`), `DictionaryProvider` (категорії GAMP 5, юрисдикції серіалізації), `MilestoneProvider` (завершення IQ/OQ/PQ), `TriggerRuleProvider` (незмінність та періодичний перегляд аудиторського сліду, двофакторний підпис).
* **Природна відповідність архітектурі DELMOS:** вимога 21 CFR Part 11 щодо незмінних електронних записів із аудиторським слідом майже дослівно збігається з уже реалізованим інваріантом платформи — незмінні ревізії `WorkProductRevision` з `payload_hash` та перевіркою SoD (див. [WORK_PRODUCTS.md](../../architecture/WORK_PRODUCTS.md)). Це означає, що базова інфраструктура доказовості GxP вимагає мінімального додаткового розширення понад ядро — лише електронний підпис із двофакторною автентифікацією є новою функціональністю.
* Спільний патерн із [BIOLOGY_RESEARCH.md](BIOLOGY_RESEARCH.md): методологія IQ/OQ/PQ повторно використовується для валідації лабораторних систем (LIMS).

## 5. Посилання

* [BIOLOGY_RESEARCH.md](BIOLOGY_RESEARCH.md) — спільний патерн валідації комп'ютеризованих систем.
* [MEDICAL.md](MEDICAL.md) — суміжний домен медичних виробів (IEC 62304).
* [architecture/WORK_PRODUCTS.md](../../architecture/WORK_PRODUCTS.md) — інваріант незмінності ревізій, що природно відповідає 21 CFR Part 11.
* [STANDARD_MAPPING_TEMPLATE.md](../STANDARD_MAPPING_TEMPLATE.md) — шаблон формального мапування.
