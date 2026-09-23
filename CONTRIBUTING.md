# Внесок у розвиток DELMOS (Contributing)

Дякуємо за інтерес до участі в розвитку **DELMOS**. Цей документ — короткий вхідний путівник; детальні інструкції винесені в [docs/guides/](docs/guides/README.md).

## 1. Перш ніж почати

1. Ознайомтеся з [бачення продукту (VISION.md)](docs/guides/VISION.md) та [ARCHITECTURE.md](docs/architecture/ARCHITECTURE.md).
2. Перегляньте [GOVERNANCE.md](GOVERNANCE.md), щоб розуміти поточну модель прийняття рішень.
3. Перевірте наявні [issues](https://github.com/nick-fedchik/delmos/issues) та [discussions](https://github.com/nick-fedchik/delmos/discussions) репозиторію <https://github.com/nick-fedchik/delmos>, щоб уникнути дублювання роботи.

## 2. Локальне середовище розробки

Повна інструкція: [docs/guides/developer/LOCAL_SETUP.md](docs/guides/developer/LOCAL_SETUP.md).

## 3. Процес контриб'юції

Стиль коду, конвенції комітів, чек-лист Merge Request: [docs/guides/developer/CONTRIBUTING_WORKFLOW.md](docs/guides/developer/CONTRIBUTING_WORKFLOW.md).

Стисло:

* Повідомлення комітів починаються з великої літери, наказовий спосіб, без префіксів на кшталт `feat:`/`fix:`.
* Після появи Makefile перед відкриттям Merge Request із кодом локально виконати `make validate`; поки DELMOS містить лише документацію, цей gate є цільовою вимогою, не доступною командою.
* Будь-яка зміна контракту (API, схема БД, поведінка) супроводжується оновленням відповідного документа вимог (`SWR-NN`) чи специфікації (`SPEC-NN`).

## 4. Пропозиція змін до вимог та архітектури

Нові вимоги, архітектурні рішення чи специфікації оформлюються за шаблонами з [docs/templates/](docs/templates/README.md) (`SHR-TEMPLATE.md`, `SWR-TEMPLATE.md`, `ADR-TEMPLATE.md`, `SPEC-TEMPLATE.md`) і додаються до відповідного реєстру (`README.md` теки).

## 5. Звітування про дефекти безпеки

Дефекти безпеки **не повідомляються** через публічні issues — див. [SECURITY.md](SECURITY.md).

## 6. Кодекс поведінки

Участь у проєкті регулюється [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

## 7. Ліцензія внеску

Надсилаючи Merge Request, ви погоджуєтесь, що ваш внесок ліцензується на умовах [Apache License 2.0](LICENSE), як і решта проєкту.
