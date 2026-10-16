-- Cuan Migration 000005: Runners & Leases (000005_runners_leases.up.sql)
-- Adds runners table, lease management columns on task_attempts, and queue indexes.

CREATE TABLE IF NOT EXISTS runners (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    hostname VARCHAR(128) NOT NULL DEFAULT '',
    supported_agents TEXT NOT NULL DEFAULT 'agy',
    status VARCHAR(32) NOT NULL DEFAULT 'offline', -- idle, busy, offline
    last_heartbeat_at TIMESTAMP WITH TIME ZONE,
    current_task_id VARCHAR(64) REFERENCES tasks(id) ON DELETE SET NULL,
    current_attempt_id VARCHAR(64) REFERENCES task_attempts(id) ON DELETE SET NULL,
    cpu_percent REAL DEFAULT 0,
    memory_used_bytes BIGINT DEFAULT 0,
    memory_total_bytes BIGINT DEFAULT 0,
    disk_used_bytes BIGINT DEFAULT 0,
    disk_total_bytes BIGINT DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_runners_status ON runners(status);
CREATE INDEX IF NOT EXISTS idx_runners_last_heartbeat ON runners(last_heartbeat_at);

ALTER TABLE task_attempts
    ADD COLUMN IF NOT EXISTS lease_expires_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS cancel_requested BOOLEAN DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS canceled_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS interrupted_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS attempt_number INT DEFAULT 1;

CREATE INDEX IF NOT EXISTS idx_attempts_queue_claim ON task_attempts(status, created_at) WHERE status = 'queued';
CREATE INDEX IF NOT EXISTS idx_attempts_lease_reap ON task_attempts(status, lease_expires_at) WHERE status = 'running';
