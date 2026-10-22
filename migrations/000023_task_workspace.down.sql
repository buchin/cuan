-- Application rollback should preserve schema/data; down is destructive.
ALTER TABLE task_attempts DROP COLUMN request_hash;
ALTER TABLE tasks DROP COLUMN creation_hash, DROP COLUMN dod_checklist, DROP COLUMN complete_note, DROP COLUMN summary;
