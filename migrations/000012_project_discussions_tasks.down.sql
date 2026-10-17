-- Cuan Migration 000012: Project Studio Discussions & Tasks (Rollback)
-- (000012_project_discussions_tasks.down.sql)

DROP TABLE IF EXISTS project_tasks CASCADE;
DROP TABLE IF EXISTS project_messages CASCADE;
DROP TABLE IF EXISTS project_rooms CASCADE;
