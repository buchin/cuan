-- Cuan Migration 000002: Owner Sessions (000002_owner_sessions.up.sql)
-- Manages secure session tokens for single-owner authentication.

CREATE TABLE IF NOT EXISTS owner_sessions (
    token_hash VARCHAR(64) PRIMARY KEY,
    owner_id VARCHAR(64) NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_owner_sessions_owner_id ON owner_sessions(owner_id);
CREATE INDEX IF NOT EXISTS idx_owner_sessions_expires_at ON owner_sessions(expires_at);
