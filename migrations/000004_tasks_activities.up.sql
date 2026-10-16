-- Cuan Migration 000004: Tasks & Activities Extension (000004_tasks_activities.up.sql)
-- Adds assigned_agent to tasks and creates task_activities table for audit history & comments.

ALTER TABLE tasks
    ADD COLUMN IF NOT EXISTS assigned_agent VARCHAR(64) DEFAULT 'agy';

CREATE TABLE IF NOT EXISTS task_activities (
    id VARCHAR(64) PRIMARY KEY,
    task_id VARCHAR(64) NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    attempt_id VARCHAR(64) REFERENCES task_attempts(id) ON DELETE SET NULL,
    activity_type VARCHAR(32) NOT NULL, -- task_created, status_change, attempt_transition, comment, task_updated
    author VARCHAR(128) NOT NULL,       -- owner email or agent identifier
    old_status VARCHAR(32),
    new_status VARCHAR(32),
    message TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_activities_task_id ON task_activities(task_id);
CREATE INDEX IF NOT EXISTS idx_activities_created_at ON task_activities(created_at);
