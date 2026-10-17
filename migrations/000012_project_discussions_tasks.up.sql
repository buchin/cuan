-- Cuan Migration 000012: Project Studio Discussions & Tasks
-- (000012_project_discussions_tasks.up.sql)

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Table: project_rooms
CREATE TABLE IF NOT EXISTS project_rooms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id VARCHAR(64) NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    topic TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_project_rooms_project_id ON project_rooms(project_id);
CREATE INDEX IF NOT EXISTS idx_project_rooms_created_at ON project_rooms(created_at);

-- Table: project_messages
CREATE TABLE IF NOT EXISTS project_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id UUID NOT NULL REFERENCES project_rooms(id) ON DELETE CASCADE,
    author_type VARCHAR(50) NOT NULL DEFAULT 'user', -- 'user', 'agent', 'system'
    author_id VARCHAR(255) NOT NULL DEFAULT '',
    author_name VARCHAR(255) NOT NULL DEFAULT '',
    content TEXT NOT NULL,
    reply_to_id UUID REFERENCES project_messages(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_project_messages_room_id ON project_messages(room_id);
CREATE INDEX IF NOT EXISTS idx_project_messages_reply_to_id ON project_messages(reply_to_id);
CREATE INDEX IF NOT EXISTS idx_project_messages_author_id ON project_messages(author_id);
CREATE INDEX IF NOT EXISTS idx_project_messages_created_at ON project_messages(created_at);

-- Table: project_tasks
CREATE TABLE IF NOT EXISTS project_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id VARCHAR(64) NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    room_id UUID REFERENCES project_rooms(id) ON DELETE SET NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status VARCHAR(50) NOT NULL DEFAULT 'backlog', -- 'backlog', 'doing', 'review', 'done'
    priority VARCHAR(50) NOT NULL DEFAULT 'normal', -- 'low', 'normal', 'high', 'urgent'
    dod_checklist JSONB NOT NULL DEFAULT '[]'::jsonb,
    assignee_id VARCHAR(255),
    parent_task_id UUID REFERENCES project_tasks(id) ON DELETE SET NULL,
    batch_id VARCHAR(255),
    tags JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_project_tasks_project_id ON project_tasks(project_id);
CREATE INDEX IF NOT EXISTS idx_project_tasks_room_id ON project_tasks(room_id);
CREATE INDEX IF NOT EXISTS idx_project_tasks_status ON project_tasks(status);
CREATE INDEX IF NOT EXISTS idx_project_tasks_priority ON project_tasks(priority);
CREATE INDEX IF NOT EXISTS idx_project_tasks_parent_task_id ON project_tasks(parent_task_id);
CREATE INDEX IF NOT EXISTS idx_project_tasks_batch_id ON project_tasks(batch_id);
CREATE INDEX IF NOT EXISTS idx_project_tasks_tags ON project_tasks USING GIN (tags);
CREATE INDEX IF NOT EXISTS idx_project_tasks_created_at ON project_tasks(created_at);
