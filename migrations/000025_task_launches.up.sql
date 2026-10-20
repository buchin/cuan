-- Additive durable launch intent. Quiesce queued launches before app rollback.
CREATE TABLE task_launches (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL,
    request_id TEXT NOT NULL,
    attempt_id TEXT NOT NULL,
    owner_id TEXT NOT NULL,
    room_id UUID NOT NULL REFERENCES project_rooms(id) ON DELETE CASCADE,
    source_request_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    revision TEXT NOT NULL DEFAULT '',
    instructions_hash TEXT NOT NULL,
    state TEXT NOT NULL DEFAULT 'queued' CHECK (state IN ('queued','claimed','started','stopped','cancelled')),
    available_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    lease_token TEXT NOT NULL DEFAULT '',
    lease_until TIMESTAMPTZ,
    checks INTEGER NOT NULL DEFAULT 0,
    safe_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE(task_id,request_id)
);
CREATE INDEX task_launches_due ON task_launches(available_at,id) WHERE state IN ('queued','claimed');

CREATE INDEX task_launches_attempt ON task_launches(attempt_id);
