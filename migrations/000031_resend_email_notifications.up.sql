-- Cuan Migration 000031: Resend Email Logs and Idle Notifications
-- (000031_resend_email_notifications.up.sql)

CREATE TABLE IF NOT EXISTS resend_email_logs (
    id VARCHAR(64) PRIMARY KEY,
    room_id VARCHAR(64) NOT NULL,
    recipient_email VARCHAR(255) NOT NULL,
    subject VARCHAR(255) NOT NULL,
    resend_email_id VARCHAR(128) NOT NULL DEFAULT '',
    message_count INT NOT NULL DEFAULT 1,
    status VARCHAR(32) NOT NULL DEFAULT 'sent',
    error_message TEXT NOT NULL DEFAULT '',
    sent_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_resend_email_logs_room_id ON resend_email_logs(room_id);
CREATE INDEX IF NOT EXISTS idx_resend_email_logs_sent_at ON resend_email_logs(sent_at);
