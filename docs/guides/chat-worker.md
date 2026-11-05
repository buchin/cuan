# Chat worker

Chat runs in its own process and durable Inbox queue, independently of the task
runner queue. The webserver accepts messages, exposes history/cancellation and
streams progress. The chat worker resolves context, calls the provider and
publishes validated replies or authorized delegation through existing services.
Task execution remains in `cuan-runner`.

## Process roles

- `cuan-server --external-chat-worker`: API/UI and task runtime maintenance;
  never launches an Inbox provider process.
- `cuan-server --chat-worker`: Inbox dispatch and result review, with no HTTP
  listener, scheduled maintenance or startup task reconciliation. PostgreSQL must
  be reachable; it cannot fall back to an in-memory queue.
- Without either flag, standalone development retains embedded execution.

Both roles reuse the existing services, authorization, FIFO claims, deadlines,
approval gates, cancellation checks and provider-account leases. They do not
create a second task queue or an additional broker dependency. Different rooms
can run concurrently; messages within a room remain ordered. An interrupted
worker never blindly replays a running request.

PostgreSQL `LISTEN/NOTIFY` carries room IDs only to wake dispatch and SSE. The
database retains the actual state. Dispatcher scans recover missed wakeups;
API listener reconnects refresh its subscriptions. A worker continues execution
when an API process or its SSE connection restarts. A long provider timeout is
still possible and must be diagnosed separately from deployment interruption.

## Capacity

Chat and task concurrency are separate. They still share provider quotas and
cross-process account session locks. At the default four sessions per account,
long tasks can lease three slots and chat can use the remaining slot. Chat tries
the reserved end of the slot range first. Optional operator settings are
`CUAN_ACCOUNT_MAX_SESSIONS` and `CUAN_ACCOUNT_CHAT_RESERVED_SESSIONS` (default 1).
Reservation 0 restores full task use. Single-session accounts retain one shared
slot so task work remains possible; reservation cannot create provider capacity.

## Native releases and rollback

`scripts/deploy.sh` declares `features.inbox_worker=1`. The activator installs
`<api-service>-chat@<release-sha>.service` under the retained service identity and
environment, pinned to its immutable release. Readiness verifies the worker's
reported release/PID against its actual executable. It starts no HTTP port.

The first transition from embedded execution drains the old API's active chat
before switching roles. Later API releases hold the short admission barrier but
allow active work to continue in the independent worker. A ready new worker
generation starts before the old generation stops accepting work. Old workers
receive SIGTERM only at their supervisor, finish owned conversations within
their original deadlines and exit. They are disabled for reboot; only the new
generation stays enabled. Release retention protects running executable/cwd
paths. Static-only activation leaves the worker generation alone.

Worker readiness or API health failure restores the old API mode and release.
Rollback to a release without the feature first drains active conversations and
then returns to embedded execution. Never force-switch a schema incompatible
with a running worker: this protocol assumes backward-compatible migrations.

## Verification

Targeted Go tests cover API shutdown during FIFO processing, PostgreSQL-backed
claims, event delivery after API listener reconnection, graceful worker drain,
and account capacity reservation. Activation tests cover admission barriers,
pinned worker units, independent shutdown and stopping only old generations.

The production check must additionally submit a harmless, task-free diagnostic
through the official Cuan CLI, confirm its request is running in the chat worker,
restart only the API, then verify the same worker PID and request produce one
saved reply. Do not replay a customer's operational task for this test.
