-- 0008: permissions for project traceability authoring.

UPDATE core.role_definitions
SET permission_keys = ARRAY['project.read', 'wp.read', 'wp.create', 'wp.edit', 'wp.retire', 'repository.manage', 'trace.create', 'trace.unlink']
WHERE key = 'project.owner';
