CREATE TABLE IF NOT EXISTS chat_message_receipts (
    message_id UUID PRIMARY KEY REFERENCES project_messages(id) ON DELETE CASCADE,
    delivered_at TIMESTAMPTZ,
    read_confirmed_at TIMESTAMPTZ
);
CREATE TABLE IF NOT EXISTS chat_email_receipts (
 email_id VARCHAR(128) PRIMARY KEY,
 status VARCHAR(32) NOT NULL,
 status_rank INT NOT NULL,
 occurred_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE IF NOT EXISTS chat_message_emails (
 message_id UUID NOT NULL REFERENCES project_messages(id) ON DELETE CASCADE,
 email_id VARCHAR(128) NOT NULL REFERENCES chat_email_receipts(email_id) ON DELETE CASCADE,
 recipient_type VARCHAR(16) NOT NULL CHECK(recipient_type IN ('operator','visitor')),
 linked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY(message_id,email_id)
);
CREATE INDEX IF NOT EXISTS chat_message_emails_email_idx ON chat_message_emails(email_id);
