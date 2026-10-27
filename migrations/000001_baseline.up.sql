-- Cuan Baseline Migration (000001_baseline.up.sql)
-- Defines core schema for single-owner projects, tasks, runner attempts, and agent logs.

CREATE TABLE IF NOT EXISTS cuan_schema_migrations (
    version VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS owners (
    id VARCHAR(64) PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    is_setup BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS projects (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    repo_url TEXT NOT NULL,
    default_branch VARCHAR(100) DEFAULT 'main',
    local_path TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tasks (
    id VARCHAR(64) PRIMARY KEY,
    project_id VARCHAR(64) REFERENCES projects(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    instructions TEXT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'backlog', -- backlog, doing, review, done, blocked
    priority VARCHAR(32) NOT NULL DEFAULT 'normal', -- low, normal, high, urgent
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_tasks_project_id ON tasks(project_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);

CREATE TABLE IF NOT EXISTS task_attempts (
    id VARCHAR(64) PRIMARY KEY,
    task_id VARCHAR(64) REFERENCES tasks(id) ON DELETE CASCADE,
    runner_id VARCHAR(64) NOT NULL,
    agent_name VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'queued', -- queued, running, succeeded, failed, canceled, interrupted
    exit_code INT,
    peak_rss_bytes BIGINT DEFAULT 0,
    started_at TIMESTAMP WITH TIME ZONE,
    finished_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_attempts_task_id ON task_attempts(task_id);
CREATE INDEX IF NOT EXISTS idx_attempts_status ON task_attempts(status);

CREATE TABLE IF NOT EXISTS agent_logs (
    id BIGSERIAL PRIMARY KEY,
    attempt_id VARCHAR(64) REFERENCES task_attempts(id) ON DELETE CASCADE,
    stream VARCHAR(16) NOT NULL, -- stdout, stderr, system
    line TEXT NOT NULL,
    logged_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_agent_logs_attempt_id ON agent_logs(attempt_id);
