-- Cuan Migration 000008: Deployments, Runtime Profiles & Deployment Logs
-- (000008_deployer.up.sql)

CREATE TABLE IF NOT EXISTS deployments (
    id VARCHAR(64) PRIMARY KEY,
    project_name VARCHAR(255) NOT NULL,
    slug VARCHAR(128) NOT NULL UNIQUE,
    source_type VARCHAR(32) NOT NULL DEFAULT 'zip', -- 'zip', 'git'
    source_ref TEXT,
    runtime VARCHAR(32) NOT NULL DEFAULT 'custom', -- 'php', 'nodejs', 'static', 'custom'
    status VARCHAR(32) NOT NULL DEFAULT 'pending', -- 'pending', 'extracting', 'building', 'running', 'stopped', 'failed'
    internal_port INTEGER DEFAULT 0,
    domain VARCHAR(255) NOT NULL DEFAULT '',
    env_vars JSONB NOT NULL DEFAULT '{}'::jsonb,
    process_pid INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_deployments_slug ON deployments(slug);
CREATE INDEX IF NOT EXISTS idx_deployments_status ON deployments(status);

CREATE TABLE IF NOT EXISTS deployment_logs (
    id BIGSERIAL PRIMARY KEY,
    deployment_id VARCHAR(64) NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
    stream VARCHAR(16) NOT NULL DEFAULT 'stdout', -- 'stdout', 'stderr', 'system'
    message TEXT NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_deployment_logs_deployment_timestamp ON deployment_logs(deployment_id, timestamp);
