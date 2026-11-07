# Native release execution

The release executor runs the repository's maintained release entry point against
an exact clean commit and expected branch. It uses no language model and does not
merge source, accept tasks or infer production health from a zero exit code.
Inbox delivery admission and durable stage orchestration remain separate work;
this command is currently an explicit maintainer operation.

## Durable stage contract

The task layer now recognizes a typed native-release launch backed by an exact
runner-authored source handoff. A model report or copied launch marker cannot
supply that evidence. The handoff binds commit, instructions, current passing
project checks, repository, target branch and release-descriptor hash. It leaves
publication unfinished. Failed/canceled/superseded sources, changed evidence and
publication prohibitions are rejected.

Admission reuses the existing durable task launch and predecessor reservation.
Original owner approval is checked before changing task state, and checked again
when the launch executes. Reconstructed dispatchers reuse the recorded attempt.
This reservation bypasses AI account selection. Memory and PostgreSQL stores keep
native stages out of older runners; a capable claimant revalidates source before
execution. A revoked source fails before any release process starts.

The source handoff, native worker and preparation transport are implemented but
staged. The current poller does not advertise native-release support, automatic
native-stage scheduling is not connected, and delivered-source independent
verification is unfinished. Do not enable this as an operational automatic
production release workflow. `cuan task execution` identifies saved intent without
exposing approval or credentials; parsing is not proof of release success.

## Repository contract

Track `.cuan/release.json` alongside its executable entry point:

```json
{
  "version": 1,
  "command": ["scripts/deploy.sh"],
  "timeout_seconds": 1200
}
```

The descriptor is limited to16KiB,16 literal command arguments and a timeout of
1–1800seconds. Unknown fields/versions and trailing JSON are rejected. The command
must be a tracked regular executable inside the repository. Symlink traversal,
absolute paths and parent escapes are rejected. Arguments are passed directly;
shell substitutions and environment interpolation are not performed by Cuan.
The maintained script still owns its release, migration, health and rollback
gates. This executor does not replace or disable them.

Planning and execution verify the commit, branch and clean source independently.
The plan cannot be changed by callers after loading. Execution holds the existing
repository lease and checks that tracked source remains unchanged afterward.
Pending work is never reset, cleaned or committed to make a release pass.

## Maintainer CLI

```sh
cuan release inspect --path . --commit FULL_SHA --branch dev
cuan release run --path . --commit FULL_SHA --branch dev --logs
```

`inspect` does not run the script. `run` explicitly uses the maintainer process
environment, like invoking that script directly. Do not use this operator path
as a Cuan user's credentials or as an Inbox delivery grant. Bound task sessions
cannot run this maintainer command. The native Go executor instead requires an
explicit environment slice; nil is rejected and never inherits host credentials.
A future Inbox delivery stage must resolve the saved owner/project environment
and original authorization before invoking it.

Results report source, start/end time, real exit code and unchanged-source status.
Process startup failure uses an unknown exit code, not zero. Raw child output is
absent from JSON. `--logs` prints only the bounded diagnostic tail with known
credential patterns and sensitive operator environment values redacted. It is a
diagnostic, not an interactive terminal or a promise of complete historical logs.

Cancellation kills the owned process tree. An inherited pipe cannot keep the
executor waiting indefinitely after the script exits, and remaining descendants
in its process group are cleaned up. Output retention is bounded to12,000bytes.
No failed/uncertain execution is retried automatically. Cancellation or failure
does not prove that external effects never happened: inspect current release state
before another attempt.

Only a trusted task/attempt/source-bound deployment observation can establish
publication, and an independent review must still check the original criteria.
A successful operator invocation does not write those receipts or mark Inbox work
Done by itself. See [deployment evidence](task-deployment-evidence.md).

## Managed developer handoff

Admission requires frozen `CUAN_MANAGED_RELEASE=1`, auto-deploy, native-capable
runner and ordinary Inbox code scope. Read-only, standalone-file, verifier,
inspection and publication-restricted work cannot acquire this mode. The backend
pins task, project, room, attempt, runner, instructions hash, repo and target
branch in a canonical claim. Settings enabled later never grant old attempts
retroactive publication ownership.

The developer owns implementation, local checks and a clean task checkpoint.
Native coordination owns push/merge/deploy. The worker observes the exact commit,
actual task branch, tracked descriptor hash and native passing-check hash; it
skips generic post-execution PR/publication. Completion requires a saved final
implementation report, matching source/checks, meaningful diff, zero exit and no
claimed deployment. It atomically records `native_release_handoff` and leaves the
task in Review with original criteria untouched. Ordinary execution retains its
existing exact-deployment gate.

## Publication and private preparation

`ApplyNativeReleaseSource` uses the frozen repo/branch and saved connection with
read-first immutable ancestry reconciliation. Its caller must validate original
approval and canonical source first. It performs no project-cache checkout.

`NativeReleaseWorkspace` and `git.PrepareReleaseWorkspace` create a separate
checkout binding dev source, prepared target and release branch. Source-to-target
ancestry must hold. Missing objects use the explicit saved connection to fetch
exact SHAs into the frozen cache without ref or working-tree changes. Existing
private workspaces are validated rather than reset. Descriptor hash is checked
through `release.Load` before any script runs.

Internal `prepareNativeReleaseWorkspace` reauthorizes original Inbox approval,
serializes work with the frozen repository lease, prepares the checkout, rereads
scope/project/source, and persists `native_release_preparation`. Identical recovery
reuses that record; a different target cannot replace it. A new target must come
from a reconciled native merge receipt, never an HTTP input.

## Worker preparation transport and execution

Authenticated `POST /api/v1/runner/attempts/{id}/release/prepare` accepts only
`task_id` and `runner_id`. It derives target from canonical preparation, validates
authorization, recreates/rechecks the exact private checkout and returns typed
workspace plus explicit frozen project environment. Response is `no-store`.
When preparation is absent, it derives the original developer checkout from
server configuration and task identity, requires a separate exact clean source,
locks that checkout, pushes the immutable approved SHA through saved access,
reauthorizes original source, and applies the native merge helper to the frozen
release branch. Only the reconciled merge result supplies the prepared target.
An existing canonical preparation bypasses further push/merge writes.

HOME, XDG directories, Go/npm caches and temporary files use private paths under
that checkout's Git metadata. Selected project values are not replaced by ambient
host credentials. The worker runs no model, MCP or account/profile setup. It
revalidates intent/source/target/branch/descriptor and invokes the native executor
with explicit environment. Nil environment never inherits the host environment.
Owner SSH trust, dependency preparation and actual production compatibility must
still be established before activation; local fixtures do not prove them.

## Process journal and completion

Before starting the script the worker syncs a private per-attempt journal, then
syncs actual result facts afterward. Only the selected environment digest is
stored, not credential values. A reconstructed worker reuses completed result
facts and re-observes production without running the script again. An incomplete
start marker or changed preparation/environment prevents replay. A duplicate
worker encountering an active process lock does not complete the active attempt.

After script success the worker requests registered production observation and
sends structured `native_release_process` through the durable completion outbox.
Backend completion requires valid preparation, exact target/branch, zero exit,
unchanged source and sane times, plus trusted target health/revision observed
AFTER process finish. Model report, PR or code-mutation fields cannot substitute.
It atomically records `native_release_delivery`, preserving the dev SHA separately
from release target, and keeps the task in Review for independent verification.

The publication observer validates frozen repo/branch/settings and active source
before and after observing health/revision. Late project archive or identity
changes reject new evidence. Neither process exit, preparation nor delivery alone
marks the original task Done.

## Immutable source push prerequisite

`PushNativeReleaseSource` validates frozen managed publication settings and the
registered saved connection, and prohibits using the deployment branch as the
developer task branch. `git.PushExecutionSource` checks exact clean source,
repository origin and task branch, observes that remote branch first, then pushes
the admitted SHA as a literal non-forced refspec. A later local HEAD cannot replace
that SHA. An already matching remote branch needs no new push; unavailable remote
observation prevents mutation. The initial preparation endpoint invokes these helpers. Native-stage scheduling,
runtime production readiness and delivered-source independent verification remain
staged; the poller capability is still disabled.

Concurrent initial preparation requests serialize on the original developer
checkout, reread native source and approval after acquiring ownership, and reuse a
preparation produced by the winning request. Source and project-cache checkouts
must be distinct. Missing access, dirty/replaced source and revoked authorization
stop before merge. A failed/unknown remote outcome is not called success; another
active request must re-observe exact push/merge state before further mutations.
