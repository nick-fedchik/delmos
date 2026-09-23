-- 0004: розширення прав власника проєкту на CRUD базового Work Product
-- (SWR-43 §2: Authoring — wp.create, wp.edit, wp.retire). Ідемпотентно: просто
-- перезаписує перелік дозволів ролі, а не додає нову роль.
UPDATE core.role_definitions
SET permission_keys = ARRAY['project.read', 'wp.read', 'wp.create', 'wp.edit', 'wp.retire']
WHERE key = 'project.owner';
