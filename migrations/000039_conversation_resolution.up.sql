-- Resolution belongs to the shared conversation; read receipts remain per user.
ALTER TABLE project_rooms ADD COLUMN resolved_at TIMESTAMPTZ;
