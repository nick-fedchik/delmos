# SWR-02: Системні вимоги до динамічних полів, типів та контенту

Дата: 2026-09-23. Статус: нормативні вимоги підсистеми Dynamic Fields & Content Models.  
Ліцензія: Apache 2.0.  
Батьківські вимоги: [SHR-04](../stakeholder/SHR-04-dynamic-content-fields.md), [SHR-09](../stakeholder/SHR-09-work-item-and-work-product.md).  
Пов'язані архітектурні рішення: [ADR-002](../../architecture/decisions/ADR-002-strict-workitem-workproduct-separation.md).

---

#### SWR-06: Реєстрація нових типів Work Products модулями
1. Модуль може реєструвати нові типи WP у `wp_type_definitions` із префіксним неймспейсом (наприклад, `iso26262:safety_goal`, `economics:cost_baseline_record`).
2. Для кожного типу модуль надає валідну JSON Schema Draft 2020-12 для перевірки `metadata`.

#### SWR-07: Розширення існуючих типів Work Products (Property Extensions)
1. Модуль може додавати типізовані властивості до базових типів WP ядра (`plan`, `requirement`, `architecture`, `test_spec`, `report`, `record`).
2. Властивості реєструються в `wp_property_definitions`: назва, тип значення, правила перевірки та перелік статусів, за яких поле стає обов'язковим (`required_on`).

#### SWR-08: Збереження та ізоляція динамічних даних
1. Дані розширених полів зберігаються в реляційній колонці `work_products.metadata` (JSONB) та дублюються в кожній immutable ревізії `work_product_revisions.metadata`.
2. Жодне розширення не має права змінювати базові колонки ядра (`id`, `project_id`, `code`, `status`, `row_version`).
3. Вимкнення модуля зберігає значення полів в історії та архівних бейзлайнах.

#### SWR-28: Розмежування Work Item та Work Product
1. `Task` належить до родини `WorkItem`. `WP` означає виключно `WorkProduct`.
2. Виконання задачі не є автоматичним схваленням артефакту (`Task.done != WP.approved`). Завершення роботи й затвердження ревізії мають незалежні команди, ролі та докази.
3. Зв'язок `WorkItemProductLink` підтримує зв'язок багато-до-багатьох із семантичними типами (`produces`, `updates`, `implements`, `verifies`, `reviews`).

#### SWR-29: Типи контенту, профілі та шаблони
1. `ContentTypeDescriptor` проєктує родину, тип і версію для інтерфейсів відображення та пошуку.
2. Шаблон надає початковий вміст, але не є екземпляром створеного артефакту.

#### SWR-30: Колекції типізованих посилань
1. Колекція має явний, динамічний (запит) або знімковий (snapshot) режим.
2. Видалення колекції не видаляє включені сутності.
