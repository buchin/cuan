-- Cuan Migration 000030: Multi-User Collaboration, Staff Invitations & RBAC
-- (000030_multi_user_and_invites.down.sql)

DROP TABLE IF EXISTS user_invitations CASCADE;

DROP INDEX IF EXISTS idx_owners_role;
ALTER TABLE owners DROP COLUMN IF EXISTS role;
ALTER TABLE owners DROP COLUMN IF EXISTS name;
ALTER TABLE owners DROP COLUMN IF EXISTS is_active;
