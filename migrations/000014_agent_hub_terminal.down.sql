-- Cuan Migration 000014: Agent Hub & In-Browser Interactive Terminal (Rollback)
-- (000014_agent_hub_terminal.down.sql)

DROP TABLE IF EXISTS agent_connections CASCADE;
DROP TABLE IF EXISTS agents CASCADE;
