## Durable authorized launch recovery

Production application release: e828d1fcb1248401155404c3fcfdcaa518ae3c58, schema25.

Accepted delegation records a durable launch intent before attempting execution.
Temporary account/model unavailability or full session slots reschedule the same
launch request after30 seconds. A dispatcher resumes waiting launches independently
of browser state and after server reconstruction. The original task, owner,
conversation, agent, project, revision and instruction hash remain bound.

Workers revalidate the persisted authorization and scope. Attempt admission also
checks cancellation, lease, agent, project and instruction hash under the task's
transaction lock. Deterministic request/attempt identity and OS task ownership
prevent replay from starting replacement work twice. Cancel/expiry are terminal;
non-transient errors stop instead of looping. The wait window is24 hours.

Task detail exposes only public launch state. Waiting UI removes the agent picker
and manual Start and offers Cancel waiting work. Start/stop messages use stable
receipt IDs in the original conversation. The queue starts accepted work; existing
attempt failures still use their separate evidence review and recovery workflow.

## Verification

- Memory and real disposable PostgreSQL storage contracts: twelve-way accepts and
  claims, immutable intent, lease reclaim, stale worker rejection, backoff, expiry,
  cancellation replay, cancelled-attempt rejection and valid admission. Latest
  SQL contract passed in0.66s.
- Full PostgreSQL-backed server reconstruction: route release starts one task,
  attempt, native fake process and notification; cancel stays cancelled; changed
  instructions do not execute. Three cases passed in5.90s. The isolated database
  was dropped. No production data or provider inference was used for these tests.
- Clean-tree targeted queue, direct delegation, structured approval and process-loss
  checks passed (36.873s server); embedded migration loading passed. Seven Python
  activation tests passed, including pending-launch block, schema24 upgrade and
  fail-closed database gate.
- Clean TypeScript/Vite build and browser fixtures at390/820/1440 passed: automatic
  wait, no agent selection/manual start, cancel and no horizontal overflow.
  Existing CSS/chunk build warnings remain. See browser.json and screenshots.

## Native production release

The pre-migration backup at /opt/cuan/backups/launch-queue-release contains a
validated private pg_dump, provider profiles and exact table snapshots. No Inbox
or task work was active. The checksummed candidate migrator applied additive25
with automatic migration disabled. The old d1a313d application remained healthy
on schema25 before the application switch.

Atomic activation/readiness passed. Actual rollback to d1a313d and reactivation
of e828d1f both passed while retaining schema25, with no down migration. The native
activator targets exact25 and checks active Inbox/attempt/pending-launch state
before switching. This is a pre-switch gate, not an HTTP admission lock; requests
arriving afterward retain their normal durable restart/recovery behavior.

Read-only production mobile/desktop recovery, English default and served waiting
assets passed. No production message, retry, task or paid provider call was made.
All36 pre-existing application tables (auth sessions excluded) and878 profile
files matched the backup afterward; the new launch table was empty. See
production.json. This release does not claim a live production queued model run.

Usage, cancellation and migration/rollback guidance: docs/guides/launch-queue.md.
The full product goal still requires the remaining onboarding/documentation audit.
