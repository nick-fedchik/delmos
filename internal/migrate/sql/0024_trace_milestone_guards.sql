INSERT INTO core.trigger_definitions (trigger_key, phase, input_kind, execution_mode, owner_module) VALUES
    ('trigger.core.before_trace_create', 'before', 'command', 'in_transaction', 'core'),
    ('trigger.core.before_milestone_accept', 'before', 'command', 'in_transaction', 'core');