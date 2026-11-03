ALTER TABLE task_attempts ADD COLUMN IF NOT EXISTS execution_deadline TIMESTAMPTZ;
