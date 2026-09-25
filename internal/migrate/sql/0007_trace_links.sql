-- 0007: типізовані зв'язки простежуваності та project-scoped граф.

CREATE TABLE core.trace_links (
    id                  uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id          uuid        NOT NULL REFERENCES core.projects (id) ON DELETE CASCADE,
    source_id           uuid        NOT NULL REFERENCES core.work_products (id) ON DELETE CASCADE,
    source_revision_id  uuid        NULL REFERENCES core.work_product_revisions (id) ON DELETE RESTRICT,
    target_id           uuid        NOT NULL REFERENCES core.work_products (id) ON DELETE CASCADE,
    target_revision_id  uuid        NULL REFERENCES core.work_product_revisions (id) ON DELETE RESTRICT,
    relation_kind       text        NOT NULL CHECK (relation_kind IN ('verifies', 'satisfies', 'refines', 'derives_from')),
    is_suspect          boolean     NOT NULL DEFAULT false,
    created_at          timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_trace_link UNIQUE (project_id, source_id, target_id, relation_kind)
);

CREATE INDEX trace_links_source_idx ON core.trace_links (project_id, source_id, target_id);
CREATE INDEX trace_links_target_idx ON core.trace_links (project_id, target_id, source_id);
