# Storage pressure and execution recovery

Cuan must pause new executions before disk exhaustion rather than spending the
user's code-correction budget on an infrastructure failure.

## Admission and retention

New terminal/task execution waits through the existing `execution_storage`
admission path when measured filesystem space available to the service user is
below 2 GiB. This excludes filesystem reserves. It is an admission floor, not a
promise that any build fits or a reservation for concurrent writes. Existing
executions are not killed by the admission check.

The native deployment path and hourly maintenance use
`scripts/prune-release-artifacts.py`. Retention keeps five newest releases,
current and previous rollback targets, releases referenced by live process
executables or working directories, and all payloads younger than one hour.
Previously the minimum age was 24 hours, allowing frequent deployments to retain
many gigabytes within a single day. The activation lock still serializes cleanup
with switching and rollback. Unknown directories remain protected. Staged uploads
require byte-verified installed counterparts before cleanup. Application data,
provider profiles, workspaces and database backups are outside this pruner.

### Scheduled disk guard

The legacy `cuan-disk-guard.sh` used broad deletion of Go build caches, staging
binaries, temporary files and provider conversations, without execution locks.
Production had both a root-crontab entry and a separate hourly copy. Updating
only the deployment pruners therefore left unsafe cleanup scheduled.

The guard now delegates only to the release pruner and idle-provider Go
toolchain pruner. It never truncates third-party logs, clears warm build caches,
deletes provider conversations, or removes arbitrary temporary directories.
Both installed guard copies are replaced during native deployment. Existing
schedules remain; no new cron job is installed. The release activation lock and
provider execution gates protect simultaneous deployment and active accounts.

The guard accepts `--base INSTALL_DIR` and `--home SERVICE_HOME`. By default it
uses `/opt/cuan` and resolves the non-root owner of the native `releases`
directory through the local user database. Root-owned installations must supply
the service home explicitly to enable provider/staging cleanup. Missing install
paths or rollback evidence stop cleanup; they do not trigger broader deletion.
Failure to reclaim enough space leaves ordinary storage admission in place.

The deployment gate exercises the actual scheduled entry point with a live
provider lock, preserving credentials, conversation history, warm cache objects,
unknown staging files and pending workspace checkpoints. Missing rollback state
is also checked before any provider cache deletion.

## Provider failure classification

Each PTY provider execution receives an official `--log-file` path inside a new
private temporary directory. Only the bounded final 128 KiB are inspected after a
failed exit. Recognized provider warning/error records must identify a local
`.gemini/antigravity-cli/` path under that exact leased profile and end with
`no space left on device`. Ordinary quoted text, another profile, generic exit
codes and unrelated paths cannot establish this cause. Raw provider logs are not
published; the temporary directory is removed when execution returns.

A matching failure becomes an interrupted infrastructure execution with preserved
files. Normal approval, publication, cancellation, workspace and storage admission
guards still control continuation. Infrastructure failures do not spend the
business correction budget, but automatic interruption recovery stops at five
recorded infrastructure attempts. Existing historical failures are not rewritten
or automatically reclassified by this change.

## Evidence and limits

The 2026-11-04 preview attempt `att_c262e67d48cdf2f3010867cac68bbed3`
started at 18:47:49 +08. Its provider log identified the exact task workspace and
reported repeated local write/mkdir failures due to exhausted disk from 18:49:38
through 18:49:46, immediately before its exit 2 at 18:49:47. This establishes a
local storage failure during that execution. It does not prove the task's code
was correct or justify accepting its results.

The native React attempt had a provider 503 before timeout. That alone does not
establish the cause of its entire stalled interval. Neither historical task is
manually resumed by this patch. Provider profiles, backups, application data and
other host logs still need their own appropriate retention policies; the release
pruner is not a complete host disk manager.

Verification covers rapid same-day deployment retention with a live process,
rollback protection, admission boundary values, bounded profile-specific provider
log classification, a real PTY fixture writing a storage failure, quota and
ordinary failure preservation, and the independent interruption recovery limit.

## Reconciling historical opaque exits

For an owner-authorized blocked task whose latest attempt failed with exit 2,
Cuan can inspect that owner's provider profiles for a matching historical CLI
process. Eligibility requires recorded start/end times within the last seven
days. Candidate filenames must match the local execution start window (two
seconds before to thirty seconds after start); at most twenty matching files per
profile are inspected. The native workspace header must name the exact task
workspace twice as passed to AGY, and the final file modification must fall
within the attempt through five seconds after completion. Only the existing
bounded local-profile ENOSPC detector can establish the cause.

Cuan saves one `execution_diagnosis` activity per attempt, including a negative
result to avoid repeated disk scans. A positive receipt binds the original
instructions hash, start/end times, attempt/task IDs, platform author and evidence
SHA-256. Raw provider logs, profile paths and credentials are not published. The
original failed attempt and exit code remain unchanged.

A valid diagnosis removes that infrastructure failure from the code correction
count and includes it in the separate infrastructure count. It does not reset
any budget or invent a successful result. Existing commits remain saved work,
not grounds to discard it or to claim deployment. Cuan may queue one idempotent
continuation of the same task only while the original approval remains valid,
code correction count is below three, infrastructure count is below five, and
normal cancellation, project concurrency, storage and publication guards permit
it. The continuation must inspect saved files and reconcile uncertain effects.
Unmatched logs, another owner's profile, changed instructions, invalid receipts
or cancellation cannot authorize recovery. Other opaque exit codes and timeouts
are not automatically reclassified by this mechanism.

Recovery compares the saved approval's agent identity through the same registered
runtime alias resolution as normal execution. A historical `agent_sm` approval
therefore remains valid for the canonical `agent_cuan` identity after migration.
Missing canonical agents, unknown identities and unrelated agents still fail the
identity guard; owner/message/room/publication checks are unchanged.
