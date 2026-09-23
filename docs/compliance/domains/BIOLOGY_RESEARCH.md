# DELMOS: домен комплаєнсу — Biology Research (Біологічні дослідження)

Дата: 2026-09-23. Статус: запланований напрямок розширення.
Ліцензія: Apache 2.0.
Контекст: [domains/README.md](README.md), [PHARMACEUTICAL.md](PHARMACEUTICAL.md) (спільний патерн валідації комп'ютеризованих систем),
[MEDICAL.md](MEDICAL.md).

---

## 1. Огляд домену

Домен біологічних досліджень (академічні лабораторії, біотех, дослідницькі підрозділи фармацевтичних компаній) поєднує фізичну біобезпеку лабораторного середовища з вимогами цілісності дослідницьких даних та етичного нагляду за дослідженнями за участю людей чи генетично модифікованих організмів. Програмний вимір — валідація лабораторних інформаційних систем (LIMS), що має спільний патерн із [PHARMACEUTICAL.md](PHARMACEUTICAL.md) (GAMP 5 / 21 CFR Part 11).

## 2. Ключові стандарти

| Стандарт / Настанова | Область застосування |
| --- | --- |
| WHO Laboratory Biosafety Manual (4th ed.) | Класифікація рівнів біобезпеки (BSL-1..BSL-4) |
| CDC/NIH BMBL (Biosafety in Microbiological and Biomedical Laboratories) | Деталізовані вимоги біобезпеки лабораторій (США) |
| OECD Principles of Good Laboratory Practice (GLP) | Забезпечення якості неклінічних досліджень безпеки |
| NIH Guidelines for Research Involving Recombinant or Synthetic Nucleic Acid Molecules | Нагляд Institutional Biosafety Committee (IBC) за генно-інженерними дослідженнями |
| Common Rule (45 CFR 46, США) / ICH E6(R2) GCP | Етичний нагляд за дослідженнями за участю людини (IRB/Ethics Committee) |
| CDISC (Clinical Data Interchange Standards Consortium) | Стандартизовані формати обміну дослідницькими клінічними даними |
| FDA 21 CFR Part 11 | Електронні записи та підписи в регульованих лабораторних системах (спільно з [PHARMACEUTICAL.md](PHARMACEUTICAL.md)) |

## 3. Модуль(і) комплаєнсу

### Плановий модуль: `compliance.biosafety_glp`
- **Внесок у Project Plan:** розділ `Biosafety, Ethics & Laboratory Data Integrity Plan`.
- **Словники:** `BIOSAFETY_LEVEL` (`BSL-1`..`BSL-4`), `GLP_STUDY_PHASE`.
- **Майлстоуни:** `ibc_protocol_approval`, `glp_study_audit`, `irb_ethics_review`.
- **Артефакти:** `biology:biosafety_protocol`, `biology:ibc_approval_record`, `biology:glp_study_report`, `biology:lims_validation_record` (валідація Laboratory Information Management System).
- **Правила:** `rule.biosafety.bsl_containment_matches_protocol`: рівень фізичного стримування лабораторії повинен відповідати заявленому рівню біобезпеки протоколу; невідповідність блокує (`veto`) затвердження протоколу.

## 4. Напрямок розширення

* Задіяні провайдери: `WPTypeProvider` (`biosafety_protocol`, `lims_validation_record`), `DictionaryProvider` (рівні BSL, фази GLP-дослідження), `MilestoneProvider` (затвердження IBC/IRB), `TriggerRuleProvider` (відповідність стримування рівню біобезпеки).
* Спільний патерн із [PHARMACEUTICAL.md](PHARMACEUTICAL.md): валідація комп'ютеризованої лабораторної системи (`lims_validation_record`) використовує ту саму методологію IQ/OQ/PQ (Installation/Operational/Performance Qualification), що й `gxp:iq_oq_pq_protocol` — обидва домени можуть повторно використовувати один модуль валідації комп'ютеризованих систем.

## 5. Посилання

* [PHARMACEUTICAL.md](PHARMACEUTICAL.md) — спільний патерн валідації комп'ютеризованих систем (GAMP 5, 21 CFR Part 11).
* [MEDICAL.md](MEDICAL.md) — суміжний домен для досліджень, що переходять у розробку медичного виробу.
* [STANDARD_MAPPING_TEMPLATE.md](../STANDARD_MAPPING_TEMPLATE.md) — шаблон формального мапування.
