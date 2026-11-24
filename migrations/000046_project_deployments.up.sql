-- Cuan Migration 000046: Project Deployments & Timeline Steps History
-- Provides Laravel Cloud style deployment tracking per project.

CREATE TABLE IF NOT EXISTS project_deployments (
    id VARCHAR(64) PRIMARY KEY,
    project_id VARCHAR(64) NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    commit_sha VARCHAR(64) NOT NULL,
    short_sha VARCHAR(16) NOT NULL,
    commit_message TEXT NOT NULL DEFAULT '',
    branch VARCHAR(128) NOT NULL DEFAULT 'main',
    status VARCHAR(32) NOT NULL DEFAULT 'building',
    trigger_source VARCHAR(64) NOT NULL DEFAULT 'manual',
    trigger_author VARCHAR(128) NOT NULL DEFAULT '',
    duration_seconds INTEGER NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    steps_json JSONB NOT NULL DEFAULT '[]',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_project_deployments_project_created
    ON project_deployments(project_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_project_deployments_commit_sha
    ON project_deployments(commit_sha);

CREATE INDEX IF NOT EXISTS idx_project_deployments_status
    ON project_deployments(status);
