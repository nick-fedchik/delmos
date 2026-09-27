INSERT INTO core.event_definitions (event_key, name, owner_module) VALUES
    ('wp.retired', 'Артефакт виведено з експлуатації', 'core');

INSERT INTO core.trigger_subscriptions (trigger_key, event_key, subscriber_module) VALUES
    ('trigger.core.after_economics_changed', 'wp.retired', 'core');

CREATE OR REPLACE FUNCTION core.advance_metric_input_version() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.project_id IS NOT NULL AND NEW.event_key IN
        ('economics.inputs_changed', 'phase.transitioned', 'wp.approved',
         'wp.revision_committed', 'plan.applied', 'wp.retired') THEN
        INSERT INTO core.metric_input_versions(project_id, version) VALUES (NEW.project_id, 1)
        ON CONFLICT (project_id) DO UPDATE SET version = core.metric_input_versions.version + 1;
    END IF;
    RETURN NEW;
END;
$$;