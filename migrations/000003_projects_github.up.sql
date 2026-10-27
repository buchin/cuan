-- Cuan Migration 000003: Projects GitHub Integration (000003_projects_github.up.sql)
-- Adds fields for encrypted access token, GitHub repository metadata, and sync status.

ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS encrypted_token TEXT,
    ADD COLUMN IF NOT EXISTS token_preview VARCHAR(64),
    ADD COLUMN IF NOT EXISTS repo_owner VARCHAR(255),
    ADD COLUMN IF NOT EXISTS repo_name VARCHAR(255),
    ADD COLUMN IF NOT EXISTS clone_status VARCHAR(32) DEFAULT 'pending',
    ADD COLUMN IF NOT EXISTS last_fetched_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS last_error TEXT;
