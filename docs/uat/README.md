# DELMOS: приймальне тестування (User Acceptance Testing, UAT)

Дата: 2026-09-23. Статус: нормативний реєстр сценаріїв приймального тестування.
Ліцензія: Apache 2.0.
Контекст: [стратегія тестування](../testing/TEST_STRATEGY.md), [каталог базових ролей](../use-cases/README.md), [use-cases: Project Manager](../use-cases/PROJECT_MANAGER.md),
[use-cases: Administrator](../use-cases/ADMINISTRATOR.md).

---

## 1. Модель статусів

| Статус | Значення |
| --- | --- |
| `Draft` | Сценарій описаний, але ще жодного разу не виконувався |
| `Passed` | Останнє виконання підтвердило очікувані результати без зауважень |
| `Failed` | Останнє виконання виявило розбіжність з очікуваним результатом |
| `Blocked` | Виконання неможливе через відсутню або незавершену функціональність |

## 2. Реєстр сценаріїв

| ID | Назва | Роль | Перевіряє | Статус |
| --- | --- | --- | --- | --- |
| [UAT-001](UAT-001-project-creation-and-generic-plan.md) | Створення проєкту з автоматичним Generic Project Plan | Project Manager / System Administrator | `SHR-01`, `SHR-09` (PM-001, PM-002, ADM-006) | Blocked (v1.0.0); кореневий інваріант підтверджено автотестами |
| [UAT-002](UAT-002-composite-specification-authoring.md) | Авторинг композитної специфікації та валідація посилань | Requirements Engineer | `SHR-11` (SPEC-01 сценарії) | Blocked (потребує `v1.x`) |
| [UAT-003](UAT-003-module-activation-and-plan-apply.md) | Активація модуля комплаєнсу та застосування плану | Project Manager | `SHR-02`, `SHR-03` (PM-005, PM-006, PM-013, PM-014) | Blocked (потребує `v1.x`) |
| [UAT-004](UAT-004-core-roles-review-approval.md) | Базові ролі й незалежне погодження без галузевих модулів | Admin / PM / Engineer / Reviewer / Approver / Auditor / Viewer | `SHR-14`, `SWR-42..48`; частково `SHR-09`, `SWR-28` | Blocked (Review/Approval потребує `v1.x`); RBAC/ізоляція проєктів підтверджено автотестами |

## 3. Покриття стейкхолдерських вимог

| SHR | Назва | Покрито сценарієм |
| --- | --- | --- |
| SHR-01 | Generic Project Plan | UAT-001 |
| SHR-02 | Module System Policy | UAT-003 |
| SHR-03 | Project Module Activation | UAT-003 |
| SHR-09 | Work Item and Work Product | UAT-001, UAT-004 (часткове покриття SoD) |
| SHR-11 | Composite Specifications | UAT-002 |
| SHR-14 | Базовий доступ і незалежне погодження | UAT-004 |

Решта стейкхолдерських вимог (`SHR-04`..`SHR-08`, `SHR-10`, `SHR-12`, `SHR-13`) наразі не мають затвердженого сценарію приймання — це визнаний пробіл, що закривається за допомогою [шаблону UAT-TEMPLATE.md](../templates/UAT-TEMPLATE.md) під час подальшої розробки відповідних підсистем.

## 4. Правило виконання

Сценарії `UAT-NNN` виконуються перед кожним мінорним і мажорним релізом (див. [TEST_STRATEGY.md, розділ 3](../testing/TEST_STRATEGY.md#3-стратегія-регресії)). Результат виконання (Passed/Failed) та докази фіксуються безпосередньо у файлі відповідного сценарію.
