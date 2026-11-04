CREATE TABLE IF NOT EXISTS task_launch_recoveries (
    launch_id TEXT NOT NULL REFERENCES task_launches(id) ON DELETE CASCADE,
    recovery_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (launch_id, recovery_key)
);
