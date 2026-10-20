## Waiting work

When the coordinator saves authorized work but no allowed account/model route is
available, Cuan retains its launch request and reports that it is waiting. You do
not need to choose another agent or keep the browser open. The dispatcher checks
again after a 30-second delay and can continue after service restart. Each check
is account/route preflight; it does not run model inference while no route exists.

The queue retains the original task, scope, project, account owner, agent and
execution request ID. It revalidates the saved authorization and instruction hash
before starting. An altered scope stops the old launch instead of running a new
plan under old approval. A deterministic attempt ID and OS execution lock prevent
replacement work from overlapping an existing attempt.

Open **View work** to see waiting status or choose **Cancel waiting work**. Queued
cancellation is durable and blocks late attempt admission. An in-flight start
that already produced an active attempt uses normal process cancellation; effects
that already happened cannot be undone. Stopped/cancelled launches do not resume
merely because the original request is delivered again. Explicit new work can use
a new execution request.

Pending launches expire after 24 hours. Cuan records the stop reason and reports
the outcome in the original conversation. Non-transient configuration, validation
or launch errors stop rather than loop indefinitely. Active/failed task attempts
keep their existing reporting/review/recovery rules; this queue is for launching
accepted work, not blindly repeating already-started execution.

## Deployment gate

This implementation needs additive migration25 (task_launches). The release requires the gates below. Back up the database and private provider profiles, verify the archive,
and quiesce both active attempts and pending launches before migrating. The atomic activator targets exact schema25 and rejects active/waiting work.

Application rollback must keep schema25 and must not start an old application
while pending launches or active attempts remain. Older releases ignore the launch
table and cannot drain it. Do not down-migrate as a routine rollback: that would
discard saved execution intent. Verify targeted memory/real PostgreSQL concurrency,
restart, cancellation admission, scope-change and UI checks before deployment.
