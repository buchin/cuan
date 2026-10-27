-- Cuan Migration 000011: Observability, Health Checks, Server Metrics, Alert Channels & Incidents
-- (000011_observability_alerts.up.sql)

CREATE TABLE IF NOT EXISTS health_checks (
    id VARCHAR(64) PRIMARY KEY,
    deployment_id VARCHAR(64) NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'healthy', -- 'healthy', 'degraded', 'unhealthy'
    last_checked_at TIMESTAMP WITH TIME ZONE,
    last_status_code INTEGER DEFAULT 0,
    response_time_ms BIGINT DEFAULT 0,
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_health_checks_deployment_id ON health_checks(deployment_id);
CREATE INDEX IF NOT EXISTS idx_health_checks_status ON health_checks(status);

CREATE TABLE IF NOT EXISTS server_metrics (
    id VARCHAR(64) PRIMARY KEY,
    server_id VARCHAR(64) REFERENCES servers(id) ON DELETE SET NULL,
    deployment_id VARCHAR(64) REFERENCES deployments(id) ON DELETE CASCADE,
    cpu_percent DOUBLE PRECISION NOT NULL DEFAULT 0,
    memory_used_mb BIGINT NOT NULL DEFAULT 0,
    memory_total_mb BIGINT NOT NULL DEFAULT 0,
    disk_used_gb DOUBLE PRECISION NOT NULL DEFAULT 0,
    disk_total_gb DOUBLE PRECISION NOT NULL DEFAULT 0,
    recorded_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_server_metrics_server_id ON server_metrics(server_id);
CREATE INDEX IF NOT EXISTS idx_server_metrics_deployment_id ON server_metrics(deployment_id);
CREATE INDEX IF NOT EXISTS idx_server_metrics_recorded_at ON server_metrics(recorded_at);

CREATE TABLE IF NOT EXISTS alert_channels (
    id VARCHAR(64) PRIMARY KEY,
    project_id VARCHAR(64) NOT NULL,
    type VARCHAR(32) NOT NULL, -- 'telegram', 'discord', 'webhook'
    name VARCHAR(255) NOT NULL,
    config_encrypted TEXT NOT NULL, -- AES-GCM encrypted payload (bot tokens, webhook URLs, secrets)
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alert_channels_project_id ON alert_channels(project_id);
CREATE INDEX IF NOT EXISTS idx_alert_channels_type ON alert_channels(type);
CREATE INDEX IF NOT EXISTS idx_alert_channels_is_active ON alert_channels(is_active);

CREATE TABLE IF NOT EXISTS alert_incidents (
    id VARCHAR(64) PRIMARY KEY,
    project_id VARCHAR(64) NOT NULL,
    target_type VARCHAR(32) NOT NULL, -- 'deployment', 'server'
    target_id VARCHAR(64) NOT NULL,
    title VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'firing', -- 'firing', 'resolved'
    triggered_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_alert_incidents_project_id ON alert_incidents(project_id);
CREATE INDEX IF NOT EXISTS idx_alert_incidents_status ON alert_incidents(status);
CREATE INDEX IF NOT EXISTS idx_alert_incidents_target ON alert_incidents(target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_alert_incidents_triggered_at ON alert_incidents(triggered_at);
