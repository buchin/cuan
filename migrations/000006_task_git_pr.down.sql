-- Cuan Migration 000006 Down: Task Git & PR Integration (000006_task_git_pr.down.sql)

DROP INDEX IF EXISTS idx_tasks_branch_name;

ALTER TABLE tasks
    DROP COLUMN IF EXISTS branch_name,
    DROP COLUMN IF EXISTS pr_url,
    DROP COLUMN IF EXISTS pr_number,
    DROP COLUMN IF EXISTS commit_sha,
    DROP COLUMN IF EXISTS diff_summary,
    DROP COLUMN IF EXISTS test_summary;
