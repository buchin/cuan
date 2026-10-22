-- Additive: old applications ignore these columns and can still run on schema23.
ALTER TABLE tasks
    ADD COLUMN creation_hash TEXT NOT NULL DEFAULT '',
    ADD COLUMN summary TEXT NOT NULL DEFAULT '',
    ADD COLUMN complete_note TEXT NOT NULL DEFAULT '',
    ADD COLUMN dod_checklist JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE task_attempts ADD COLUMN request_hash TEXT NOT NULL DEFAULT '';
