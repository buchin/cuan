-- Cuan Migration 000014: Agent Hub & In-Browser Interactive Terminal
-- (000014_agent_hub_terminal.up.sql)

CREATE TABLE IF NOT EXISTS agents (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    avatar VARCHAR(512) NOT NULL DEFAULT '',
    role VARCHAR(128) NOT NULL DEFAULT '',
    system_prompt TEXT NOT NULL DEFAULT '',
    model_id VARCHAR(128) NOT NULL DEFAULT 'gemini-1.5-pro',
    provider VARCHAR(64) NOT NULL DEFAULT 'google',
    status VARCHAR(64) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);
CREATE INDEX IF NOT EXISTS idx_agents_provider ON agents(provider);

CREATE TABLE IF NOT EXISTS agent_connections (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    provider VARCHAR(64) NOT NULL DEFAULT 'google',
    credentials_encrypted TEXT NOT NULL DEFAULT '',
    plan_hint VARCHAR(128) NOT NULL DEFAULT '',
    status VARCHAR(64) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_agent_connections_user_id ON agent_connections(user_id);
CREATE INDEX IF NOT EXISTS idx_agent_connections_provider ON agent_connections(provider);
