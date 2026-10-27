-- Cuan Migration 000019: Auth Layer, Session Management & Multi-Project Schema (Rollback)
-- (000019_auth_and_multiproject.down.sql)

DROP INDEX IF EXISTS idx_task_projects_task_id;
DROP INDEX IF EXISTS idx_task_projects_project_id;
DROP TABLE IF EXISTS task_projects CASCADE;

DROP INDEX IF EXISTS idx_projects_is_archived;
ALTER TABLE projects DROP COLUMN IF EXISTS is_archived;

DROP INDEX IF EXISTS idx_owners_username;
ALTER TABLE owners DROP COLUMN IF EXISTS username;
