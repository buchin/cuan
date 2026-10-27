-- Additive, independent of existing rooms. Email is claimed metadata, never a key.
CREATE TABLE crm_contacts (
 id UUID PRIMARY KEY,
 product TEXT NOT NULL,
 guest_uuid TEXT NOT NULL,
 session_uuid UUID NOT NULL UNIQUE,
 name TEXT NOT NULL,
 email TEXT NOT NULL DEFAULT '',
 tags JSONB NOT NULL DEFAULT '[]',
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE(product, guest_uuid)
);
CREATE INDEX crm_contacts_updated ON crm_contacts(updated_at DESC);
