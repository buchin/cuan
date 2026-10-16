-- Cuan Migration 000005: Down (000005_runners_leases.down.sql)

DROP INDEX IF EXISTS idx_attempts_lease_reap;
DROP INDEX IF EXISTS idx_attempts_queue_claim;

ALTER TABLE task_attempts
    DROP COLUMN IF EXISTS attempt_number,
    DROP COLUMN IF EXISTS interrupted_at,
    DROP COLUMN IF EXISTS canceled_at,
    DROP COLUMN IF EXISTS cancel_requested,
    DROP COLUMN IF EXISTS lease_expires_at;

DROP TABLE IF EXISTS runners CASCADE;
