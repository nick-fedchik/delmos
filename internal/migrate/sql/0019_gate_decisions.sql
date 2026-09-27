-- Незмінний доказ приймання віхи; повтор на ту саму генерацію не створює другого рішення.
ALTER TABLE core.project_milestones ADD CONSTRAINT project_milestones_id_project_unique UNIQUE (id, project_id);

CREATE TABLE core.gate_decisions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL,
    milestone_id uuid NOT NULL,
    plan_revision_id uuid NOT NULL REFERENCES core.work_product_revisions(id) ON DELETE RESTRICT,
    config_generation bigint NOT NULL,
    outcome text NOT NULL CHECK (outcome IN ('passed', 'failed')),
    decided_by uuid NOT NULL REFERENCES core.users(id) ON DELETE RESTRICT,
    decided_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (milestone_id, project_id) REFERENCES core.project_milestones(id, project_id) ON DELETE RESTRICT,
    UNIQUE (milestone_id, config_generation)
);

CREATE TABLE core.gate_decision_evidence (
    gate_decision_id uuid NOT NULL REFERENCES core.gate_decisions(id) ON DELETE RESTRICT,
    work_product_id uuid NOT NULL REFERENCES core.work_products(id) ON DELETE RESTRICT,
    revision_id uuid NOT NULL REFERENCES core.work_product_revisions(id) ON DELETE RESTRICT,
    payload_hash bytea NOT NULL,
    PRIMARY KEY (gate_decision_id, work_product_id)
);

CREATE FUNCTION core.reject_gate_decision_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'gate decision evidence is immutable';
END;
$$;
CREATE TRIGGER gate_decisions_immutable BEFORE UPDATE OR DELETE ON core.gate_decisions
    FOR EACH ROW EXECUTE FUNCTION core.reject_gate_decision_change();
CREATE TRIGGER gate_decision_evidence_immutable BEFORE UPDATE OR DELETE ON core.gate_decision_evidence
    FOR EACH ROW EXECUTE FUNCTION core.reject_gate_decision_change();

UPDATE core.role_definitions SET permission_keys = permission_keys || ARRAY['milestone.accept']
WHERE key = 'project.approver';

INSERT INTO core.event_definitions (event_key, name, owner_module) VALUES
    ('milestone.accepted', 'Віху проєкту прийнято', 'core');