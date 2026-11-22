-- Migration 000044: Chat and Task Query Performance Indexes
-- Composite indexes for frequent inbox/chat message ordering and task aggregation.

CREATE INDEX IF NOT EXISTS idx_project_messages_room_created_at
    ON project_messages(room_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_tasks_source_room_status
    ON tasks(source_room_id, status)
    WHERE source_room_id IS NOT NULL;
