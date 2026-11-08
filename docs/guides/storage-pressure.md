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
python3 scripts/prune-runtime-build-cache.py --runtime /opt/cuan/data/go-build-runtime
```

Add `--apply` for maintenance. The script is included in both native deployment
paths and the existing scheduled guard. Default host Go caches are excluded
because they have no Cuan execution lease. The policy is intentionally unable
to free disk by deleting uncoordinated application data; insufficient reclamation
leaves task admission queued rather than weakening safety.

## Release ancestry must fail closed

Native activation must prove that a new release contains the active release's
commit. Missing host-cache objects previously skipped this check, allowing a
stale task workspace to replace a newer production revision. New release payloads
include `source-commits.pack`, a checksum-covered Git pack containing actual commit
objects and their parent hashes, without source trees or blobs. Activation imports
it into a temporary bare repository with host Git configuration and replace refs
disabled, and proves ancestry there. It does not fetch credentials or depend on a
warm server repository cache. The temporary graph is disposed afterward.

An older payload may use the registered native cache only when both exact objects
and their ancestry can be verified. Missing metadata/objects, malformed packs,
grafts, timeouts and divergent history block activation before switching the
running release. The normal rollback path remains explicit. `--allow-divergent`
is an operator recovery action; task-capability executions cannot use it. It is
not an automatic response to a failed check. Keep any rejected task commits for
review and proper integration rather than weakening deployment checks.

This is an integrity gate in the maintained native deployment path, not an OS
sandbox against a process with unrestricted sudo or direct installation write
access. Such privilege separation remains a separate deployment concern.

## Upload copies and installed release retention

Installed releases retain the five newest payloads, active/previous rollback
targets and live process references. Upload staging is a redundant transfer
cache, not a second rollback catalogue. Once an upload is at least one hour old,
contains no unknown files or links, and every available byte matches a complete
installed payload and its manifest checksums, its redundant copy may be removed.
The installed current and rollback copies remain protected independently.

Cleanup verifies and removes upload duplicates before retiring their installed
counterparts under the same activation lock. A process executing from, or using,
an upload as its working directory protects both that upload and its installed
counterpart until it exits. Recheck eligibility immediately before deletion.
This avoids the previous orphan condition where the five newest uploads were
kept after their installed counterparts were deleted, making subsequent
byte-for-byte verification impossible. Unknown or already-orphaned uploads are
preserved for independent reconciliation; this policy does not guess that they
are safe to delete.

### Owner-scoped npm caches

Native executions use stable owner-scoped npm caches alongside managed Go caches
under `npm-build-runtime/<owner hash>/npm-cache`. This avoids duplicating package
cache contents across AI accounts. An explicit project `NPM_CONFIG_CACHE` (or
lowercase equivalent) remains authoritative. Registry and authentication settings
are preserved; changing cache location does not connect an npm account.

The execution holds the shared build-cache lease for its entire lifetime.
`scripts/prune-runtime-build-cache.py --kind npm` previews eligible caches;
`--apply` invokes the official npm cache cleaner as the cache owner under isolated
maintenance configuration. Scheduled maintenance acts below3GiB free and aims
for4GiB. Busy caches, recent files, symlinks, unknown owners and process readers
remain protected. Optional legacy-account cleanup uses the provider execution
gate and only recognizes canonical npm content/index caches. Provider sign-in,
conversation history, repository source and node_modules are outside this path.
Insufficient reclamation leaves storage admission queued rather than broadening
deletion. Native ENOSPC is an infrastructure interruption, not a code defect.

### Cache recovery between executions

Native deployment also installs `cuan-runtime-cache-maintenance.timer`. It checks
idle Go/npm caches every two minutes (15-second timer accuracy), including caches
which were busy during the hourly guard. It never starts or accepts a task. Tasks
continue through normal storage admission after actual available space recovers.
Healthy disks skip cache cleaning through the same3GiB/4GiB pressure policy.

The oneshot service uses content-pinned, root-owned cache tools under
`maintenance/runtime-cache-tools/`, independent of `/usr/local/bin` and the hourly
guard. An older task deployment overwriting those legacy paths therefore cannot
replace this timer's tools. All provider/cache leases and process-reader checks
still apply. This is maintenance integrity, not an OS sandbox for workers with
unrestricted sudo. The timer has no repository, project credentials or build scripts
as inputs. Check its status with `systemctl status cuan-runtime-cache-maintenance.timer`
and logs with `journalctl -u cuan-runtime-cache-maintenance.service`. Failure to
reclaim space does not authorize broader deletion or bypass admission.

### Recent verification dependencies during critical admission pressure

The pinned two-minute cache timer also checks verification dependencies with
`--under-storage-pressure`. The normal one-hour warm-cache age remains in force
unless available filesystem space is below the same2GiB execution admission floor.
Only then may it reclaim recent ignored node_modules from a recognized, unchanged
snapshot with a free existing verification lease and no observed/unknown process
reader. Missing leases, unknown files, tracked dependencies, changed source,
unsupported lockfiles, symlinks and mounts remain protected. It never removes the
snapshot, Git objects, reports or delivered evidence. Capacity measurement failure
keeps normal retention. No admission threshold or acceptance check is bypassed.

### Host Nginx log rotation

A host audit found about400MiB in inactive, rotated but uncompressed Nginx logs.
The retained `delaycompress` setting kept yesterday's large files uncompressed,
and the daily systemd timer checked `maxsize 100M` only once per day. Host
maintenance now uses `nodelaycompress`, an hourly logrotate timer override, and
`maxage 7`/`rotate 168` so size-triggered rotation can retain up to seven days.
Existing daily rotation, Nginx reopen hook and100M threshold remain.

Initial compression used the official gzip tool only on regular rotated files
with no open readers/writers, with decompressed SHA256 equality checked afterward.
No log contents were discarded. Native logrotate configuration was validated in
debug mode before reloading the timer. This is host configuration, not a new Cuan
cleanup rule for arbitrary third-party logs. A private original config and
compression receipt remain under `/var/backups/cuan-maintenance/` on the host.
To roll back scheduling, restore the saved Nginx stanza, remove only
`/etc/systemd/system/logrotate.timer.d/90-cuan-storage-pressure.conf`, reload
systemd and restart the logrotate timer. Compressed logs remain readable with
standard gzip tools; no archive restoration is needed to inspect their contents.

### Native release workspace caches

Release workspaces also contain isolated Go module/build and npm download caches
under `.git/cuan-native-runtime`. The two-minute maintenance timer now covers
these workspaces as well as verifier snapshots. Previously those private caches
were outside the managed runtime cache roots, so each release retained another
copy and could eventually block admission even though routine cleanup succeeded.

`scripts/prune-native-release-caches.py --workspaces /opt/cuan/data/projects/release-workspaces --apply`
starts below 4 GiB available and stops at 8 GiB. `--force` is an explicit operator
option for reclaiming all eligible idle caches. It validates the native snapshot
binding and source HEAD, takes the workspace verification lease, checks process
references and rejects linked cache ancestors. Official Go/npm cache commands run
as the workspace owner with isolated configuration. Source, Git history, runtime
configuration, reports, attachments and live executions are retained. Ignored
node_modules use the existing dependency pruner with the same workspace lease.

On 2026-11-08, production recovery increased available space from about 0.7 GiB
to 8.6 GiB by clearing idle release dependencies and isolated caches. The queued
native inspection then resumed automatically. Other hosted applications were
also inspected: inactive rotated Dracin logs were compressed with contents
preserved; the existing Laravel expired-file-cache pruner runs independently
with file locking and expiry checks, preserving sessions and application data.

### Locked PHP archives

PHP preparation retains public Composer archives in
`data/php-build-runtime/<owner-sha256>/composer-cache/files`. Each snapshot still
installs its own ignored `vendor/`; provider profiles, auth, source and vendor
are never borrowed. Repository/VCS caches and Composer configuration use a
removed temporary HOME. Scripts/plugins stay disabled and custom cache paths
require project-specific preparation. Composer's archive limit is 256 MiB per
owner, with the existing pressure cleaner taking the same exclusive cache lease
before invoking auth-free `composer clear-cache`. Active installs hold a shared
lease. Neither maintenance nor install failure deletes the owner's source.

Setup allows at most four minutes, or half the remaining execution budget,
whichever is smaller. Timeout stops the installer process group, retains already
downloaded archives, and records an explicit setup diagnosis. This budget does
not replace verification or turn setup into passing checks.

Readiness evidence travels with the runner's persisted heartbeat, so API and
external chat-worker processes see the same installed preparation policy. An
additive `runners.runtime_evidence` JSONB column defaults to an empty object;
previous binaries can run against the migrated schema. Roll back application
code without dropping the column; dropping it requires stopping all binaries
that read it. Omitted, expired, unknown or future-dated capabilities never grant
recovery or claim eligibility.
