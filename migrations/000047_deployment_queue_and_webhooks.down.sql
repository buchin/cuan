-- Cuan Migration 000047 Rollback: Project Deployment Queue Index

DROP INDEX IF EXISTS idx_project_deployments_queue;
