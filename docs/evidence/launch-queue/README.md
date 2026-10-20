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
