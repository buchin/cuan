-- Cuan Migration 000046 Rollback: Project Deployments & Timeline Steps History

DROP INDEX IF EXISTS idx_project_deployments_status;
DROP INDEX IF EXISTS idx_project_deployments_commit_sha;
DROP INDEX IF EXISTS idx_project_deployments_project_created;
DROP TABLE IF EXISTS project_deployments;
