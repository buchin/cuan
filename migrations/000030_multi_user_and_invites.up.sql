-- Cuan Migration 000030: Multi-User Collaboration, Staff Invitations & RBAC
-- (000030_multi_user_and_invites.up.sql)

-- 1. Extend owners table for multi-user support with roles (owner/member) and active state
ALTER TABLE owners ADD COLUMN IF NOT EXISTS role VARCHAR(32) NOT NULL DEFAULT 'owner';
ALTER TABLE owners ADD COLUMN IF NOT EXISTS name VARCHAR(255);
ALTER TABLE owners ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE;

CREATE INDEX IF NOT EXISTS idx_owners_role ON owners(role);

-- 2. Staff Invitations table for zero-friction team onboarding
CREATE TABLE IF NOT EXISTS user_invitations (
    id VARCHAR(64) PRIMARY KEY,
    token_hash VARCHAR(64) UNIQUE NOT NULL,
    email VARCHAR(255) NOT NULL,
    role VARCHAR(32) NOT NULL DEFAULT 'member',
    created_by VARCHAR(64),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    accepted_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_user_invitations_token_hash ON user_invitations(token_hash);
CREATE INDEX IF NOT EXISTS idx_user_invitations_email ON user_invitations(email);
