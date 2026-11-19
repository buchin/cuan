-- Cuan Migration 000043 rollback: Drop CRM Broadcast history tables
DROP TABLE IF EXISTS crm_broadcast_logs;
DROP TABLE IF EXISTS crm_broadcast_sessions;
