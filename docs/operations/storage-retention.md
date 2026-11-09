## Production storage policy

The production VPS hosts several projects. On 2026-10-21 the40GB root filesystem
reached100%, preventing a release upload. The active application release was not
switched by that failed upload.

### Nginx logs

The overlapping `/etc/logrotate.d/dojo-nginx` rule matched both active `.log`
files and already rotated `.log.[0-9]*` files. This produced repeated `.1.1`
generations and retained7.3GB in `/var/log/nginx`.

The overlapping rule was backed up outside logrotate's configuration directory.
The standard `/etc/logrotate.d/nginx` rule now matches only active `*.log`, rotates
daily, retains seven generations, compresses old logs and checks `maxsize100M`
at each logrotate invocation. This is not a continuous100MB hard cap.
`logrotate -d /etc/logrotate.conf` and an actual Nginx rotation passed.

400 nested-generation archives were moved into a dated legacy directory below
`/var/log/nginx` and compressed:6,099,951,255 bytes became682,709,774 bytes. No log
contents were deleted. The legacy directory is a retained one-time archive;
ordinary rotation does not prune it. Active logs continue under the standard rule.

### Cuan releases

Use `python3 scripts/prune-release-artifacts.py` for a dry run or append `--apply`
for removal. The default installation is `/opt/cuan`; `--base` supports an isolated
installation. Keep five newest valid payloads and remove only candidates older
than24hours. Preserve current/previous symlinks and any payload referenced by
running processes. Unknown directories and invalid manifests are left alone.
The pruner shares `.release.lock` with activation/rollback and fails closed if
rollback references are missing. Application data, workspaces and backups are
outside its scope.

Production runs the installed script at
`/opt/cuan/maintenance/prune-release-artifacts.py`, daily through
`/etc/cron.d/cuan-release-retention`. Its log is overwritten per run at
`/var/log/cuan-release-retention.log`. Existing application backups were retained.
Fourteen redundant upload copies were additionally removed only after verifying
the complete installed counterpart against its manifest, reclaiming525,444,752
bytes; installed releases were retained during that upload-copy cleanup.

### Capacity decision

After log correction and release retention, production has8.6GB free (78% used),
versus458MB before the full cleanup. Nginx and Cuan remain active. Dracin has the
same40GB disk and about5.6GB free, with less available memory at observation time.
Moving Cuan there is not a storage improvement. Fix retention first; consider a
larger volume or dedicated server when real persistent data, rather than retained
artifacts, requires it. Backup retention needs its own restore-tested policy;
this cleanup intentionally did not delete existing backups.

### Task terminal diagnostics

Task attempt output is a rolling diagnostic buffer: at most 1,000 recent complete
lines and 256 KiB of line text per attempt. Older diagnostic lines roll off as
new output arrives, so a late provider/quota error remains available after reload.
This is not a full terminal transcript. Task instructions, approvals, reports,
artifacts and conversation history are separate and are not pruned by this buffer.
Secrets are masked before storage; oversized terminal lines are omitted rather
than storing a partial credential. No schema migration or historical-log cleanup
is required; the window advances when an attempt writes new output.

Targeted checks: `go test ./internal/task ./internal/server -run
'TestRecentAttemptLogRetention|TestTaskOutputRetainsLateRedactedDiagnostics|TestInboxLifecycleNegativeCasesAndRetainedTaskOutput'
-count=1 -timeout=30s`. Memory and SQL tests cover late errors, line/byte bounds,
incremental cursors, oversized lines and isolation between attempts.

### November 4: upload staging retention

The 50 GB production disk filled again during frequent deployments. Inspection
found 64 valid installed releases and 64 staged copies younger than 24 hours;
the daily installed-release policy was running but did not cover upload staging.
Old staging copies were removed, and the official installed-payload pruner
removed six eligible releases. The extended pruner recovered further space,
leaving about 4 GB free at observation time. Application/task data and backups
were preserved; this does not establish the cause of provider exit code 2.

`--staging /home/ubuntu/.local/share/cuan-releases` removes redundant upload
copies older than one hour, preserving five newest and current/previous IDs.
It requires an intact matching installed manifest and verifies SHA-256 checksums
for the complete installed payload and every remaining staged file. Partial
uploads are eligible only when the installed copy is complete. Unknown files,
symlinks, corrupt copies and missing counterparts are retained. The activation
lock protects pruning; staging and installation roots must not overlap.

Production cron now runs at minute 17 every hour with `--staging`. The deploy
script installs the maintained pruner and runs it before transferring payloads.
Installed releases still retain the original five-newest/24-hour protection.
Run `python3 scripts/test-prune-release-artifacts.py` for targeted checks.

### Task admission at critically low space

Before reserving a bound task attempt, the run-agent endpoint checks available
bytes on the selected workspace filesystem. Below 256 MiB it returns a retryable
`execution_storage` response. Durable launches stay queued without exhausting
their ordinary preparation-retry limit and can proceed after space returns.
Unknown/unsupported disk telemetry is distinct from a measured zero-byte result.
High percentage usage alone does not block a filesystem with sufficient bytes.

This is a minimum admission floor, not a guarantee that a large build will fit.
It does not stop existing attempts or replace runner-host monitoring. Historical
exit-code-2 attempts have no final agent report, but their retained terminal logs
do not prove the exact provider exit cause; disk pressure remains a hypothesis
for those failures, separate from the verified full-disk deployment incident.

### Bound release compilation peak

Native cache maintenance scans both independent-verifier generations:
`data/projects/verification-snapshots` and `data/workspaces/verification`.
Both use the same lease/process-reader, source identity, ignored dependency and
lockfile checks. The older root is not deleted wholesale. A2026-11-09 production
preview found zero eligible dependencies there; adding coverage does not claim
source-checkout reclamation. Retained Git/source copies still need a separate
reconstructible-source lifecycle while preserving reports and unique work.

Backend release binaries compile sequentially (server, migrate, runner). Each
`go build` shares completed cache entries with the next; Cuan no longer creates
three simultaneous temporary dependency graphs on the production filesystem.
A component failure stops before starting the remaining components or publishing
artifacts. The existing exact-source, test and activation gates remain required.

On2026-11-09 native release dca6eee1 failed with `no space left on device` while
three Go builds ran concurrently. The host had1.5GiB free after failure; observed
sizes included developer workspaces13GiB, verifier snapshots7.2GiB, provider
runtime7.2GiB and PostgreSQL231MiB. An idle host Go cache was cleaned through
`go clean -cache`, recovering approximately1.3GiB. This is immediate headroom,
not proof that retention/capacity is sufficient indefinitely. Preserve task
sources and evidence; continue measuring growth and validating native delivery.

Native runner publication now prepares compiler/download caches through the same
owner-scoped Go/npm cache helpers as ordinary workers. It holds their shared
maintenance leases for execution and completion. Release HOME, npm configuration,
project values and temporary workspace remain private to the prepared checkout;
only regenerable cache locations change. Other owners do not share these paths.
The actual selected environment participates in the existing native process
journal binding before a script can start. This does not authorize replay of an
unknown prior process or make old failed attempts successful.

Verification: `go test ./internal/runner -run
'TestNativeRelease|TestManagedGoBuild' -count=1`. Regression checks stable owner
cache selection, preserved preparation/configuration, distinct owners, and real
exclusive maintenance-lock rejection while the native shared lease is active.

### Legacy provider module migration cleanup

For profiles migrated to owner-managed build caches, the existing provider cache
maintenance tool now accepts explicit `--all-modules`. It uses official
`go clean -modcache` only for old, idle legacy profile module directories while
holding the provider account's exclusive execution lease. Active profiles and
paths referenced by processes are retained. Default maintenance behavior remains
limited to its previous toolchain/archive policy; full legacy module cleanup is
an explicit migration operation, not recurring deletion of warm owner caches.

`python3 scripts/test-prune-provider-go-cache.py` passes ten tests, including a
real offline Go module-cache cleanup and preservation of account credentials,
account lock inode, busy accounts and process-referenced caches.

On9November the reviewed script was transferred to a standalone temporary
maintenance path and its SHA-256 matched the local source
`0a52090d193fe19a56715c294f21f9423ea9c002d34aaab2bc64e71f0d4a8134`.
Preview identified six eligible legacy module caches. Explicit apply removed all
six, increasing observed free space from1.3GB to3.3GB during the native build.
Source/workspaces, credentials, provider histories and active native caches were
not deleted. This is an infrastructure intervention in the native benchmark,
not evidence that Cuan performed storage recovery autonomously.

### Reuse application schedules and completion events

Native installations register the existing scheduler's `storage_maintenance` job
with a15-minute interval. Attempt-completion events (including failed attempts)
and HTTP completion acknowledgement request the same job; blocked admission at
the existing storage floor also requests it. Requests advance its next run by up
to10seconds and coalesce, without starting separate cleanup goroutines. The fixed
installed oneshot remains the single worker and retains its OS timer as a fallback.
Resource leases and installed pinned-tool protections are unchanged.

The native installer validates an exact sudo rule allowing the host service user
to start only `cuan-runtime-cache-maintenance.service`. No caller-supplied command,
path or shell is involved. Hosts without that native unit register the job disabled;
this is an optional native-host maintenance bridge, not a vendor or platform
requirement for Cuan. Local deployment resolves the real service user's home from
the registered SSH user instead of passing the isolated native build HOME as the
legacy account directory. Secrets remain outside scheduler job arguments/results.

Targeted checks cover completion-event scheduling, burst coalescing, periodic
recovery, the existing storage floor, exact worker permission and failed rule
validation preserving the previous rule. Application lifecycle pruning and cache
cleanup remain distinct from task acceptance or permission to rerun a release.

Native releasebae44cd33f61f6ca12bfcaa2466adfa88e633540 passed exact-SHA production
health with37 migrations. The new installed job was observed enabled/idle through
`cuan maintenance status`, requested through the official CLI, observed running,
and then success with`last_run_at=2026-11-09T10:06:44.699991726+08:00`.
Official MCP returned the same actual job state. This proves the scheduler-to-worker
production path. Event coalescing/completion wiring is covered by isolated tests;
it is not claimed as a fresh native production completion-event demonstration.

The diagnostic tooling had supplied a static always-active scheduler list. It now
reads the authenticated registered job API and propagates unavailable state as an
error. The maintenance CLI acknowledges only the fixed server job request and
checks enabled/existing state before requesting; it does not claim scan completion
or arbitrary byte savings. Client tests cover actual failed status, API failures,
disabled/missing jobs and an unconfirmed trigger receipt without automatic replay.
