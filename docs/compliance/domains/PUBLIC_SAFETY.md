# DELMOS: домен комплаєнсу — Public Safety (Системи громадської безпеки)

Дата: 2026-09-23. Статус: запланований напрямок розширення.
Ліцензія: Apache 2.0.
Контекст: [domains/README.md](README.md), [каталог модулів](../../architecture/MODULE_CATALOG.md).

---

## 1. Огляд домену

Системи громадської безпеки (служби екстреного виклику 911/112, радіозв'язок аварійних служб, системи пожежної сигналізації та охоронно-тривожної сигналізації, системи масового оповіщення) вимагають надзвичайно високої готовності (Availability) та детермінованого часу реакції, оскільки відмова прямо загрожує життю людей. Регулювання поєднує технічні стандарти обладнання (NFPA, EN 54) з операційними стандартами управління кризовими ситуаціями (ISO 22320).

## 2. Ключові стандарти

| Стандарт | Область застосування |
| --- | --- |
| NFPA 1221 | Установка, обслуговування та використання систем зв'язку служб екстреного реагування (США) |
| ANSI/TIA-102 (Project 25 / P25) | Стандарт цифрового радіозв'язку для служб громадської безпеки (Північна Америка) |
| EN 54 (усі частини) | Системи пожежної сигналізації та оповіщення про пожежу (ЄС) |
| IEC 60839 | Системи охоронно-тривожної сигналізації та контролю доступу |
| ISO 22320 | Управління надзвичайними ситуаціями — вимоги до реагування на інциденти |
| NENA i3 (US) / EENA (EU) | Архітектурні стандарти служб екстреного виклику наступного покоління (Next Generation 911/112) |

## 3. Модуль(і) комплаєнсу

### Плановий модуль: `compliance.public_safety_availability`
- **Внесок у Project Plan:** розділ `Emergency Communication Availability & Response Time Plan`.
- **Словники:** `AVAILABILITY_CLASS` (за NFPA 1221: `Class I`..`Class IV`), `RESPONSE_TIME_TARGET` (секунди до відповіді диспетчера).
- **Майлстоуни:** `system_acceptance_test`, `failover_drill` (регулярне навчання перемикання на резервний центр).
- **Артефакти:** `public_safety:availability_design_record`, `public_safety:failover_drill_report`, `public_safety:incident_response_procedure`.
- **Правила:** `rule.public_safety.dual_path_redundancy_required`: критичний канал зв'язку не приймається без підтвердженого дублювання шляху передачі (`dual path`).

### Плановий модуль: `compliance.fire_alarm_en54`
- **Словники:** `EN54_COMPONENT_CATEGORY` (детектори, панелі керування, джерела живлення).
- **Артефакти:** `en54:type_approval_certificate`, `en54:false_alarm_rate_record`.

## 4. Напрямок розширення

* Задіяні провайдери: `WPTypeProvider` (`availability_design_record`, `failover_drill_report`), `DictionaryProvider` (класи готовності NFPA, категорії компонентів EN 54), `MilestoneProvider` (регулярні навчання відмовостійкості, аналогічно [DISASTER_RECOVERY.md](../../operations/DISASTER_RECOVERY.md)), `TriggerRuleProvider` (обов'язковість дублювання шляху зв'язку).
* Природний перетин з операційною документацією платформи: концепція `failover_drill` тут прямо повторює патерн **DR Drill**, вже описаний для самої платформи DELMOS у [operations/DISASTER_RECOVERY.md, розділ 3](../../operations/DISASTER_RECOVERY.md#3-регулярне-навчання-з-відновлення-dr-drill) — рекомендовано повторно використати той самий шаблон регулярного навчання для інженерних артефактів клієнтського проєкту.
* Прогалина: платформа не має вбудованої метрики «час реакції» (response time) як окремого класу вимірювання в реальному часі — потрібне розширення [METRICS.md](../../architecture/METRICS.md) класом метрик з потоковим (near-real-time) оновленням, а не лише періодичним спостереженням.

## 5. Посилання

* [MODULE_CATALOG.md](../../architecture/MODULE_CATALOG.md) — зразок структури галузевого модуля.
* [operations/DISASTER_RECOVERY.md](../../operations/DISASTER_RECOVERY.md) — шаблон регулярного навчання відмовостійкості (DR Drill), придатний для `failover_drill`.
* [architecture/METRICS.md](../../architecture/METRICS.md) — реєстр метрик, що потребує розширення.
* [STANDARD_MAPPING_TEMPLATE.md](../STANDARD_MAPPING_TEMPLATE.md) — шаблон формального мапування.
