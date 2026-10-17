-- Cuan Migration 000015: Creator Blueprints & In-Task Agent Execution
-- (000015_creator_blueprints_execution.up.sql)

ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS project_type VARCHAR(64) NOT NULL DEFAULT 'custom',
    ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX IF NOT EXISTS idx_projects_project_type ON projects(project_type);

ALTER TABLE project_tasks
    ADD COLUMN IF NOT EXISTS agent_output TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS execution_status VARCHAR(64) NOT NULL DEFAULT 'idle';

CREATE INDEX IF NOT EXISTS idx_project_tasks_execution_status ON project_tasks(execution_status);
