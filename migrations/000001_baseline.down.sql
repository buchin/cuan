-- Cuan Baseline Migration Down (000001_baseline.down.sql)
-- Drops core schema tables in reverse order.

DROP TABLE IF EXISTS agent_logs CASCADE;
DROP TABLE IF EXISTS task_attempts CASCADE;
DROP TABLE IF EXISTS tasks CASCADE;
DROP TABLE IF EXISTS projects CASCADE;
DROP TABLE IF EXISTS owners CASCADE;
DROP TABLE IF EXISTS cuan_schema_migrations CASCADE;
