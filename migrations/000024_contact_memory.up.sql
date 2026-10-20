-- Additive: rollback the application without dropping retained relationship notes.
CREATE TABLE contact_memory (
 id UUID PRIMARY KEY,
 contact_id UUID NOT NULL REFERENCES crm_contacts(id) ON DELETE CASCADE,
 kind TEXT NOT NULL CHECK (kind IN ('note','fact','birthday')),
 body TEXT NOT NULL CHECK (length(body) BETWEEN 1 AND 6000),
 recorded_by TEXT NOT NULL,
 source_room_id TEXT NOT NULL DEFAULT '',
 source_message_id TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 deleted_at TIMESTAMPTZ
);
CREATE INDEX contact_memory_contact_date ON contact_memory(contact_id,created_at DESC,id DESC) WHERE deleted_at IS NULL;
