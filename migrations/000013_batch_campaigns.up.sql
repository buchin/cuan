-- Cuan Migration 000013: Batch Campaigns Engine
-- (000013_batch_campaigns.up.sql)

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Table: batch_campaigns
CREATE TABLE IF NOT EXISTS batch_campaigns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    template_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    target_criteria JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(50) NOT NULL DEFAULT 'draft',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_batch_campaigns_status ON batch_campaigns(status);
CREATE INDEX IF NOT EXISTS idx_batch_campaigns_created_at ON batch_campaigns(created_at);
CREATE INDEX IF NOT EXISTS idx_batch_campaigns_title ON batch_campaigns(title);
CREATE INDEX IF NOT EXISTS idx_batch_campaigns_template_payload ON batch_campaigns USING GIN (template_payload);
CREATE INDEX IF NOT EXISTS idx_batch_campaigns_target_criteria ON batch_campaigns USING GIN (target_criteria);
