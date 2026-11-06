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

The guard delegates to the release pruner, idle-provider Go toolchain pruner,
leased verification dependency pruner, and pressure-triggered managed Go build cache
maintenance. It never truncates third-party logs, deletes provider conversations,
or removes arbitrary temporary directories. Warm build caches remain untouched
when available space is at least 3 GiB.
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

### Verification dependency caches

Verification snapshots remain saved repositories, not disposable workspaces.
The runner holds an exclusive `.git/cuan-verification.lock` for the entire
inspection, including dependency preparation, native capture, model execution,
verification and reporting. Maintenance uses that same persistent inode and
skips busy snapshots. Waiting for the lease uses the original task deadline.
Read-only inspections do not write workspace MCP configuration; provider-side
configuration and the scoped reporting CLI remain available.

`scripts/prune-verification-dependencies.py` previews eligible caches:

```sh
python3 scripts/prune-verification-dependencies.py \
  --snapshots /opt/cuan/data/projects/verification-snapshots
```

Add `--apply` to remove eligible `node_modules` directories. Hourly maintenance
uses this mode. Eligibility requires a recognized snapshot binding, matching
HEAD, clean tracked and untracked source, tracked npm manifests and a modern
lockfile compatible with credential-free dependency preparation. The dependency
directory must be Git-ignored, contain no tracked files, and be at least one
hour old. Linked roots, metadata, gate files and dependency directories, nested
mounts, unknown/unreadable process references and missing legacy leases are kept.
Process executable/cwd, command arguments and open descriptors additionally
protect readers that do not use runner leases.
Root maintenance runs Git checks and dependency removal as the snapshot owner, with optional index
writes, hooks and filesystem monitoring disabled. Repository-defined Git filters
therefore cannot inherit the maintenance process's root identity.

Only dependency trees are removed. Source, `.git`, lease inodes, snapshots,
reports, attachments, provider state and warm build caches remain. Dependency
preparation recreates locked packages before the next applicable visual check.
The pruner cannot reclaim older uncoordinated snapshots or arbitrary workspaces;
insufficient free space still keeps task admission queued. Tests exercise real
Git, OS locks, an independent process reader, safe removal, linked-file targets,
unknown readers, mounts and changed or unsupported repositories.

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

### Idle profiles that predate execution gates

A missing `.execution.lock` is not evidence of an active session. Maintenance now
runs under the service account owning the provider directory. That owner may
create the same mode-0600 gate used by runtime leasing and acquire it exclusively
before pruning. A different user cannot create a missing gate; symlinks,
nonregular gates and active leases remain protected. Scheduled root maintenance
drops to the directory owner's UID for this operation, and deployment uses its
existing service identity, preventing root-owned gate files from breaking login
or execution.

Only recognized Go distributions older than the existing retention window qualify,
and live executable/cwd references still prevent deletion. Their owning user can
remove read-only distribution directories without elevated privileges. Provider
credentials, conversation history, other modules and the warm owner build cache
remain outside cleanup. Preview mode may create coordination gates, but does not
remove distributions; `--apply` performs deletion.

The same policy also covers their retained Go download ZIPs. These live only in
the private `go/pkg/mod/cache/download/golang.org/toolchain/@v` directory and must
have a recognized version/platform filename, a regular Go checksum marker and
an existing download lock. Maintenance holds both the account execution gate
and the Go download lock, observes the normal24-hour age window, and protects
open file descriptors as well as executables/cwd. It removes only the ZIP,
retaining checksum and lock inodes. Unknown/private modules, partial downloads,
linked files, current downloads and warm compiled build caches remain untouched.


## Managed Go build cache pressure

The owner-scoped `data/go-build-runtime/<owner-sha256>/go-build` cache can grow
beyond the available capacity of a small VPS. Hourly maintenance now measures
available bytes on that filesystem. Below 3 GiB it considers only recognized
managed build caches, stopping when 4 GiB is available. The 2 GiB execution
admission floor stays unchanged. This is headroom management, not proof that an
arbitrarily large build fits.

The runner holds a shared `.build-cache.lock` for its complete Go execution,
including native verification and reporting. Maintenance must acquire that same
persistent inode exclusively and also checks live process environments, command
arguments, executable/cwd and descriptors for older readers. Unknown readers,
missing legacy gates, linked paths, foreign-owned gates, non-Go directories and
separate filesystems are skipped. A task waiting for maintenance retains its
original execution deadline. Parallel executions can hold shared leases.

Eligible idle caches are cleared through `/usr/bin/go clean -cache`, running as
the directory owner with a minimal environment, GOENV disabled and automatic
toolchain downloads disabled, outside any repository. Maintenance does not
execute repository code or import account credentials. Only compiled build
artifacts are affected; module sources, toolchains, workspace checkpoints,
reports, authentication and database backups remain saved. The next build may
be slower while recompiling. Healthy disks keep their warm caches.

Preview without deleting:

```sh
python3 scripts/prune-runtime-go-cache.py --runtime /opt/cuan/data/go-build-runtime
```

Add `--apply` for maintenance. The script is included in both native deployment
paths and the existing scheduled guard. Default host Go caches are excluded
because they have no Cuan execution lease. The policy is intentionally unable
to free disk by deleting uncoordinated application data; insufficient reclamation
leaves task admission queued rather than weakening safety.
