-- Cuan Migration 000006: Task Git & PR Integration (000006_task_git_pr.up.sql)
-- Adds branch_name, pr_url, pr_number, commit_sha, diff_summary, and test_summary to tasks.

ALTER TABLE tasks
    ADD COLUMN IF NOT EXISTS branch_name VARCHAR(128),
    ADD COLUMN IF NOT EXISTS pr_url TEXT,
    ADD COLUMN IF NOT EXISTS pr_number INT,
    ADD COLUMN IF NOT EXISTS commit_sha VARCHAR(64),
    ADD COLUMN IF NOT EXISTS diff_summary TEXT,
    ADD COLUMN IF NOT EXISTS test_summary TEXT;

CREATE INDEX IF NOT EXISTS idx_tasks_branch_name ON tasks(branch_name);
