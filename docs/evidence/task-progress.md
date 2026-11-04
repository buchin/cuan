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

## Approved noise reduction and cancellation transparency

Zero-progress pies are now hidden; Inbox additionally requires at least two tasks
(including canceled tasks). Single-task status remains in its task card. Errors
are not hidden: a quiet amber icon and explicit count/Needs attention text persist
at zero progress; working and genuine review counts remain visible.

The shared44px button opens a nonmodal exact-count breakdown by tap/click, keyboard
or desktop hover. Trigger is a sibling of the row's navigation button/link, never
nested interactive content. Panel is rendered in a portal, fits the viewport,
closes on outside/Escape/scroll/resize and restores focus. A complete remaining
scope with cancellations uses a neutral filled pie and an explicit remaining-work
heading, not an all-success check. Unknown breakdown metadata uses Remaining
rather than inventing task states. Empty/unknown/all-canceled scopes stay quiet.

Inbox-only quiet ItemActions reclaim the permanent menu column; hover, long press,
context menu and keyboard access remain available. Recognizable system reports
get a concise preview reflecting current counters; ordinary message text and the
full report are preserved. Negative evaluation is distinguished from ordinary
PERLU DIREVIU so awaiting review alone is not presented as failure.

Production root cause confirmed with official read-only MCP: Visitor #5ea132 has
one canonical review task, msg_3ff6e2dc-669d-49a3-b5c6-d4e185fd0311, whose latest
attempt is canceled with cancellation recorded. The old counters counted its
stored review status, while task cards used attempt state. No task/attempt history
was rewritten. The read model now reads only latest attempt/launch state columns
and applies consistent presentation precedence: accepted done is preserved, a new
queued launch/new attempt supersedes older cancellation, and stopped/canceled
work does not masquerade as pending review. Conversation snapshot counters use
the same model; frontend preserves those server counters instead of replacing
them with raw task status counts. Card cancellation wins over a stopped launch.

Tests now include authentic API list/snapshot cancellation consistency, preserved
stored review, latest-attempt resume, queued launch resume, accepted done,7000-task
PostgreSQL and memory fixtures. Browser checks use real touch/coarse-pointer390px
and desktop1440px contexts in light/dark, including long press without navigation,
keyboard,44px targets with16px glyphs, no nested buttons/links, mixed cancellation
breakdown, zero-progress attention markers, truthful previews and live refresh.
Screenshots inspected at `/home/dev/reports/cuan-inbox-noise/`.
