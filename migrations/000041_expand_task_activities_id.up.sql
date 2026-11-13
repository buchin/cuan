-- Cuan Migration 000041: Expand task_activities id and attempt_id columns (000041_expand_task_activities_id.up.sql)
-- Prevents SQLSTATE 22001 (value too long for type character varying(64)) when activity IDs contain attempt identifiers.

ALTER TABLE task_activities
    ALTER COLUMN id TYPE VARCHAR(128),
    ALTER COLUMN attempt_id TYPE VARCHAR(128);
