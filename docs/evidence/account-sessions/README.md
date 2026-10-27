## Concurrent sessions per connected account

Application release: d1a313d9ead8b3cfdea799c77b0ae5fcb94b22ce.

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

## Production verification

Clean-worktree agent/server targeted tests pass, including integrated task
process-loss recovery (server checks 3.902s). Clean build and native atomic
activation/readiness pass. Read-only production mobile/desktop recovery and
English-default smoke pass. No model was called and no message/task retried.
All 36 application tables (auth sessions excluded) and 878 account-profile files
match the pre-release backup at /opt/cuan/backups/account-sessions-release.
Schema24 and disabled automatic migration are retained. Previous release: 3f98a03.
See production.json. Existing frontend build warnings remain unchanged.

The earlier live informational continuation accounts for the increased profile
file count versus previous 863-file snapshots; this release retained its own
878-file pre-release baseline exactly.
