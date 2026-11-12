-- Declares which stages an approved task must reach. NULL keeps the previous
-- behaviour for existing rows, so no running project silently changes gates.
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS required_stages TEXT[];

COMMENT ON COLUMN tasks.required_stages IS
  'Declared acceptance stages: implement, verify, publish. NULL derives legacy behaviour.';
