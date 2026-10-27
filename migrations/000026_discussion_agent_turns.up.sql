-- Durable agent turns for project discussions. A turn is separate from a task
-- attempt so chat can be observed, stopped, and resumed without losing state.
CREATE TABLE discussion_agent_turns (
    id UUID PRIMARY KEY,
    room_id UUID NOT NULL REFERENCES project_rooms(id) ON DELETE CASCADE,
    prompt_message_id UUID NOT NULL REFERENCES project_messages(id) ON DELETE CASCADE,
    reply_message_id UUID REFERENCES project_messages(id) ON DELETE SET NULL,
    agent_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','running','paused','completed','failed','cancelled')),
    resume_from_turn_id UUID REFERENCES discussion_agent_turns(id) ON DELETE SET NULL,
    error_message TEXT NOT NULL DEFAULT '',
    context JSONB NOT NULL DEFAULT '{}'::jsonb,
    steps JSONB NOT NULL DEFAULT '[]'::jsonb,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX discussion_agent_turns_room ON discussion_agent_turns(room_id, created_at);
CREATE INDEX discussion_agent_turns_active ON discussion_agent_turns(room_id) WHERE status IN ('queued','running','paused');
