-- Cuan Migration 000011 Rollback: Observability, Health Checks, Server Metrics, Alert Channels & Incidents
-- (000011_observability_alerts.down.sql)

DROP INDEX IF EXISTS idx_alert_incidents_triggered_at;
DROP INDEX IF EXISTS idx_alert_incidents_target;
DROP INDEX IF EXISTS idx_alert_incidents_status;
DROP INDEX IF EXISTS idx_alert_incidents_project_id;
DROP TABLE IF EXISTS alert_incidents CASCADE;

DROP INDEX IF EXISTS idx_alert_channels_is_active;
DROP INDEX IF EXISTS idx_alert_channels_type;
DROP INDEX IF EXISTS idx_alert_channels_project_id;
DROP TABLE IF EXISTS alert_channels CASCADE;

DROP INDEX IF EXISTS idx_server_metrics_recorded_at;
DROP INDEX IF EXISTS idx_server_metrics_deployment_id;
DROP INDEX IF EXISTS idx_server_metrics_server_id;
DROP TABLE IF EXISTS server_metrics CASCADE;

DROP INDEX IF EXISTS idx_health_checks_status;
DROP INDEX IF EXISTS idx_health_checks_deployment_id;
DROP TABLE IF EXISTS health_checks CASCADE;
