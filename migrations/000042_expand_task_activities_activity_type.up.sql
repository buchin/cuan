-- Cuan Migration 000042: Expand task_activities activity_type column (000042_expand_task_activities_activity_type.up.sql)
-- Prevents SQLSTATE 22001 (value too long for type character varying(32)) for longer activity types.

ALTER TABLE task_activities
    ALTER COLUMN activity_type TYPE VARCHAR(64);
