# Task duration

Duration should answer how long the user has waited for the requested result.
Measure from `created_at` to the native Done transition, including admission,
development, independent verification and final PM review. It does not include
the initial discussion before the task exists.

Task detail and task listings expose optional `completed_at`, read from the
latest persisted `status_change` activity only when the current task is Done.
It is not `updated_at` and not the latest attempt's `finished_at`: the former
changes on edits, while the latter omits PM acceptance and earlier attempts.
List reads batch the transition history for the returned Done tasks. No schema
migration, worker execution or historical status rewrite is involved.

Missing/contradictory history leaves `completed_at` absent. Do not substitute the
last edit timestamp or display a successful completion duration for stopped,
failed, canceled or reopened work. A later genuine Done transition replaces the
earlier completion when the work is reopened and accepted again.

The shared `TaskDuration` display uses `Durasi 3 menit` / `Elapsed 3 minutes`
during active work and `Selesai dalam 7 menit` / `Completed in 7 minutes` once accepted.
Rows use the compact duration with an accessible full label and explanatory title;
detail and expanded Inbox tasks show the full label. Completed tasks without
known completion history retain their ordinary last-update timestamp in rows.
Use minute-level refresh without another status pill. Avoid estimated time left
until sufficient data supports an estimate. Detailed phase timing needs attempt
and review intervals; elapsed lifecycle duration must not be described as model
compute time. Paused or user-blocked time remains part of elapsed time and must
not be labeled continuous agent execution.

The Inbox card's old per-attempt, per-second timer is removed. The shared display
uses minute refresh and preserves elapsed time across dev/verifier handoff.
It appears in task rows/table/cards, expanded Composer Tasks and task details.
No API polling is added for the timer. API omission means unknown, not0.

Built-app browser verification: `node web/tests/task-duration/check.cjs`.
The isolated API fixture exercises390px and1440px in light/dark, detail and
Inbox accordion, English/Indonesian and multi-day formatting. Advancing the
browser clock verifies live minute refresh and frozen Done duration; deleting
fixture completion evidence verifies no fallback to last-edit duration.
Actual screenshots under `/home/dev/artifacts/cuan-task-duration/` were opened
and evaluated. Quiet muted text fits the existing rows/header without another
badge, progress bar or counter. Desktop header/data column alignment was checked
and corrected during visual inspection. The simulated Inbox connecting notice
belongs to the isolated fixture, not an observed production outage.

The first release attempt (`d84074a8`) stopped in preflight before activation:
three legacy source-contract tests still required the removed per-attempt timer
and literal Updated header. Their duration assertions now require the shared
creation/Done contract without per-attempt reset. Typecheck and all463 frontend
tests pass. Browser runtime assertions remain in place; no failing gate was waived.

Release `f1c1e47395c05f906584bcb405009c2ab55e1a80` passed exact-SHA production
health with38 migrations at2026-11-09T05:31:26Z. Read-only official CLI checks
confirmed both detail and batched list expose the native completion time for
task `msg_8832ed4a-9219-4068-bf24-662ea8250f04`:
created13:04:23.220706+08, completed13:09:31.985382+08. Completion exactly
matches its saved Done transition, giving308.765 seconds from task creation.
Initial PM discussion before creation is deliberately excluded. Two older Done
tasks also expose their matching completion events through the actual SQL-backed
list. No production task was edited merely to test timer freezing; that behavior
is covered by the isolated lifecycle and browser-clock tests above.
