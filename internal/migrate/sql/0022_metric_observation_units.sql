ALTER TABLE core.metric_observations ADD COLUMN unit text;

ALTER TABLE core.metric_observations DISABLE TRIGGER metric_observations_no_update;
UPDATE core.metric_observations o SET unit = CASE
    WHEN d.unit LIKE 'currency:%' THEN 'currency:' || COALESCE(
        (SELECT cb.currency FROM core.cost_baselines cb WHERE cb.id = o.baseline_id),
        (SELECT cb.currency FROM core.cost_baselines cb
         WHERE cb.project_id = o.project_id AND cb.status = 'approved'),
        substring(d.unit FROM 10))
    ELSE d.unit END
FROM core.metric_definitions d WHERE d.metric_key = o.metric_key;
ALTER TABLE core.metric_observations ENABLE TRIGGER metric_observations_no_update;
ALTER TABLE core.metric_observations ALTER COLUMN unit SET NOT NULL;
ALTER TABLE core.metric_observations ADD CONSTRAINT metric_observations_unit_check
    CHECK (unit IN ('count', 'percent', 'ratio', 'ms') OR unit ~ '^currency:[A-Z]{3}$');

ALTER TABLE core.metric_definitions DROP CONSTRAINT metric_definitions_unit_check;
ALTER TABLE core.metric_definitions ADD CONSTRAINT metric_definitions_unit_check
    CHECK (unit IN ('count', 'percent', 'ratio', 'ms', 'currency') OR unit ~ '^currency:[A-Z]{3}$');
UPDATE core.metric_definitions SET unit = 'currency' WHERE unit LIKE 'currency:%';