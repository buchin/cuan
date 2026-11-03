# Task completion indicators and speech grouping

Approved scope: soft green completed icons; shared16px pie progress beside Inbox
and project names; small same-surface speech tails on the first bubble of a group.
Uses existing --status-success / --status-working light/dark tokens, neutral
surfaces, exact localized accessible counts. No percentages rounded to100%.
Canceled/cancelled excluded from progress, review stays incomplete; no indicator
for empty/unknown scopes. Folders and avatars retain their existing identities.

Server read model uses one PostgreSQL grouped query across canonical tasks and
legacy project_tasks. Canonical status supersedes a mirror; UNION prevents double
counting primary/multiple project associations. No task bodies, output, attempts
or logs fetched for progress; all rows contribute independently of pagination.
In-memory mode follows the same precedence. Query failure returns an actionable
error rather than false completion. Existing task_count retains canceled work;
new canceled_tasks subtracts it only from progress.

Targeted tests: TestTaskProgressCountsThousandsAndMirrors and
TestTaskProgressCountsPostgres exercise7000tasks, all seven states, mirrors,
legacy-only rows and shared-project links. PostgreSQL test ran against an isolated
local database with all production migrations. Existing Inbox counter and project
API regressions also pass. Private report: cuan-progress-postgres-test.log.

Browser harness: docs/evidence/task-progress-check.cjs. Renders actual frontend
build at390/1440 in light/dark; verifies exact large counts, completion, no empty/
canceled-only/unknown indicator, review not complete, Indonesian labels, no page
overflow, shared16px size, colored completed icon, and tails on first speech only.
It also checks consecutive messages, six-minute gap, attachment-only message,
no tails on task cards, and pseudo-element color matching the actual surface.
Screenshots inspected under /home/dev/reports/cuan-task-progress/. Fixture SSE
ends deliberately, so its Connecting notice is not a production outage claim.

Pure grouping tests cover author/visibility boundaries, same-speaker continuity,
five-minute gap, new day and invalid/out-of-order dates. Existing message alignment
and task-counter frontend tests pass. Production smoke is recorded on release.
