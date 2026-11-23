-- Migration 000045: Task Attempts and Launches Composite Indexes for Fast Aggregation
-- Accelerates taskCountsSQL, latest_attempts, and latest_launches subqueries.

CREATE INDEX IF NOT EXISTS idx_task_attempts_task_created_id
    ON task_attempts(task_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_task_launches_task_created_id
    ON task_launches(task_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_task_launches_task_attempt
    ON task_launches(task_id, attempt_id);
