-- Cuan Migration 000048: Remote MCP Connectors Persistence
-- Persists custom Remote MCP server configurations across server restarts and deployments.

CREATE TABLE IF NOT EXISTS remote_mcp_connectors (
    id VARCHAR(128) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    base_url TEXT NOT NULL,
    transport VARCHAR(32) NOT NULL DEFAULT 'http',
    api_key_encrypted TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_remote_mcp_connectors_enabled
    ON remote_mcp_connectors(enabled, created_at ASC);
