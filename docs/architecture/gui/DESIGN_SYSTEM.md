# DELMOS: дизайн-система та компоненти Web GUI

Дата: 2026-09-23. Статус: цільові рекомендації для реалізації Vue 3 Web GUI.
Ліцензія: Apache 2.0.
Контекст: [оболонка](CORE_SHELL.md), [Guided UI](GUIDED_EXPERIENCE.md), [стани та повідомлення](STATES_AND_FEEDBACK.md), [архітектура модулів](../MODULES.md).

---

## 1. Джерело правил

Для DELMOS орієнтиром є **GitLab Pajamas**: UX-патерни з `contents/patterns`, правила компонентів із `contents/components`, дизайн-токени/адаптивність із `contents/product-foundations`, тексти й доступність із `contents/content` та `contents/accessibility`. Для реалізації Vue 3 звіряти кожен компонент із установленою версією `@gitlab/ui` і її експортами: бібліотека історично містить Vue 2-сумісні компоненти, тому приклади з документації не є доказом працездатності у конфігурації DELMOS. Якщо сумісність компонента не підтверджена тестом, застосувати доступний семантичний HTML/Vue-адаптер з тим самим UX-контрактом, а не копіювати залежності чи невідомий проп.

Описувати семантичні ролі й композицію, не прив'язуватися до конкретного номера релізу, CSS-значень чи назв внутрішніх компонентів іншого застосунку. Компонент → перевірена утиліта `gl-*` → семантичний токен → власний CSS як останній варіант. Імена токенів, іконок, пропів і утиліт перевіряти у вихідному коді [GitLab UI](https://gitlab.com/gitlab-org/gitlab-services/design.gitlab.com/-/tree/main/packages/gitlab-ui) та [іконках](https://gitlab.com/gitlab-org/gitlab-services/design.gitlab.com/-/tree/main/packages/gitlab-svgs), не вгадувати з префікса.

## 2. Карта UX-джерел

| Завдання DELMOS | Правило Pajamas | Рішення для DELMOS |
| --- | --- | --- |
| Просторова ієрархія та панелі | [Layout](https://design.gitlab.com/product-foundations/layout), [spacing](https://design.gitlab.com/product-foundations/spacing) | Постійна оболонка, не вкладені картки; допомога як панель, коротка довідка як тимчасовий drawer. Контейнер визначає проміжки між елементами. |
| Ліва панель | [Navigation sidebar](https://design.gitlab.com/patterns/navigation-sidebar) | До двох рівнів, контекстні групи, впізнавані назви, індикатор активного пункту; без прихованих прав. |
| Нова сторінка й перший запис | [Empty states](https://design.gitlab.com/patterns/empty-states), [feature discovery](https://design.gitlab.com/patterns/feature-discovery) | Стан «немає даних» відмінний від «потрібне налаштування» й «немає збігів»; одна пріоритетна дія. |
| Складна форма й кроки | [Forms](https://design.gitlab.com/patterns/forms), [multi-step form](https://design.gitlab.com/components/form-multi-step), [progressive disclosure](https://design.gitlab.com/patterns/progressive-disclosure) | Лейбл + пояснення наслідку + валідація; складні потоки — URL-кроки з поверненням і збереженням введеного. |
| Пояснення значення | [Contextual help](https://design.gitlab.com/patterns/contextual-help), [popover](https://design.gitlab.com/components/popover), [drawer](https://design.gitlab.com/components/drawer) | Необхідне пояснення inline; невелике додаткове — popover; тимчасова довідка — drawer; постійна багатоконтекстна — права панель. |
| Вибір повідомлення | [Choosing a messaging pattern](https://design.gitlab.com/patterns/choosing-a-messaging-pattern), [saving and feedback](https://design.gitlab.com/patterns/saving-and-feedback) | Немає повідомлення без користі; inline для видимого результату, toast для непомітного підтвердження, alert для помилки/блокування. |
| Списки й аналітика | [Table](https://design.gitlab.com/components/table), [filtering](https://design.gitlab.com/patterns/filtering), [dashboards](https://design.gitlab.com/patterns/dashboards) | Таблиця для порівняння однотипних рядків, фільтри і сортування в URL; графіки доповнені таблицею/текстом, не тільки кольором. |
| Складний ризик/підтвердження | [Destructive actions](https://design.gitlab.com/patterns/destructive-actions), [modal](https://design.gitlab.com/components/modal) | Підтвердження небезпечної дії не замінює права, кворум і SoD; робоче редагування лишається на сторінці. |
| Зрозумілий текст | [UI text](https://design.gitlab.com/content/ui-text), [content and semantics](https://design.gitlab.com/accessibility/content-and-semantics) | Короткий текст про мету й наслідок, єдині терміни з [глосарію](../GLOSSARY.md), локалізація та змістовний текст помилки. |
| Клавіатура і фокус | [Keyboard-only](https://design.gitlab.com/accessibility/keyboard-only), [focus management](https://design.gitlab.com/accessibility/focus-management), [screen readers](https://design.gitlab.com/accessibility/screen-readers) | Фокус після переходу/закриття панелі, видимий outline, `aria-live` лише для значущих змін, доступні дії без hover. |

## 3. Карта компонентів для реалізації

Відповідність *можливостей* компонентам Pajamas, не гарантія наявності у майбутній збірці DELMOS:

| Потреба | Компонент/джерело | Обмеження |
| --- | --- | --- |
| Дії й іконки | [Button](https://design.gitlab.com/components/button), [iconography](https://design.gitlab.com/product-foundations/iconography) | Один primary у контексті; незнайома іконка має назву, клавіатурний фокус і пояснення. |
| Дані й стани | [Table](https://design.gitlab.com/components/table), [badge](https://design.gitlab.com/components/badge), [pagination](https://design.gitlab.com/components/pagination) | Бейдж лише для значущого статусу; цифри вирівняти для порівняння, таблиці адаптувати, не зменшувати шрифт. |
| Структура | [Breadcrumb](https://design.gitlab.com/components/breadcrumb), [tabs](https://design.gitlab.com/components/tabs), [accordion](https://design.gitlab.com/components/accordion) | Вкладки для рівноправних підрозділів; вкладку в URL, якщо адресна. |
| Форми | [Form group](https://design.gitlab.com/components/form-group), [form input](https://design.gitlab.com/components/form-input), [form select](https://design.gitlab.com/components/form-select), [checkbox](https://design.gitlab.com/components/checkbox) | Пов'язані label, description, invalid feedback; не сховати причину disabled у tooltip лише. |
| Стани | [Empty state](https://design.gitlab.com/patterns/empty-states), [skeleton loader](https://design.gitlab.com/components/skeleton-loader), [alert](https://design.gitlab.com/components/alert), [toast](https://design.gitlab.com/components/toast) | Скелет має відповідати структурі, alert не зникає перед усуненням проблеми, toast не містить дії. |
| Допомога | [Popover](https://design.gitlab.com/components/popover), [tooltip](https://design.gitlab.com/components/tooltip), [drawer](https://design.gitlab.com/components/drawer) | Для інтерактивного вмісту popover відкривається дією, tooltip — тільки короткий неінтерактивний текст. |

## 4. Візуальна мова й обмеження

Використовувати семантичні токени Pajamas для фону, тексту, status/feedback, відступів і focus; не дублювати hex-таблиці ризиків, кольори ASIL чи пульсацію кнопки без перевірки доступності. Обов'язкова читабельність при zoom, forced colors, світлій/темній темі та reduced motion. Різниця статусів передається текстом, підписом серії графіка та формою, не лише кольором. Компактний інженерний екран має залишатися сканованим: повторювані записи — таблиця, навігаційні розділи — не картки, деталі — окрема сторінка/панель. Не показувати на всіх кнопках іконку; виділяти лише ті, яким вона допомагає.

Перевіряти в репозиторії дизайн-системи [CSS guidance](https://design.gitlab.com/product-foundations/css), [semantic tokens](https://design.gitlab.com/product-foundations/design-tokens-using), [type](https://design.gitlab.com/product-foundations/type-fundamentals) і [heading hierarchy](https://design.gitlab.com/product-foundations/type-headings) перед написанням CSS. На release gate — видимий стан фокусу й скриншоти основних станів для desktop/mobile та light/dark; до побудови коду це вимога, не звіт про успішні тести.