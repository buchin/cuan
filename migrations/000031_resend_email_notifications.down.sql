-- Cuan Migration 000031: Resend Email Logs and Idle Notifications (Rollback)
-- (000031_resend_email_notifications.down.sql)

DROP INDEX IF EXISTS idx_resend_email_logs_sent_at;
DROP INDEX IF EXISTS idx_resend_email_logs_room_id;
DROP TABLE IF EXISTS resend_email_logs;
