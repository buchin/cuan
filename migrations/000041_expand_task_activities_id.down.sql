-- Cuan Migration 000041 Down: Revert task_activities id and attempt_id columns to VARCHAR(64)

ALTER TABLE task_activities
    ALTER COLUMN id TYPE VARCHAR(64),
    ALTER COLUMN attempt_id TYPE VARCHAR(64);
