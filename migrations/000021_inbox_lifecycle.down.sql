-- Explicit maintenance only; application rollback must retain the additive schema.
UPDATE inbox_requests SET state='interrupted',safe_error='Proses dihentikan untuk rollback schema.'
 WHERE state IN ('queued','cancelled','cancelling');
DROP INDEX inbox_requests_queue;
ALTER TABLE inbox_requests DROP COLUMN steps;
ALTER TABLE inbox_requests DROP COLUMN started_at;
ALTER TABLE inbox_requests DROP CONSTRAINT inbox_requests_state_check;
ALTER TABLE inbox_requests ADD CONSTRAINT inbox_requests_state_check
 CHECK (state IN ('running','completed','failed','interrupted'));

ALTER TABLE inbox_requests DROP COLUMN owner_id;
ALTER TABLE inbox_requests DROP COLUMN agent_id;
ALTER TABLE tasks DROP COLUMN source_message_id;
ALTER TABLE tasks DROP COLUMN source_room_id;
