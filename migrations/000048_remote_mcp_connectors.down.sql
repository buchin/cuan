-- Cuan Migration 000048 Down: Drop Remote MCP Connectors table

DROP INDEX IF EXISTS idx_remote_mcp_connectors_enabled;
DROP TABLE IF EXISTS remote_mcp_connectors;
