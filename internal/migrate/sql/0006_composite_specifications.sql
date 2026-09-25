-- 0006: незмінні маніфести композитних специфікацій (SPEC-01..10).

CREATE TABLE core.specifications (
    id                  uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    work_product_id     uuid        NOT NULL REFERENCES core.work_products (id) ON DELETE CASCADE,
    revision_id         uuid        NOT NULL REFERENCES core.work_product_revisions (id) ON DELETE CASCADE,
    manifest            jsonb       NOT NULL,
    elements_hash       bytea       NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_specification_revision UNIQUE (revision_id)
);

CREATE TABLE core.specification_occurrences (
    id                    uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    specification_id      uuid        NOT NULL REFERENCES core.specifications (id) ON DELETE CASCADE,
    occurrence_id         uuid        NOT NULL,
    section_id            text        NOT NULL,
    target_work_product_id uuid       NOT NULL REFERENCES core.work_products (id) ON DELETE RESTRICT,
    target_revision_id    uuid        NOT NULL REFERENCES core.work_product_revisions (id) ON DELETE RESTRICT,
    item_order            integer     NOT NULL CHECK (item_order >= 1),
    CONSTRAINT uq_occurrence_per_spec UNIQUE (specification_id, occurrence_id)
);

CREATE INDEX specification_occurrences_target_idx
    ON core.specification_occurrences (target_work_product_id, target_revision_id);
