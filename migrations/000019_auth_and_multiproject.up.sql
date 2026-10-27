-- Cuan Migration 000019: Auth Layer, Session Management & Multi-Project Schema
-- (000019_auth_and_multiproject.up.sql)

-- 1. Multi-project support: add is_archived to projects
ALTER TABLE projects ADD COLUMN IF NOT EXISTS is_archived BOOLEAN NOT NULL DEFAULT FALSE;
CREATE INDEX IF NOT EXISTS idx_projects_is_archived ON projects(is_archived);

-- 2. Multi-project task linking: many-to-many task_projects relation
CREATE TABLE IF NOT EXISTS task_projects (
    task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (task_id, project_id)
);

CREATE INDEX IF NOT EXISTS idx_task_projects_project_id ON task_projects(project_id);
CREATE INDEX IF NOT EXISTS idx_task_projects_task_id ON task_projects(task_id);

-- 3. Auth layer: ensure owners table has username column and index
ALTER TABLE owners ADD COLUMN IF NOT EXISTS username VARCHAR(100);
CREATE UNIQUE INDEX IF NOT EXISTS idx_owners_username ON owners(username);

-- Fresh installs must create their own owner through authenticated setup.
-- Do not seed shared credentials. Already-applied migrations are skipped, so this
-- does not remove or modify an existing installation's owner.

-- Backfill username for existing owners without username
UPDATE owners SET username = 'admin' WHERE (username IS NULL OR username = '') AND id = 'owner_admin';
UPDATE owners SET username = split_part(email, '@', 1) WHERE username IS NULL OR username = '';
