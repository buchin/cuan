-- Cuan Migration 000017: Script Deployer & Database Provisioner (Rollback)
-- (000017_script_deployer_and_provisioner.down.sql)

DROP INDEX IF EXISTS idx_provisioned_databases_status;
DROP INDEX IF EXISTS idx_provisioned_databases_engine;
DROP INDEX IF EXISTS idx_provisioned_databases_server_id;
DROP INDEX IF EXISTS idx_provisioned_databases_project_id;
DROP TABLE IF EXISTS provisioned_databases CASCADE;

DROP INDEX IF EXISTS idx_script_deployments_domain;
DROP INDEX IF EXISTS idx_script_deployments_health;
DROP INDEX IF EXISTS idx_script_deployments_build_status;
DROP INDEX IF EXISTS idx_script_deployments_server_id;
DROP INDEX IF EXISTS idx_script_deployments_project_id;
DROP TABLE IF EXISTS script_deployments CASCADE;
