-- Cuan Migration 000028 Down: Task Execution Lanes (000028_task_execution_lanes.down.sql)

DROP INDEX IF EXISTS idx_tasks_execution_category;

ALTER TABLE tasks
    DROP COLUMN IF EXISTS execution_category;
