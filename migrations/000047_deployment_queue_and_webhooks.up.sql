-- Cuan Migration 000047: Project Deployment Queue Index
-- Supports dedicated deployment lane processing and Git webhook triggers.

CREATE INDEX IF NOT EXISTS idx_project_deployments_queue
    ON project_deployments(project_id, status, created_at ASC);
