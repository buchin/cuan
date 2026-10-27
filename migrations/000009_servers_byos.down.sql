-- Cuan Migration 000009 Rollback: BYOS Servers & Remote Deployments
-- (000009_servers_byos.down.sql)

DROP INDEX IF EXISTS idx_deployments_server_id;
ALTER TABLE deployments DROP COLUMN IF EXISTS server_id;
DROP TABLE IF EXISTS servers CASCADE;
