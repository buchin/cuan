-- Cuan Migration 000028: Task Execution Lanes (000028_task_execution_lanes.up.sql)
-- Adds execution_category column to tasks for multi-lane concurrency (code, ops, readonly)

ALTER TABLE tasks
    ADD COLUMN IF NOT EXISTS execution_category VARCHAR(32) NOT NULL DEFAULT 'code';

CREATE INDEX IF NOT EXISTS idx_tasks_execution_category ON tasks(execution_category);
