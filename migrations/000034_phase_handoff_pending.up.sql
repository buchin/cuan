CREATE INDEX IF NOT EXISTS idx_phase_handoff_pending ON task_activities(created_at,id)
WHERE activity_type='phase_handoff_pending' AND author='cuan-plan';
CREATE INDEX IF NOT EXISTS idx_phase_handoff_retry ON task_activities(task_id,created_at)
WHERE activity_type='phase_handoff_retry' AND author='cuan-plan';
