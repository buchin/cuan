-- Cuan Migration 000016: Modular Workspace OS, Integrations & Creator Publishing Hub (Rollback)
-- (000016_modular_workspaces_and_publishing.down.sql)

DROP INDEX IF EXISTS idx_project_tasks_published_at;
DROP INDEX IF EXISTS idx_project_tasks_scheduled_at;

ALTER TABLE project_tasks
    DROP COLUMN IF EXISTS publication_url,
    DROP COLUMN IF EXISTS published_at,
    DROP COLUMN IF EXISTS scheduled_at;

DROP INDEX IF EXISTS idx_project_integrations_status;
DROP INDEX IF EXISTS idx_project_integrations_type;
DROP INDEX IF EXISTS idx_project_integrations_project_id;

DROP TABLE IF EXISTS project_integrations CASCADE;

DROP INDEX IF EXISTS idx_projects_source_type;

ALTER TABLE projects
    DROP COLUMN IF EXISTS source_metadata,
    DROP COLUMN IF EXISTS source_type,
    DROP COLUMN IF EXISTS enabled_modules;
