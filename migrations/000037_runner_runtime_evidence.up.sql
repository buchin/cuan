-- Native readiness must survive API/chat worker process boundaries. Old runners
-- omit evidence and old binaries ignore this additive column.
ALTER TABLE runners ADD COLUMN IF NOT EXISTS runtime_evidence JSONB NOT NULL DEFAULT '{}'::jsonb;
