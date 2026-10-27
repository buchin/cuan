-- Cuan Migration 000009: BYOS Servers & Remote Deployments
-- (000009_servers_byos.up.sql)

CREATE TABLE IF NOT EXISTS servers (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    host VARCHAR(255) NOT NULL,
    port INTEGER NOT NULL DEFAULT 22,
    username VARCHAR(100) NOT NULL DEFAULT 'ubuntu',
    auth_type VARCHAR(50) NOT NULL DEFAULT 'key',
    encrypted_key TEXT NOT NULL,
    key_fingerprint VARCHAR(255),
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    last_ping_at TIMESTAMPTZ,
    os_info JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_servers_status ON servers(status);
CREATE INDEX IF NOT EXISTS idx_servers_host ON servers(host);

ALTER TABLE deployments ADD COLUMN IF NOT EXISTS server_id VARCHAR(64) REFERENCES servers(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_deployments_server_id ON deployments(server_id);
