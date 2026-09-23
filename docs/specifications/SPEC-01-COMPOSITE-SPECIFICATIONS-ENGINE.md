# SPEC-01: Технічна специфікація рушія композитних специфікацій

Дата: 2026-09-23. Статус: нормативна технічна специфікація підсистеми.  
Ліцензія: Apache 2.0.  
Контекст: [Work Products та специфікації](../architecture/WORK_PRODUCTS.md), [ADR-003](../architecture/decisions/ADR-003-composite-specification-manifests.md),
[SWR-33..35](../requirements/software/SWR-07-composite-specifications.md).

---

## 1. Схема даних маніфесту композиції (CompositionManifest)

Специфікація є Work Product (`type: requirement`, `profile: core:specification`). У межах кожної ревізії зберігається незмінний маніфест структури та входжень:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "CompositionManifest",
  "type": "object",
  "required": ["manifest_version", "sections", "occurrences", "elements_hash"],
  "properties": {
    "manifest_version": { "type": "string", "enum": ["1.0.0"] },
    "sections": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["section_id", "title", "order"],
        "properties": {
          "section_id": { "type": "string" },
          "parent_section_id": { "type": ["string", "null"] },
          "title": { "type": "string" },
          "narrative": { "type": "string" },
          "order": { "type": "integer", "minimum": 1 }
        }
      }
    },
    "occurrences": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["occurrence_id", "section_id", "target_wp_id", "target_revision_id", "target_payload_hash", "order"],
        "properties": {
          "occurrence_id": { "type": "string", "format": "uuid" },
          "section_id": { "type": "string" },
          "target_wp_id": { "type": "string", "format": "uuid" },
          "target_revision_id": { "type": "string", "format": "uuid" },
          "target_payload_hash": { "type": "string", "pattern": "^[a-f0-9]{64}$" },
          "order": { "type": "integer", "minimum": 1 }
        }
      }
    },
    "elements_hash": { "type": "string", "pattern": "^[a-f0-9]{64}$" }
  }
}
```

---

## 2. Алгоритм розрахунку детермінованого Payload Hash

Хеш корисного навантаження ревізії специфікації розраховується детерміновано:

$$\text{payload\_hash} = \text{SHA-256}\Big(\text{CanonicalJSON}\big(\text{metadata without dynamic sync} + \text{body} + \text{CompositionManifest}\big)\Big)$$

1. **Канонізація JSON (RFC 8785 / JCS):** ключі сортуються лексикографічно, пробіли між токенами нормалізуються, кодування UTF-8.
2. **Включення ревізій елементів:** поле `elements_hash` маніфесту обчислюється як SHA-256 від конкатенації відсортованого списку пар `target_wp_id:target_revision_id:target_payload_hash`.
3. Будь-яка зміна ревізії хоча б одного включеного елемента автоматично змінює `elements_hash` і, відповідно, `payload_hash` нової ревізії специфікації.

---

## 3. Таблиці сховища в PostgreSQL

```sql
CREATE TABLE specifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    work_product_id UUID NOT NULL REFERENCES work_products(id) ON DELETE CASCADE,
    revision_id UUID NOT NULL REFERENCES work_product_revisions(id) ON DELETE CASCADE,
    manifest JSONB NOT NULL,
    elements_hash VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT uq_spec_revision UNIQUE (revision_id)
);

CREATE TABLE specification_occurrences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    specification_id UUID NOT NULL REFERENCES specifications(id) ON DELETE CASCADE,
    occurrence_id UUID NOT NULL,
    section_id VARCHAR(64) NOT NULL,
    target_wp_id UUID NOT NULL REFERENCES work_products(id) ON DELETE RESTRICT,
    target_revision_id UUID NOT NULL REFERENCES work_product_revisions(id) ON DELETE RESTRICT,
    item_order INT NOT NULL,
    CONSTRAINT uq_occurrence_per_spec UNIQUE (specification_id, occurrence_id)
);

CREATE INDEX idx_spec_target ON specification_occurrences(target_wp_id, target_revision_id);
```

---

## 4. Верифікація сценаріїв приймання (SPEC-01..10)

Усі 10 сценаріїв верифікуються автоматизованими тестами:
* **SPEC-01:** коректне парсування дерева розділів та створення маніфесту.
* **SPEC-02:** перевірка, що посилання на одну вимогу в кількох специфікаціях мають однакові `target_wp_id` та різні `occurrence_id`.
* **SPEC-03:** створення нової ревізії вимоги не змінює `elements_hash` раніше зафіксованої ревізії специфікації.
* **SPEC-05:** блокування спроби схвалення специфікації, якщо автор специфікації є єдиним затверджувачем (SoD Veto).
* **SPEC-08:** оптимістичне блокування при паралельній зміні маніфесту двома редакторами.
* **SPEC-09:** захист від циклічного вкладення специфікацій.
