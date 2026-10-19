CREATE TABLE inbox_requests (
 room_id UUID NOT NULL REFERENCES project_rooms(id) ON DELETE CASCADE,
 request_id UUID NOT NULL,
 payload_hash TEXT NOT NULL,
 message_id UUID NOT NULL,
 reply_id UUID NOT NULL,
 state TEXT NOT NULL CHECK (state IN ('running','completed','failed','interrupted')),
 safe_error TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL,
 deadline TIMESTAMPTZ NOT NULL,
 PRIMARY KEY (room_id, request_id)
);
