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

This contract is staged: the current poller does not advertise native-release
support, and ordinary source completion does not emit managed-release handoffs
yet. Automatic production delivery therefore remains disabled until the native
worker and handoff producer are connected. Existing deployment acceptance gates
remain unchanged. `cuan task execution` identifies the saved native intent without
exposing its approval or credentials; parsing it is not proof of release success.

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

### Staged managed-release claim binding

The claim service can record native publication ownership only when the frozen
project environment contains exactly `CUAN_MANAGED_RELEASE=1`, auto-deploy is on,
and a native-release-capable runner claims ordinary code work from Inbox. The
native poller does not advertise that capability yet. Do not enable the setting
as a completed automatic release workflow.

The server records the exact task, project, Inbox room, attempt, runner,
instructions hash, repository and target branch as a `managed_release_claim`
activity. No environment values are copied into that receipt. Read-only,
verification, inspection, standalone-file and publication-restricted work cannot
acquire it. Completion revalidates canonical admission against its original
identity and frozen project context. Absent admissions never become retroactive
grants.

Managed developer sessions own implementation, local checks and a clean task
checkpoint. Their prompt leaves push, PR, merge and deployment to native
coordination, and the generic post-execution publication flow is skipped. The
runner observes the exact clean checkpoint, actual task branch, tracked release
descriptor hash and passing native check hash through the release plan loader;
it never executes the release script during observation.

A successful managed source completion additionally requires a saved final
implementation report, native check/source consistency, meaningful branch diff,
zero exit, no PR claim and no claimed live deployment. It atomically persists
`native_release_handoff` with completion and verification records, and keeps the
task in Review with original acceptance criteria untouched. This validated
pending-publication handoff can finish developer execution without a deployment
receipt; it does not establish deployment or permit Done. Ordinary executions
retain the strict exact-deployment receipt requirement.

The source handoff producer is connected, but the native poller still does not
advertise support. Native release workspace preparation, execution, trusted live
observation and subsequent independent delivered-source verification remain
unfinished. Do not enable the project setting or call this an operational
end-to-end release workflow yet.
