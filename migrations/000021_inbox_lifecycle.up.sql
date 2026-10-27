-- Additive lifecycle; existing accepted running requests retain their semantics.
ALTER TABLE inbox_requests DROP CONSTRAINT inbox_requests_state_check;
ALTER TABLE inbox_requests ADD CONSTRAINT inbox_requests_state_check
 CHECK (state IN ('queued','running','completed','failed','interrupted','cancelled','cancelling'));
ALTER TABLE inbox_requests ADD COLUMN started_at TIMESTAMPTZ;
ALTER TABLE inbox_requests ADD COLUMN steps JSONB NOT NULL DEFAULT '[]'::jsonb;
CREATE INDEX inbox_requests_queue ON inbox_requests(room_id,created_at,request_id)
 WHERE state IN ('queued','running','cancelling');

ALTER TABLE inbox_requests ADD COLUMN owner_id TEXT NOT NULL DEFAULT '';
ALTER TABLE inbox_requests ADD COLUMN agent_id TEXT NOT NULL DEFAULT '';

-- Preserve navigable source linkage on the canonical task board.
ALTER TABLE tasks ADD COLUMN source_room_id UUID REFERENCES project_rooms(id) ON DELETE SET NULL;
ALTER TABLE tasks ADD COLUMN source_message_id UUID REFERENCES project_messages(id) ON DELETE SET NULL;
CREATE INDEX tasks_source_room ON tasks(source_room_id) WHERE source_room_id IS NOT NULL;
CREATE UNIQUE INDEX tasks_source_message ON tasks(source_message_id) WHERE source_message_id IS NOT NULL;
