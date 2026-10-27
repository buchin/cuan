-- Cuan Migration 000015: Creator Blueprints & In-Task Agent Execution (Rollback)
-- (000015_creator_blueprints_execution.down.sql)

DROP INDEX IF EXISTS idx_project_tasks_execution_status;

ALTER TABLE project_tasks
    DROP COLUMN IF EXISTS execution_status,
    DROP COLUMN IF EXISTS agent_output;

DROP INDEX IF EXISTS idx_projects_project_type;

ALTER TABLE projects
    DROP COLUMN IF EXISTS metadata,
    DROP COLUMN IF EXISTS project_type;
