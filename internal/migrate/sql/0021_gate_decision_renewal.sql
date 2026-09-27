ALTER TABLE core.gate_decisions DROP CONSTRAINT gate_decisions_milestone_id_config_generation_key;
CREATE INDEX gate_decisions_latest_idx ON core.gate_decisions
    (milestone_id, config_generation, decided_at DESC, id DESC);