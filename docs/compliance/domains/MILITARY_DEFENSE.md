# DELMOS: домен комплаєнсу — Military & Defense (Військова та оборонна техніка)

Дата: 2026-09-23. Статус: запланований напрямок розширення.
Ліцензія: Apache 2.0.
Контекст: [domains/README.md](README.md), [каталог модулів](../../architecture/MODULE_CATALOG.md).

---

## 1. Огляд домену

Військова та оборонна техніка (наземна, авіаційна, безпілотні системи) поєднує вимоги системної безпеки (MIL-STD-882E), стійкості до умов довкілля (MIL-STD-810) та, для авіаційних платформ, стандарти DO-178C/DO-254 у військовій адаптації. Додатковим виміром є контроль експортних обмежень (ITAR/EAR) та вимоги до цілісності ланцюга постачання компонентів.

## 2. Ключові стандарти

| Стандарт | Область застосування |
| --- | --- |
| MIL-STD-882E | Стандартна методологія системної безпеки (System Safety) для програм оборонного відомства США |
| MIL-STD-810H | Випробування на стійкість до умов довкілля (вібрація, удар, температура, вологість) |
| DEF STAN 00-56 | Британський еквівалент управління безпекою (Safety Management) для оборонних систем |
| STANAG 4671 (NATO) | Вимоги льотної придатності для безпілотних авіаційних систем (UAS) класу до 20 кг |
| DO-178C / DO-254 (Military profile) | Розробка бортового ПЗ/HW для військових авіаційних платформ |

## 3. Модуль(і) комплаєнсу

### Плановий модуль: `compliance.mil_std_882e`
- **Внесок у Project Plan:** розділ `System Safety Program Plan (SSPP)`.
- **Словники:** `MISHAP_SEVERITY` (`Catastrophic`..`Negligible`), `MISHAP_PROBABILITY` (`Frequent`..`Improbable`), `RISK_ASSESSMENT_CODE` (`1`..`5`).
- **Майлстоуни:** `preliminary_hazard_analysis_review`, `system_safety_working_group_review`.
- **Артефакти:** `mil882e:system_safety_program_plan`, `mil882e:preliminary_hazard_analysis`, `mil882e:hazard_tracking_log`.
- **Правила:** `rule.mil882e.hazard_open_until_mitigated`: запис у журналі небезпек (`hazard_tracking_log`) залишається у статусі `open` до підтвердженого зниження ризику.

### Плановий модуль: `compliance.export_control`
- **Призначення:** контроль обмежень ITAR/EAR на рівні артефакту та учасника проєкту.
- **Словники:** `EXPORT_CLASSIFICATION` (`ITAR`, `EAR99`, `Public Domain`).
- **Правила:** `rule.export_control.classification_gate`: доступ до артефакту з обмеженою класифікацією перевіряється проти громадянства/дозволу учасника (розширення ABAC поверх Scoped RBAC).

## 4. Напрямок розширення

* Задіяні провайдери: `WPTypeProvider` (`system_safety_program_plan`, `hazard_tracking_log`), `DictionaryProvider` (шкали Mishap Severity/Probability), `MilestoneProvider` (Working Group Review), `TriggerRuleProvider` (безперервне відстеження небезпек), а також **розширення моделі авторизації** ([ACCESS_CONTROL.md](../../architecture/ACCESS_CONTROL.md)) атрибутом класифікації експортного контролю.
* Прогалина: наявна модель ABAC (`classification: internal/restricted`) недостатньо гранульована для експортного контролю — потрібне окреме поле класифікації, не тотожне грифу конфіденційності.

## 5. Посилання

* [MODULE_CATALOG.md](../../architecture/MODULE_CATALOG.md) — зразок структури галузевого модуля.
* [ACCESS_CONTROL.md](../../architecture/ACCESS_CONTROL.md) — модель авторизації, що потребує розширення.
* [STANDARD_MAPPING_TEMPLATE.md](../STANDARD_MAPPING_TEMPLATE.md) — шаблон формального мапування.
