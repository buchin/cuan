# Cuan Deployment Integrity & Anti-Regression Standard

## 1. Overview

Cuan employs an atomic, versioned deployment system on native host environments (such as systemd services on Linux VPS). To prevent deployment regressions, stale process execution, or pseudo-releases where symlinks point to new directories while stale processes run in memory, the following integrity controls are enforced.

### Inbox admission during native activation

Before switching a binary release, the activator takes an exclusive filesystem
lock on `inbox-admission.lock` beside the configured workspace root. The server
takes its shared side only while claiming a durable Inbox request. This closes
the gap between checking for active conversations and starting a new one.
Existing turns retain their deadlines and finish normally. Incoming messages
are still accepted into the durable queue and resume under their original IDs
after the lock is released and the dispatcher scans again.

The gate waits only for `running`/`cancelling` Inbox requests; queued requests do
not block a release. It remains mandatory when `CUAN_DEPLOY_ALLOW_RESUME` permits
resumable task work. A bounded eleven-minute wait aborts activation rather than
cancelling conversations. Exceptions, health failure/rollback and successful
activation release the lock. Static-only `--no-restart` releases do not pause
Inbox admission. All server instances on this native host must use the same
configured workspace root and support the claim lock. For the first rollout
from an older binary, its existing Inbox must be idle before activation; that
older dispatcher cannot participate in the new admission lock.

Targeted verification: `python3 scripts/test-activate-release.py` and
`go test ./internal/server -run TestInboxDeploymentGatePreservesActiveAndQueuedRequests`.

---

## 2. Commit SHA Health Verification Contract

The `/healthz` liveness endpoint exposes the active running binary metadata:

```json
{
  "status": "healthy",
  "app": "cuan",
  "version": "0.9.0",
  "commit_sha": "616f3840dc2dcdbbe962d639937da250b1d694c4",
  "timestamp": "2026-10-25T09:30:00Z"
}
```

### Detection Hierarchy
The active binary resolves its `commit_sha` via:
1. Compile-time linker flags (`-ldflags "-X github.com/buchin/cuan/internal/config.GitCommit=<SHA>"`).
2. Environment variable override (`CUAN_COMMIT_SHA`).
3. Release manifest lookup (`release.json` in current or parent working directory, or `/opt/cuan/current/release.json`).

---

## 3. Strict `--no-restart` Guard

The deployment script `scripts/activate-release.py` supports `--no-restart` **strictly for static asset changes only** (e.g., CSS/JS bundles in `web/dist/`).

If the checksum of `bin/cuan-server` differs between the currently active release (`current/bin/cuan-server`) and the target release, `activate-release.py` will abort immediately with:
```
RuntimeError: Cannot use --no-restart when bin/cuan-server binary has changed; service restart is required
```

This prevents leaving an outdated Go server process running in memory with an outdated schema or business logic while the file tree has moved.

---

## 4. End-to-End Post-Deployment Verification Gate

`scripts/deploy.sh` executes an end-to-end verification gate against `https://cuan.dojo.cc/healthz`:
- Must return `HTTP 200 OK` with `"status": "healthy"`.
- Must return `"commit_sha"` identical to the target `$release_sha`.
- If the running process reports a stale SHA, the deploy script waits and retries for service restart. If the SHA fails to match within the threshold, deployment halts with an error.

---

## 5. Rollback Behavior

If `systemctl restart cuan` fails or `/healthz` / `/readyz` fails health checks after symlink switch:
- `activate-release.py` automatically restores symlinks to the previously known healthy release (`previous`).
- Restarts and validates health of the restored release.
- Exits with a descriptive status code indicating rollback success or manual intervention requirement.

---

## 6. Worker Daemon Graceful Drain & Auto-Undrain Hook

Starting with Sprint 56, background task attempts are decoupled from HTTP server processes and executed by the standalone `cuan-runner` daemon:
1. **Graceful Drain Initiation**: Before symlink atomic switch, `activate-release.py` touches `/tmp/cuan-drain` and sends `SIGUSR1` to active `cuan-runner` workers. Active in-flight tasks finish uninterrupted while new attempts remain safely queued.
2. **Auto-Undrain & Service Restart Hook**: After the new release symlink switch and server health verification pass:
   - `activate-release.py` automatically unlinks `/tmp/cuan-drain` and triggers `systemctl restart cuan-runner`.
   - `scripts/deploy.sh` executes a post-deploy hook ensuring `/tmp/cuan-drain` removal and `cuan-runner.service` restart.
3. **Rollback Resilience**: In the event of health check failure or rollback, `/tmp/cuan-drain` is immediately cleared and the worker daemon restarted so queue processing is never left stranded in drain mode.

---

## 7. Detached Worker Process Architecture (Pola #1: Zero-Kill Deployment)

Starting with the Detached Worker Process architecture, task worker execution is decoupled from the `cuan-runner` supervisor daemon process tree:
1. **Isolated Execution Scope (`setsid` & `systemd-run --scope`)**: Task worker executions run in detached sessions independent of the supervisor daemon's systemd cgroup unit.
2. **Instant Pipeline Deploy (Zero-Wait)**: Pipeline deploy restarts the supervisor daemon (`systemctl restart cuan-runner`) in <1 second without waiting for in-flight tasks.
3. **Zero-Kill Execution**: Child workers continue running independently until task completion (LLM/tool execution, git commit, project verification, DoD evaluation, and database updates finish uninterrupted). Users never encounter interrupted or stopped states during deployments.
4. **Startup Reconciliation & Double-Claim Prevention**: When the new supervisor starts, it scans active worker records, verifies process liveness via PID, adopts in-flight attempts into its heartbeat reporting to renew leases, locks occupied lanes, and claims subsequent queued tasks immediately using the new release binary.

## Deployment evidence and task acceptance

Native activation does not complete task attempts, move tasks to Review, close
launches or fabricate a final agent report. After the running binary matches the
full release SHA, readiness matches the current schema and the served HTML matches
the retained payload, activation may record one `deployment_verified` activity
bound to the exact task and attempt supplied by the task capability/launch.
Repeating the same task/attempt/release reuses its receipt identity. A missing
attempt binding produces no receipt. A failed health check produces no receipt
and follows the normal rollback path.

The receipt is an external deployment fact, not acceptance of the task's other
criteria. Normal reporting, cancellation, recovery and coordinator review remain
responsible for that acceptance. A receipt storage failure does not roll back an
otherwise healthy release or complete a task; it logs that evidence is missing
and allows normal drain cleanup. Static-only activation does not attest a server
binary deployment. Historical task records are not rewritten.

Run `python3 scripts/test-activate-release.py` for activation safety. The focused
`scripts/test-deployment-receipt-postgres.py` integration check requires an already
migrated, disposable local `cuan_launch_queue_*` database selected through
`CUAN_LAUNCH_TEST_DATABASE_URL`, with the standard launch-contract fixture loaded.
It verifies exact task/attempt binding, retry idempotency and unchanged task,
attempt and launch records. Never point this fixture at production.

### Approved task scope at apply time

Apply/merge publishes repository changes, so an explicit task prohibition on push
also prevents manual and automatic apply. A deployment prohibition prevents
apply/merge and runtime deployment. The apply endpoint returns a JSON 403 with
the reason, leaving the task's status and delivery note unchanged. The internal
merge boundary re-reads persisted instructions and validates the project rather
than relying only on a caller's older snapshot. Runtime restart separately checks
the current task's deployment prohibition and project binding. These restrictions
can narrow project defaults; they never grant permission or verify a release.

Regression check: `go test ./internal/server -run 'Test(HandleApplyTask|ApplyTask)'`.

Apply and runtime deployment also read publication restrictions from the exact
latest attempt's launch revision, using the same policy as runner completion.
A newer unstarted launch does not replace the limits attached to the existing
result. If the attempt policy cannot be read, manual Apply returns503 and all
publication paths stop before a provider action. Initial task instructions still
apply when no execution attempt exists.
