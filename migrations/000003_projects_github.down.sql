-- Cuan Migration 000003: Projects GitHub Integration (000003_projects_github.down.sql)
-- Rollback GitHub integration fields from projects table.

ALTER TABLE projects
    DROP COLUMN IF EXISTS encrypted_token,
    DROP COLUMN IF EXISTS token_preview,
    DROP COLUMN IF EXISTS repo_owner,
    DROP COLUMN IF EXISTS repo_name,
    DROP COLUMN IF EXISTS clone_status,
    DROP COLUMN IF EXISTS last_fetched_at,
    DROP COLUMN IF EXISTS last_error;
