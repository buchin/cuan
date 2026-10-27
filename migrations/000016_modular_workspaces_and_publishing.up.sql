-- Cuan Migration 000016: Modular Workspace OS, Integrations & Creator Publishing Hub
-- (000016_modular_workspaces_and_publishing.up.sql)

ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS enabled_modules JSONB NOT NULL DEFAULT '["tasks", "discussions"]'::jsonb,
    ADD COLUMN IF NOT EXISTS source_type VARCHAR(64) NOT NULL DEFAULT 'blank',
    ADD COLUMN IF NOT EXISTS source_metadata JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX IF NOT EXISTS idx_projects_source_type ON projects(source_type);

CREATE TABLE IF NOT EXISTS project_integrations (
    id VARCHAR(64) PRIMARY KEY,
    project_id VARCHAR(64) NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    integration_type VARCHAR(64) NOT NULL,
    config_encrypted TEXT NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_project_integrations_project_id ON project_integrations(project_id);
CREATE INDEX IF NOT EXISTS idx_project_integrations_type ON project_integrations(integration_type);
CREATE INDEX IF NOT EXISTS idx_project_integrations_status ON project_integrations(status);

ALTER TABLE project_tasks
    ADD COLUMN IF NOT EXISTS scheduled_at TIMESTAMP WITH TIME ZONE NULL,
    ADD COLUMN IF NOT EXISTS published_at TIMESTAMP WITH TIME ZONE NULL,
    ADD COLUMN IF NOT EXISTS publication_url VARCHAR(512) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_project_tasks_scheduled_at ON project_tasks(scheduled_at);
CREATE INDEX IF NOT EXISTS idx_project_tasks_published_at ON project_tasks(published_at);
