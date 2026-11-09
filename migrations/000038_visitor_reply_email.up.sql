-- Future inserts only. Never backfill historical customer conversations.
CREATE TABLE chat_visitor_email_outbox (
 message_id UUID PRIMARY KEY REFERENCES project_messages(id) ON DELETE CASCADE,
 contact_id UUID NOT NULL REFERENCES crm_contacts(id) ON DELETE CASCADE,
 recipient TEXT NOT NULL,
 content TEXT NOT NULL,
 payload JSONB,
 connection_hash TEXT NOT NULL DEFAULT '',
 state TEXT NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','sent','skipped','failed')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '5 minutes',
 attempts INT NOT NULL DEFAULT 0,
 last_error TEXT NOT NULL DEFAULT ''
);
CREATE INDEX chat_visitor_email_due ON chat_visitor_email_outbox(next_attempt_at) WHERE state='pending';
CREATE FUNCTION queue_visitor_reply_email() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.author_type='user' AND NEW.author_id='operator_user' THEN
  INSERT INTO chat_visitor_email_outbox(message_id,contact_id,recipient,content)
   SELECT NEW.id,id,email,NEW.content FROM crm_contacts
   WHERE session_uuid=NEW.room_id AND email<>'' ON CONFLICT DO NOTHING;
 END IF;
 RETURN NEW;
END;
$$;
CREATE TRIGGER queue_visitor_reply_email AFTER INSERT ON project_messages
 FOR EACH ROW EXECUTE FUNCTION queue_visitor_reply_email();
