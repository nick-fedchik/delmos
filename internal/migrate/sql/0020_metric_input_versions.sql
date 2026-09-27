CREATE TABLE core.metric_input_versions (
    project_id uuid PRIMARY KEY REFERENCES core.projects(id) ON DELETE CASCADE,
    version bigint NOT NULL DEFAULT 0 CHECK (version >= 0)
);

ALTER TABLE core.metric_observations
    ADD COLUMN source_version bigint NOT NULL DEFAULT 0 CHECK (source_version >= 0),
    ADD COLUMN baseline_id uuid REFERENCES core.cost_baselines(id) ON DELETE RESTRICT,
    ADD COLUMN baseline_version integer,
    ADD COLUMN config_generation bigint;

CREATE FUNCTION core.advance_metric_input_version() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.project_id IS NOT NULL AND NEW.event_key IN
        ('economics.inputs_changed', 'phase.transitioned', 'wp.approved', 'wp.revision_committed', 'plan.applied') THEN
        INSERT INTO core.metric_input_versions(project_id, version) VALUES (NEW.project_id, 1)
        ON CONFLICT (project_id) DO UPDATE SET version = core.metric_input_versions.version + 1;
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER metric_input_version_on_event AFTER INSERT ON core.event_outbox
    FOR EACH ROW EXECUTE FUNCTION core.advance_metric_input_version();

INSERT INTO core.trigger_subscriptions(trigger_key, event_key, subscriber_module) VALUES
    ('trigger.core.after_economics_changed', 'wp.revision_committed', 'core'),
    ('trigger.core.after_economics_changed', 'plan.applied', 'core');