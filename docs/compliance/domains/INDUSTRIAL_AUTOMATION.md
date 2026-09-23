# DELMOS: домен комплаєнсу — Industrial Automation & Machinery (Промислова автоматизація та машинобудування)

Дата: 2026-09-23. Статус: запланований напрямок розширення.
Ліцензія: Apache 2.0.
Контекст: [domains/README.md](README.md), [каталог модулів](../../architecture/MODULE_CATALOG.md).

---

## 1. Огляд домену

Промислова автоматизація охоплює виробниче обладнання, роботизовані системи та системи безпеки процесів (Process Industry Safety Instrumented Systems). Це найбільш узагальнений (generic) домен функціональної безпеки — саме звідси походить IEC 61508, від якого успадковані галузеві варіанти (автомобільний ISO 26262, залізничний EN 5012x, морський IEC 61508+IACS).

## 2. Ключові стандарти

| Стандарт | Область застосування |
| --- | --- |
| IEC 61508 (усі частини) | Генерична функціональна безпека E/E/PE систем (SIL 1–4) |
| IEC 62061 | Функціональна безпека систем керування машин (похідний від IEC 61508) |
| ISO 13849-1/2 | Безпека машин — деталі систем керування, пов'язаних з безпекою (Performance Level PL a–e) |
| IEC 61511 | Системи безпеки (SIS) для процесної промисловості (нафтогаз, хімія) |
| ISO 10218 / ISO/TS 15066 | Безпека промислових роботів та колаборативної робототехніки (Cobots) |

## 3. Модуль(і) комплаєнсу

### Плановий модуль: `compliance.iec61508_generic`
- **Внесок у Project Plan:** розділ `Functional Safety Lifecycle Plan` (спільний з морським доменом, але з іншим набором словників тайлорингу).
- **Словники:** `SIL` (`SIL1`..`SIL4`), `PERFORMANCE_LEVEL` (`PLa`..`PLe`, для ISO 13849).
- **Майлстоуни:** `hazop_review`, `functional_safety_assessment`.
- **Артефакти:** `iec61508:safety_requirements_specification`, `iec61511:sis_design_basis`, `iso13849:performance_level_calculation`.
- **Правила:** `rule.iec61508.proof_test_interval_defined`: кожна безпечна функція (`Safety Instrumented Function`) повинна мати визначений інтервал тестування (`proof test interval`).

### Плановий модуль: `compliance.robotics_safety`
- **Словники:** `COLLABORATIVE_OPERATION_MODE` (за ISO/TS 15066: Safety-rated monitored stop, Hand guiding, Speed and separation monitoring, Power and force limiting).
- **Артефакти:** `robotics:risk_assessment`, `robotics:collaborative_workspace_layout`.

## 4. Напрямок розширення

* Задіяні провайдери: `WPTypeProvider` (`safety_requirements_specification`, `sis_design_basis`, `performance_level_calculation`), `DictionaryProvider` (SIL/PL шкали), `MilestoneProvider` (HAZOP Review), `MetricProvider` (розрахунок `PFDavg`/`PFH` для SIS).
* Прогалина: розрахунок кількісних показників надійності (`PFDavg`, `PFH`) вимагає підключення модуля метрик до формул надійності компонентів (частотні характеристики відмов) — потенційне розширення [METRICS.md](../../architecture/METRICS.md) новим класом формул.

## 5. Посилання

* [MODULE_CATALOG.md](../../architecture/MODULE_CATALOG.md) — зразок структури галузевого модуля.
* [METRICS.md](../../architecture/METRICS.md) — реєстр метрик та формул.
* [STANDARD_MAPPING_TEMPLATE.md](../STANDARD_MAPPING_TEMPLATE.md) — шаблон формального мапування.
