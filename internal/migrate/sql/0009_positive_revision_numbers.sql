-- 0009: Базова версія будь-якого Work Product, зокрема єдиного Generic
-- Project Plan, є натуральним номером незмінної ревізії.

ALTER TABLE core.work_product_revisions
    ADD CONSTRAINT work_product_revisions_positive_revision_number
    CHECK (revision_number > 0);