-- Cuan Migration 000008 Rollback: Deployments & Deployment Logs
-- (000008_deployer.down.sql)

DROP TABLE IF EXISTS deployment_logs CASCADE;
DROP TABLE IF EXISTS deployments CASCADE;
