-- Cuan Migration 000043: CRM Broadcast history and session logs
CREATE TABLE IF NOT EXISTS crm_broadcast_sessions (
    id UUID PRIMARY KEY,
    idempotency_key TEXT UNIQUE NOT NULL,
    subject TEXT NOT NULL,
    content TEXT NOT NULL,
    tags JSONB NOT NULL DEFAULT '[]',
    custom_emails JSONB NOT NULL DEFAULT '[]',
    total_recipients INT NOT NULL DEFAULT 0,
    sent_count INT NOT NULL DEFAULT 0,
    failed_count INT NOT NULL DEFAULT 0,
    status TEXT NOT NULL,
    message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS crm_broadcast_sessions_created ON crm_broadcast_sessions(created_at DESC);

CREATE TABLE IF NOT EXISTS crm_broadcast_logs (
    id UUID PRIMARY KEY,
    broadcast_id UUID NOT NULL REFERENCES crm_broadcast_sessions(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    error_message TEXT NOT NULL DEFAULT '',
    sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS crm_broadcast_logs_broadcast_id ON crm_broadcast_logs(broadcast_id);
