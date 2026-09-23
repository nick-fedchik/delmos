# Модель прийняття рішень у проєкті DELMOS (Governance)

Дата: 2026-09-23. Статус: чинна модель для поточної (докодової/ранньої) стадії проєкту.

## 1. Поточна стадія

DELMOS перебуває на ранній стадії розвитку (фаза документаційно-архітектурного фундаменту, [ROADMAP.md, фази P0–P1](docs/architecture/ROADMAP.md)). На цій стадії проєкт використовує спрощену модель управління:

* **Модель:** Maintainer-led (аналог BDFL на ранній стадії) — засновник(и) проєкту приймають остаточне рішення щодо архітектурних змін, які не мають консенсусу.
* **Механізм рішень:** будь-яка нетривіальна архітектурна зміна оформлюється як [ADR](docs/architecture/decisions/README.md) з розділом «Розглянуті альтернативи» — рішення приймається публічно та обґрунтовано, а не кулуарно.

## 2. Шлях до розширеної моделі управління

Відповідно до [MEMO-004: стратегія валідації цілісності продуктового задуму](docs/memos/MEMO-004-product-concept-validation-strategy.md), перехід до моделі з ширшим колом контриб'юторів, що мають право `merge` (maintainer team), відбувається після досягнення стійкого потоку зовнішніх внесків (не раніше появи перших зовнішніх Merge Request з підтвердженим проходженням `make validate`).

## 3. Прийняття рішень щодо вимог та архітектури

* Стейкхолдерські (`SHR`) та системні (`SWR`) вимоги — пропонуються будь-ким через Merge Request за шаблоном [SHR-TEMPLATE.md](docs/templates/SHR-TEMPLATE.md) / [SWR-TEMPLATE.md](docs/templates/SWR-TEMPLATE.md), затверджуються maintainer'ом.
* Архітектурні рішення (`ADR`) — оформлюються за [ADR-TEMPLATE.md](docs/templates/ADR-TEMPLATE.md); статус `Accepted` присвоює maintainer після публічного обговорення в Merge Request.

## 4. Контакт

Офіційний репозиторій проєкту: <https://github.com/nick-fedchik/delmos>.

* Питання управління проєктом та кодексу поведінки — [GitHub Issues](https://github.com/nick-fedchik/delmos/issues) або [Discussions](https://github.com/nick-fedchik/delmos/discussions).
* Повідомлення про вразливості безпеки — [GitHub Security Advisories](https://github.com/nick-fedchik/delmos/security/advisories/new) (див. [SECURITY.md](SECURITY.md)).

## 5. Пов'язані документи

* [CONTRIBUTING.md](CONTRIBUTING.md) — процес контриб'юції.
* [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) — кодекс поведінки спільноти.
* [SECURITY.md](SECURITY.md) — політика повідомлення про вразливості.
* [docs/memos/MEMO-004-product-concept-validation-strategy.md](docs/memos/MEMO-004-product-concept-validation-strategy.md) — стратегія розвитку від docs-first до перших зовнішніх контриб'юторів.
