## SQL-backed approved-goal interruption and reconstruction

`TestFreshInstallPostgresApprovedGoalRecovery` passes in6.58s against an empty,
dedicated real PostgreSQL database. Every application store comes from
`NewServer(cfg, database)`; no memory stores are substituted. The local provider
and Bubblewrap launcher are deterministic fixtures, not external inference.

The scenario migrates to25, creates one owner despite six simultaneous setup
requests, connects a dedicated fixture account, and submits a goal through Studio.
The provider presents a structured exact-scope proposal; no task exists before
approval. The owner's approval uses the canonical saved approval request identity.

A real child shell writes one artifact, reports a progress-only checkpoint, and
receives SIGKILL before producing a final summary. The test discards the original
server, service and terminal-manager instances, retaining only PostgreSQL and
persistent HOME/workspaces. A new server's normal Inbox dispatcher immediately
reviews the interrupted attempt. The review must contain the saved checkpoint;
it requests a scoped revision of the same task under the original authorization.
The replacement shell verifies the artifact, checkpoint and no-deploy constraint
before reporting done. The dispatcher automatically delivers the verified final
result without a browser reconnect, user message or manual delivery-helper call.

Assertions after another reconstruction prove one task, two attempts, one artifact
creation, exactly one final coordinator result, retained owner session and retained
renamed project. Replaying the initial Studio goal does not duplicate execution.
The companion direct-goal SQL lifecycle still passes after the shared fixture
refactor (4.37s). Both dedicated databases were dropped; no production database
writes or external provider calls occurred.

## Reproduce

Use separate empty disposable databases for these two tests. The guard rejects
names outside `cuan_fresh_install_*` and rejects nonempty databases. Supply a DSN
for a role with migration privileges, for example using local PostgreSQL peer auth:

```sh
createdb cuan_fresh_install_recovery
CUAN_FRESH_INSTALL_DATABASE_URL='postgres:///cuan_fresh_install_recovery?host=/var/run/postgresql' \
  go test ./internal/server -run '^TestFreshInstallPostgresApprovedGoalRecovery$' -count=1 -timeout 35s
dropdb cuan_fresh_install_recovery
```

## Boundaries and production relationship

This proves application-server reconstruction over durable SQL state after actual
agent process loss, not a literal host reboot or arbitrary external side effects.
OS supervisor parent-loss/ownership checks and real native binary restart evidence
are separate checks in task-recovery and onboarding. The provider fixture verifies
recovery contracts rather than a live model's interpretation of every goal.

Only test/evidence files changed in this checkpoint. The production implementation
remains49f89ec/schema25, whose native delivery, rollback/reactivation and retained
data checks passed. Docker distribution/runtime verification remains open in the
full goal audit.
