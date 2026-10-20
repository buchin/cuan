## Concurrent sessions per connected account

Pending clean release verification; not deployed yet.

Replaced the profile-wide exclusive execution lease with a shared profile gate
plus bounded exclusive slot files. Default capacity is four sessions per profile;
CUAN_ACCOUNT_MAX_SESSIONS accepts 1–16. Old exclusive-lease application releases
remain incompatible with shared sessions while active, because the same gate
path is retained. Deployment still quiesces all work before restart.

Backend OAuth refresh retains an exclusive gate. It cannot rewrite credentials
while sessions use the profile, and sessions cannot start during that rewrite.
This does not replace the provider CLI's own credential/state coordination.
All sessions share the provider account's quota and rate limits.

Targeted tests pass: twenty parallel claimants obtain exactly four slots;
owner isolation and refresh exclusion hold; release is idempotent; killing an
actual child process releases its slot; a simulated long task account lease no
longer prevents a fake-provider Inbox response on the same account. A full
capacity does not invoke the provider, emits safe no-execution proof, and is
reported separately from quota. Existing usage-refresh, quota-failover and task
preflight checks pass. No live parallel inference was started.

The durable launch queue foundation in the worktree is separate, unfinished,
and excluded from this release. Its schema25 migration must not ship here.
