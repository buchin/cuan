-- Cuan Migration 000004 Down: Tasks & Activities Extension (000004_tasks_activities.down.sql)

DROP TABLE IF EXISTS task_activities;

ALTER TABLE tasks
    DROP COLUMN IF EXISTS assigned_agent;
