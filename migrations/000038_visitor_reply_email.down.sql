DROP TRIGGER IF EXISTS queue_visitor_reply_email ON project_messages;
DROP FUNCTION IF EXISTS queue_visitor_reply_email();
DROP TABLE IF EXISTS chat_visitor_email_outbox;
