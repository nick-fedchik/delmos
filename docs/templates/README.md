# Шаблони документів DELMOS (Document Templates)

Дата: 2026-09-23. Статус: нормативний реєстр шаблонів для майбутнього заповнення.
Ліцензія: Apache 2.0.

---

## 1. Призначення

Ця директорія містить порожні (незаповнені) шаблони документів, які використовуються під час подальшої розбудови платформи DELMOS. Автор нового документа копіює відповідний шаблон, присвоює наступний вільний ідентифікатор у відповідному реєстрі та заповнює розділи змістом.

## 2. Реєстр шаблонів

| Шаблон | Призначення | Реєстр призначення |
| --- | --- | --- |
| [ADR-TEMPLATE.md](ADR-TEMPLATE.md) | Архітектурне рішення (Architecture Decision Record) | [docs/architecture/decisions/](../architecture/decisions/README.md) |
| [SHR-TEMPLATE.md](SHR-TEMPLATE.md) | Стейкхолдерська вимога | [docs/requirements/stakeholder/](../requirements/stakeholder/README.md) |
| [SWR-TEMPLATE.md](SWR-TEMPLATE.md) | Системна вимога до підсистеми | [docs/requirements/software/](../requirements/software/README.md) |
| [MEMO-TEMPLATE.md](MEMO-TEMPLATE.md) | Інженерний меморандум / концептуальна нотатка | [docs/memos/](../memos/README.md) |
| [SPEC-TEMPLATE.md](SPEC-TEMPLATE.md) | Технічна специфікація підсистеми | [docs/specifications/](../specifications/README.md) |
| [UAT-TEMPLATE.md](UAT-TEMPLATE.md) | Сценарій приймального тестування користувачем | [docs/uat/](../uat/README.md) |
| [RELEASE-NOTES-TEMPLATE.md](RELEASE-NOTES-TEMPLATE.md) | Нотатки до релізу версії | Корінь репозиторію, `CHANGELOG.md` |

## 3. Правила присвоєння ідентифікаторів

1. Новий ідентифікатор завжди є наступним вільним номером у межах свого префікса (`SHR-15`, `SWR-49`, `ADR-011`, `MEMO-005`, `UAT-005`). Для нових технічних специфікацій використовувати окремий `TECHSPEC-001` або `CORE-CONTRACT-002`, а не `SPEC-05`: `SPEC-05` уже позначає сценарій приймання композитної специфікації.
2. Ідентифікатор ніколи не використовується повторно, навіть якщо документ згодом архівується чи скасовується.
3. Після заповнення шаблону автор додає документ до відповідного `README.md` реєстру та оновлює матрицю простежуваності в [docs/TRACEABILITY.md](../TRACEABILITY.md).
