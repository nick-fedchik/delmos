# DELMOS: домен комплаєнсу — Home Security (Побутові системи безпеки)

Дата: 2026-09-23. Статус: запланований напрямок розширення.
Ліцензія: Apache 2.0.
Контекст: [domains/README.md](README.md), [PUBLIC_SAFETY.md](PUBLIC_SAFETY.md) (спільна база IEC 60839),
[IOT_CYBERSECURITY.md](IOT_CYBERSECURITY.md) (кібербезпека підключеного компонента).

---

## 1. Огляд домену

Побутові та комерційні системи безпеки (охоронна сигналізація, відеоспостереження, розумні замки) поєднують фізичні апаратні стандарти (клас стійкості до злому) із вимогами кібербезпеки підключених пристроїв. Домен є спеціалізацією [PUBLIC_SAFETY.md](PUBLIC_SAFETY.md) (спільний базовий стандарт IEC 60839) для споживчого та комерційного, а не критичного інфраструктурного сегмента.

## 2. Ключові стандарти

| Стандарт | Область застосування |
| --- | --- |
| EN 50131 (усі частини) | Системи охоронної сигналізації — класи (Grade 1–4) за стійкістю до злому |
| UL 1023 / UL 639 | Побутові пристрої охоронної сигналізації (США) |
| IEC 60839 | Загальні вимоги до систем охоронно-тривожної сигналізації та контролю доступу (спільно з [PUBLIC_SAFETY.md](PUBLIC_SAFETY.md)) |
| CP01 / Secured by Design (Велика Британія) | Схвалення продукту фізичної безпеки правоохоронними органами |
| ONVIF | Індустріальний стандарт інтероперабельності IP-відеоспостереження (не регуляторний, але де-факто вимога сумісності) |
| UL 2900-2-3 | Кібербезпека систем безпеки життя та охоронних систем (спеціалізація UL 2900 для цього домену) |
| ETSI EN 303 645 | Базова кібербезпека підключеного компонента (див. [IOT_CYBERSECURITY.md](IOT_CYBERSECURITY.md)) |

## 3. Модуль(і) комплаєнсу

### Плановий модуль: `compliance.home_security_systems`
- **Внесок у Project Plan:** розділ `Physical Security Grade & Video Retention Plan`.
- **Словники:** `ALARM_GRADE` (`Grade 1`..`Grade 4` за EN 50131), `CAMERA_RESOLUTION_CLASS`.
- **Майлстоуни:** `intrusion_system_type_approval`, `false_alarm_rate_review`.
- **Артефакти:** `home_security:alarm_grade_certification`, `home_security:false_alarm_rate_record`, `home_security:video_retention_policy_record`.
- **Правила:** `rule.home_security.grade_matches_risk_assessment`: обраний клас (`Grade`) не приймається без супровідної оцінки ризику, що його обґрунтовує.

## 4. Напрямок розширення

* Задіяні провайдери: `WPTypeProvider` (`alarm_grade_certification`, `video_retention_policy_record`), `DictionaryProvider` (класи Grade EN 50131), `MilestoneProvider` (типове схвалення системи), `TriggerRuleProvider` (відповідність класу оцінці ризику).
* Прогалина: сертифікація фізичного апаратного пристрою зовнішньою випробувальною лабораторією (аналогічно [MARINE.md](MARINE.md), розділ «типове схвалення») ще не має уніфікованої сутності в предметній моделі — розглядається спільно з іншими доменами, що потребують «type approval» (Marine, Railway).

## 5. Посилання

* [PUBLIC_SAFETY.md](PUBLIC_SAFETY.md) — спільна база IEC 60839 та готовність систем екстреного реагування.
* [IOT_CYBERSECURITY.md](IOT_CYBERSECURITY.md) — базова кібербезпека підключеного компонента.
* [STANDARD_MAPPING_TEMPLATE.md](../STANDARD_MAPPING_TEMPLATE.md) — шаблон формального мапування.
