## Durable launch storage checkpoint

Storage foundation only. Not connected to the dispatcher and not deployed.
Production remains on schema24; the schema25 migration is unshipped.

A launch intent stores the authorized task, owner, conversation, source request,
agent, revision, immutable instruction hash and original launch request identity.
Re-acceptance returns the same record and cannot change its scope. Claims use a
lease token, one-minute lease and bounded available/expiry timestamps. A stale
worker cannot finish another worker's claim. Cancellation and expiry are terminal
and cannot be undone by replaying the original acceptance.

The same contract passes against memory and an actual disposable PostgreSQL
instance: twelve concurrent accepts, twelve competing claims with one winner,
lease-loss reclamation, stale-token rejection, backoff, cancellation, replay and
expiry. PostgreSQL applies all embedded migrations including additive25 in a
new dedicated cuan_launch_queue_* database. The database is dropped afterward;
production database writes are zero. SQL check passed in 0.74 seconds.

No provider was called. These checks prove storage transitions, not an automatic
launch workflow. Remaining implementation: persist intent before launch; worker
scope/authorization revalidation and request dedup; typed temporary-route failures;
cancellation admission guards and UI state; restart/route-release integration
fixtures; notification delivery; documented migration/rollback and clean release.
The queue must not be shipped before those requirements pass.

## Dispatcher integration checkpoint (not deployed)

Authorized delegation now records durable intent before attempting launch. Known
route/capacity failures reschedule the same request with 30-second backoff; the
Inbox dispatcher resumes it independently of the browser. Scope/project/agent
and original accepted authorization are checked before launch. Store admission
also rejects cancelled/stopped/expired or changed-scope intent under the task's
transaction lock. Backend DTO exposes only public launch state, never lease tokens
or stored owner metadata.

Integrated fake-provider tests reconstruct service and terminal manager over the
same stores, release the route, and use the actual queue drainer: one task, one
attempt, one native fake process and one start notification. Cancellation and
changed instructions prevent execution. The existing direct/structured approval,
process-loss recovery and launch identity tests pass (36.778s server checks).
Updated real PostgreSQL contract including cancellation-at-attempt-admission and
valid admission passes in0.79s; its isolated database is dropped afterward.

Browser fixtures at390/820/1440 show automatic waiting, no agent selector or
manual Start, durable cancel action and no horizontal overflow. Build passes with
existing warnings. See browser.json and waiting screenshots. This is test evidence,
not a production launch or provider-inference claim.

Clean-tree targeted memory/server checks pass (2.829s server) and the clean
TypeScript/Vite build passes without extension WIP. The latest PostgreSQL
admission contract passes in0.66s.

Still required before release: full PostgreSQL-backed server reconstruction scenario; exact schema25 activation
and rollback gates; private backup/migration/retention and read-only production
smoke. Production remains d1a313d/schema24 until those gates pass.

## PostgreSQL server and activation gates

The full server reconstruction fixture passes against a dedicated disposable
PostgreSQL database (5.90s): unavailable route retains one launch, reconstructed
repositories/service/manager start one attempt when the route returns, cancellation
remains terminal, and edited instructions do not execute. Three cases use real
PostgreSQL tasks/messages/launch rows plus a fake native provider. The dedicated
database is dropped afterward; production database writes are zero.

The native activator now targets exact schema25 and checks persisted Inbox,
attempt and launch state before release/rollback. Unknown database state fails
closed. Seven targeted Python tests pass, including a pending-launch block and
schema24 upgrade without a launch table. This is a pre-switch state check, not
an HTTP admission lock; incoming requests keep normal durable restart semantics.
Production backup/migration, activation and rollback verification remain pending.
