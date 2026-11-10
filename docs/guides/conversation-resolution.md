# Resolve conversations and release work resources

Inbox defaults to Unresolved. All includes both unresolved and resolved rooms;
Assistant and Customers filter the audience. The checked-circle filter opens
Resolved. Sidebar counts are conversations still unresolved, not unread messages.
Read receipts and customer email delivery remain independent.

A task being Done does not resolve its conversation. Resolve explicitly from the
conversation header or the Tasks panel. The double-check action resolves rooms
including conversations without tasks and those whose work is Done or canceled.
It shows the eligible count, how many unfinished conversations stay in the inbox,
and any partial failures. Active work,
queued replies, unfinished tasks and newly arrived messages block the action.
A new message reopens the room. Team members share resolution, while their read
markers remain personal. Existing personal contact archiving remains unchanged.

Resolve authorizes disposal of that room's repository working copies, including
local workspace edits and generated files. No replacement archive is created.
The project repository, chat uploads, delivered attachments, message history,
email receipts, task records and saved verification reports remain. Standalone
artifact tasks are excluded from repository cleanup. Reopening does not recover
discarded uncommitted edits; saved committed project branches remain available.

The existing storage-maintenance scheduler is requested after resolve. It scans
bounded batches with a persistent-in-process cursor and rechecks room/task state
under execution ownership before deletion. Repository paths come from native task
identities and saved verification/publication proofs, never a user-supplied path.
Standalone clones and snapshots require a private Git root; symlink roots, project
roots, non-repo folders and active snapshot leases are refused. Ordinary retention
for unresolved work keeps its prior clean-copy/remote/age requirements.

Logs distinguish deleted repo copies and logical file bytes from device-space
savings. Hardlinks mean logical sizes must not be summed as physical savings.
Monitor filesystem free space before and after maintenance; concurrent activity
can also change that figure. A scheduled request is not proof of finished cleanup.
Deleted-task orphans without an authenticated room/task binding are not covered by
Mark all resolved; missing records are not resolution authorization.

Owner CLI:

```sh
cuan chat resolve ROOM_ID
cuan chat reopen ROOM_ID
cuan chat resolve-all
# Optional narrower operation: only rooms with at least one Done task
cuan chat resolve-completed
cuan maintenance status
cuan maintenance run
```

The CLI observes the exact current conversation cutoff before resolving and never
retries an uncertain write automatically. Bound task agents cannot use these Owner
operations. The UI and CLI use the same authenticated resolution endpoint:
`PATCH /api/v1/chat/conversations/{id}/resolution` with `resolved` and, when
resolving, the server-projected `resolution_observed_at` as `observed_at`.

Development verification: run the targeted conversation-resolution tests in
`internal/db`, `internal/server`, `internal/diagnostics`, plus resolved workspace
tests in `internal/git` and `internal/project`. The browser fixture under
`web/tests/conversation-resolution` renders the real InboxSubPanel, ComposerTasks
and resolution actions at390px/1440px. Set PLAYWRIGHT_MODULE and
CHROMIUM_EXECUTABLE as needed; serve Vite at127.0.0.1:5174 and run its check.cjs.
