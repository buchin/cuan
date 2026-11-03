# One assistant, multiple AI brains

Studio and new conversations use the public assistant (`agent_cuan`), including
its saved display name in **Settings → Assistant & AI Brain**. Internal account
names and historical roles are not recipients users need to choose. Existing
conversations keep their IDs and history; new turns use the public identity.

## Settings

The assistant name is saved on the server and restored after reload. The browser
cache is updated only after a successful save. AI brains appear as a quiet list;
open a row to edit its display name and preferred model, check the connection,
or disconnect it. Leave the preferred model empty for Auto. Legacy server
profiles remain read-only; their authentication is managed by the installation.
BYOK entries retain their existing provider configuration and connection checks.

The preferred model is a preference for that connection, not a promise that the
provider makes it available. An unsupported model can fail; connection status and
recorded output remain the source of truth. The UI does not infer vision support
or readiness from a hardcoded model name. Saved keys are never displayed in full.

## Native brain routing and recovery

Only the request owner's explicitly saved private native profiles are eligible.
The pool distributes new selections across eligible profiles, skips cooldowns,
and uses cross-process session leases to avoid oversubscribing an account.
The runner receives the owner identity from the authenticated server claim.
It does not borrow an ambient runner login for an owner-scoped claim.

When a native brain reports exhausted quota, the runner finishes that process,
records its cooldown, and selects another eligible brain. It keeps the same task,
attempt, workspace and approval. The continuation prompt requires inspection of
saved files, reports and external effects before performing unfinished work;
uncertain operations must be reconciled before being repeated. Native provider
sessions are not transferred across accounts. A confirmed final task report is
not repeated. Cancellation stops the routing loop as well as the current process.

Provider failures and task interruptions are different from business revisions.
Quota interruptions are recorded as usage-limit failures instead of user
cancellation. Existing launch guards, durable approval, active-work checks and
idempotency remain in force. When capacity is unavailable before launch, the
existing task stays queued and is checked automatically until its launch expires;
Cuan does not create another task or reassign the public assistant to a brain.

Safe Inbox reply turns that exhaust available capacity remain queued as the
**same saved request**. The dispatcher rechecks after 30 seconds. Cancellation
remains available, and unknown or unsafe tool outcomes are not automatically
replayed. Capacity waiting is bounded to 24 hours from acceptance; after that,
the message is retained for an explicit retry. Old failed/cancelled requests are
not silently replayed when this release is installed.

## Work status and typography

Only doing/in-progress/running tasks count as working. Review is counted
separately; backlog, queued, cancelled and completed tasks are not working.
Stopped task cards do not claim successful completion. Tasks no longer expose
an Agent column, assignee filter or assignee selector. Historical execution
metadata remains available for diagnostics.

The Project view supplies the reference hierarchy: readable 14px primary list
text, muted 12px metadata and clear spacing. Inbox and Tasks follow that hierarchy.
The sidebar brand uses a 28px mark and 18px wordmark. Account status in settings
uses a single text summary; model and connection details belong in a SlideDrawer.

## Slow repository connections

Cuan persists a new project's identity before cloning. Cloning has a 45-second
budget, reduced when necessary to leave five seconds for persisting status before
the request deadline. A slow download leaves a visible, saved project with a clear
synchronization message. Retry synchronization on that project rather than
creating a duplicate.

## Verification, rollout and rollback

On mobile, open **Akun → Pengaturan → Assistant & AI Brain** to manage the
assistant and its brains. The settings shortcut is directly below your identity
in the account menu, marked with a gear icon. It is also available from the
desktop account menu and the Settings sidebar entry.

Targeted Go tests cover ownership, pool leases and cooldowns, quota classification,
continuation of the same workspace, cancellation, durable Inbox capacity waiting,
review counts, assistant renaming, brain metadata editing and slow clones. The
frontend deployment gate covers typechecking and browser-related regression tests.
Real desktop and mobile 390px previews are inspected before release.

Release through `fural-agent repos ship` and the repository's atomic native
release script. There is no database migration. Existing private profile metadata
accepts an optional `model_id`; older metadata remains readable. Verify the health
SHA, native runner restart, public conversation identity and saved settings after
release. The native activation script restores the previous release on failed
health checks. Operational rollback uses the existing documented release rollback;
saved conversations, credentials and project settings remain intact.
