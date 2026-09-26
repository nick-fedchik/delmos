-- 0012: семантичні ембедінги ревізій вимог і тест-кейсів (VEC-01..03).
-- Розширення vector активується заздалегідь суперкористувачем ОС (не тут —
-- воно не є trusted, LOCAL_SETUP.md §3, scripts/sql/bootstrap.sql,
-- scripts/sql/test-template.sql). Ця міграція лише створює таблицю й індекс.

CREATE TABLE core.wp_embeddings (
    id                  uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    work_product_id     uuid         NOT NULL REFERENCES core.work_products (id) ON DELETE CASCADE,
    revision_id         uuid         NOT NULL REFERENCES core.work_product_revisions (id) ON DELETE CASCADE,
    model_name          text         NOT NULL,
    model_version       text         NOT NULL,
    dimensions          int          NOT NULL,
    embedding           vector(1536) NOT NULL,
    created_at          timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT uq_wp_embedding_rev_model UNIQUE (revision_id, model_name, model_version)
);

CREATE INDEX wp_embeddings_wp_idx ON core.wp_embeddings (work_product_id);

-- HNSW-індекс косинусної відстані для гібридного семантичного пошуку (VEC-02).
CREATE INDEX wp_embeddings_hnsw_idx ON core.wp_embeddings
    USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);
