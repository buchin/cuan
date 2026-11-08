# Native release execution

## Independent source review before publication

The production build-gate benchmark exposed a gap in the current order:
developer checks can trigger native merge/release before independent source
review. Candidate `441ca645` added release-check bypasses outside its approved
scope. Project tests passed, and the release had to be cancelled before activation.
Passing those tests did not establish scope compliance. The same task was resumed
for correction; it must not be accepted as an autonomous successful benchmark.

The source-review admission path preserves these invariants:

- Review the exact developer source independently against the saved original
  scope before native merge or release. Test fixtures must not authorize new
  production bypasses. Reviewer execution stays read-only.
- Bind release admission to both the developer handoff and the successful
  source-review attempt. A changed SHA, changed authorization, failed review,
  cancellation or revoked project connection invalidates admission.
- A passing source review is a stage result, not task acceptance. Preserve
  incomplete publication and live-behavior criteria. Never infer review success
  merely from process exit zero or a partial checklist.
- Publish only the reviewed source through the registered native path, then
  record fresh post-process revision/health evidence. Perform additional live
  checks when required by the original task; retain the existing visual gate.
- Return source defects to the same task's developer. Reconcile publication
  failures without repeating implementation or replaying uncertain effects.
- Keep immutable stage identities and evidence separate from retry counts;
  successful publication/verification stages are not failed developer retries.

Regression coverage checks rejection before publication for unreviewed,
changed, cancelled and explicitly rejected source, successful handoff of the
reviewed source, and preservation of every original acceptance requirement.
These gates are implemented in the task admission and Inbox coordinator, with
memory and PostgreSQL queue coverage. Production rollout and the end-to-end
benchmark remain pending; passing regression tests is not proof of deployment.

Reviewed launches use the `native-release:v2:` request namespace. Historical v1
launches remain readable without acquiring source approval or fresh execution
authority. An already verified publication is not repeated merely because its
request namespace changed. Active older executors must finish or be reconciled
before replacing them; a new namespace is never permission to replay effects.

Legacy developer attempts without a backend-managed source handoff cannot
retroactively acquire one from a model report or passing verifier. Recovery must
preserve their exact source and original authorization through a native stage;
it must not forge runner evidence or weaken read-only/publication restrictions.

Source observation does not depend on automatic deployment. After passing native
checks, code sessions with publication disabled still record the clean local
checkpoint and diff against their pinned execution baseline. Pending tracked or
untracked files are preserved and are never attributed to HEAD. This supplies
native source evidence for independent verification; it creates no release
handoff, managed publication ownership, push permission or deployment receipt.
Verifier/read-only sessions do not enter this developer observation path.

Initial source-verification launches use `verification-only:v2:` identities bound
to task, exact commit and developer attempt. Repeated scans of that same handoff
reuse its launch. A new developer handoff at the same commit cannot reuse an old
verifier bound to another attempt, or fall through to production inspection just
because the older verifier stopped. Legacy v1 launches remain readable and are
not rewritten. Runtime-recovery and finalization identities retain their existing
bounded contracts; this change grants no additional recovery or release authority.

An older dispatcher may already have started a current-production inspection
after an initial verifier collision. Once that exact observation is terminal,
the coordinator can recover its retained managed developer handoff into source
verification. The typed observation must bind the same source attempt and SHA;
native source checks, original publication policy and handoff receipts are
revalidated. Active/cancelled observations and unrelated source bindings cannot
take this path. The observation remains history, never source approval, release
permission or a reason to repeat development. Verifier-backed and prepared-release
inspection branches retain their own protocols and do not enter this recovery.

New coordinator reviews must include an explicit `source_review` verdict. The
leading response instructions and provider JSON schema both include that field
and the `publication` remaining-work stage. Legacy assessments with a missing
source verdict and native independent-source evidence can receive one bounded
assessment-only refresh under `Source review policy: v1`. The refresh uses current
evidence and cannot approve source from the old report, rerun execution or reset
correction budgets. Explicit newer verdicts are not treated as missing fields.

GitHub merge HTTP 409 is a confirmed source conflict, separate from transport
failure or an uncertain merge outcome. Preparation saves the fixed
`source_merge_conflict` diagnostic against the native attempt and returns that
typed code to the runner. The runner retains a concrete reconciliation gap in
its final failure report; raw provider messages are not forwarded. This does not
retry a merge, rewrite the source approval or execute the release script. A
developer must reconcile only the approved source with the current branch and
obtain fresh checks and independent review for the resulting checkpoint.

Ordinary repository developer claims refresh the registered project branch ref
through the saved repository connection before returning execution context. The
private task branch still resumes in place and pins that imported baseline;
refreshing does not check out source files, reset commits or resolve conflicts.
Read-only verification, inspection, operational and native release stages keep
their exact-source protocols and do not refresh a developer baseline. A failed
refresh stops the claim before provider execution and retains saved work.

Managed-release developers own local reconciliation with that baseline, including
local task-branch merge/rebase and conflict resolution when original scope allows
commits. They cannot change the shared branch, push, open PRs or deploy. Remote
merge and publication remain native stages after independent source review.
Before running project checks for a managed developer handoff, the runner verifies
that the resulting commit contains the pinned baseline supplied for that session.
An unchanged older branch or unverifiable ancestry fails with a concrete local
reconciliation gap, before another source-verifier cycle. This read ignores Git
replacement/graft ancestry and does not modify the task branch. Remote changes
after session admission still go through native merge and target verification;
the pinned baseline is not a claim that the release branch can never advance.

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

The source handoff, native worker, preparation transport and delivered-source
independent verification are implemented but staged. The poller advertises native
release capability only when its operator explicitly sets
`CUAN_RUNNER_NATIVE_RELEASE=true` in the **worker service environment**; the default
is off. This switch does not grant a project publication permission. Project
`CUAN_MANAGED_RELEASE=1`, saved auto-deploy approval, original task authorization,
exact source/target checks and saved verifier/SSH trust remain required.

Before opting in a worker, validate its actual explicit runtime tool path and
private execution, saved project publication verifier and pinned host trust, then
run the real autonomous code plus deployment benchmark through independent Done.
These production readiness checks are still unproven; do not treat opt-in or
protocol fixtures as operational readiness. `cuan task execution` identifies saved
intent without exposing approval or credentials; parsing is not release proof.

## Repository contract

Track `.cuan/release.json` alongside its executable entry point:

```json
{
  "version": 1,
  "command": ["scripts/deploy.sh"],
  "timeout_seconds": 1200,
  "required_tools": ["bash", "git", "go", "node", "npm", "ssh", "scp", "sshpass", "rsync", "python3", "tar", "sha256sum", "curl"]
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

If a native release verifier is rejected before model execution because the
running release changed while queued, the coordinator can schedule one bounded
read-only successor inspection. It preserves the original source, delivery,
approval and failed-attempt history. Fresh native observation and Git ancestry
must prove the new running target contains the delivered target; snapshot
preparation independently checks the implementation ancestry. Cancellation,
model results and actual check failures do not qualify. The replacement verifier
still has to pass original acceptance and unchanged-source checks; recovery
never repeats publication or accepts work merely from the earlier delivery.

A managed developer can find that the approved implementation already exists in
the refreshed baseline. After native comparison reports no code changes and
project checks pass, the runner records that existing clean HEAD and release
descriptor for independent source review. It records no new diff, commit, push
or deployment, and keeps both change/deployment-required flags false. Dirty
workspaces and failed checks cannot use this path. A source handoff is still not
acceptance or live publication proof; the original criteria remain authoritative.

```sh
cuan release inspect --path . --commit FULL_SHA --branch dev [--runtime-path PATH]
cuan release run --path . --commit FULL_SHA --branch dev [--logs]
```

`inspect` does not run the script. `run` explicitly uses the maintainer process
environment, like invoking that script directly. Do not use this operator path
as a Cuan user's credentials or as an Inbox delivery grant. Bound task sessions
cannot run this maintainer command. The native Go executor instead requires an
explicit environment slice; nil is rejected and never inherits host credentials.
A future Inbox delivery stage must resolve the saved owner/project environment
and original authorization before invoking it.

### `--runtime-path` option (inspect only)

The `--runtime-path <PATH>` flag verifies that every tool listed in
`required_tools` exists and is executable within that exact PATH string—without
running any tool or inheriting the maintainer's environment.

| Invocation | Behavior |
|---|---|
| `--runtime-path /usr/local/bin:/usr/bin` | Checks `required_tools` against the given PATH only. Exit 0 if all present; exit 1 with tool name if any are missing. |
| `--runtime-path ""` | Fails (exit 1) if any `required_tools` are listed. Never falls back to the host PATH. |
| _(flag omitted)_ | Skips runtime check entirely—backward-compatible behavior. |

```sh
# Verify tools exist in a custom runtime before releasing
cuan release inspect --path . --commit SHA --branch dev --runtime-path /opt/deploy/bin:/usr/bin

# Explicitly test that an empty PATH fails (proves no host fallback)
cuan release inspect --path . --commit SHA --branch dev --runtime-path ""
```

The check uses `release.CheckRuntime` with the explicit PATH; it never executes
the tools, runs the release script, or touches the host environment.

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
Before initial publication the endpoint requires the saved SSH-release observer
configuration and host public key. It creates private known_hosts/config and
ssh/scp wrappers using explicit -F, so OpenSSH does not read the host user's
passwd-home profile, global trust or SSH agent. Repeated identical setup reuses
immutable files; changed/symlink trust is rejected, and concurrent setup is
serialized. Maintained deployment health checks no longer bypass host-key checks.
Dependency preparation and actual owner production connectivity/compatibility
still need proof before activation; local fixtures do not prove them.

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
observation prevents mutation. The initial preparation endpoint invokes these helpers. Native-stage scheduling is implemented; runtime production readiness and
delivered-source independent verification remain unfinished; the poller capability is still disabled.

Concurrent initial preparation requests serialize on the original developer
checkout, reread native source and approval after acquiring ownership, and reuse a
preparation produced by the winning request. Source and project-cache checkouts
must be distinct. Missing access, dirty/replaced source and revoked authorization
stop before merge. A failed/unknown remote outcome is not called success; another
active request must re-observe exact push/merge state before further mutations.

## Native scheduling from Inbox

Before ordinary model review, Inbox checks for a canonical successful developer
handoff. Frozen managed project configuration, exact current native source and
original saved Inbox approval must all match. It then reserves the existing
deterministic native launch through normal durable admission, without another PM
model request or AI-capacity query. Reconstructed scans reuse the recorded launch.
This scheduling does not check off criteria, claim deployment or mark Done.
The poller capability remains off until delivered-source verification and owner
runtime readiness are completed and exercised in production.

## Completed delivery lineage for verification

`NativeReleaseDeliveryForAttempt` reads a completed successful native attempt's
canonical delivery, preparation and post-process deployment observation together
with the original developer source/check evidence. It preserves source and target
as distinct commits and rejects revoked/changed source, failed stages, mismatched
records and invalid process chronology. Reading historical delivery does not
reopen completed release admission or assert that production has not changed
since that observation. The native completion gate also requires preparation to
precede process startup. Delivered-source verifier intent/admission and final
acceptance still need to be connected to this validated lineage.

Delivered native verification reuses `LiveInspectionIntent` with
`purpose: native_release`. Its predecessor/source-attempt identity is the completed
native release stage; implementation source and delivered target remain separate
commit fields. Validation requires the completed canonical delivery and original
owner/request binding, rejects unrelated targets and mixed recovery modes, and
admits only the existing read-only inspection lane through a claimed launch and
exact native-stage predecessor. It grants no implementation/publication authority.
Inbox now schedules that inspection after validating completed delivery,
original saved approval and the current running release identity. It reuses the
existing live-inspection launcher rather than generating another PM model turn.
Actual verifier execution and final acceptance still need end-to-end proof.

Native delivery scheduling rejects archived projects, revoked developer source or
a running release different from the recorded target. GitHub repository identity
comparison follows provider case-insensitive owner/repository semantics; commit
identity remains exact. Reconstructed review scans reuse the deterministic launch.
The original task stays pending until the independent verifier establishes its
original criteria and the normal acceptance gates pass.

## Acceptance verification status

The runner treats delivered native inspection as source-ancestry verification,
while current-outcome inspection remains separate. Backend protocol tests now
exercise native delivery → claimed read-only verifier → exact snapshot content
inspection → verifier completion → normal PM acceptance gates → Done, preserving
the original dev SHA and original criterion IDs/text/evidence. Only the verifier
report completes that fixture's pending criterion. These tests use synthetic
model/process/health facts and do not establish autonomous production execution.
The native capability remains disabled pending private runtime readiness and a
real production end-to-end run without owner intervention.

The saved project public key must have legitimate owner-approved provenance.
Cuan does not copy ambient developer known_hosts, discover-and-silently-trust a
new key, or write password values into SSH runtime files. A missing observer/key
stops preparation before source push/merge. Runtime setup includes public trust
only; authentication remains the explicit saved project environment.

## Runtime tool and dependency checks

An optional `required_tools` descriptor list names up to32 distinct bare executable
names. Invalid paths/arguments/duplicates are rejected. `CheckRuntime` searches
only the explicitly selected PATH, ignores relative entries, checks executability
and never executes the tools or inherits the maintainer PATH. `Run` enforces the
same check before any process. Native initial publication loads the exact admitted
source descriptor/hash and checks this runtime before push. These are availability
checks, not version or functional probes.

The maintained Cuan release script runs locked `npm ci` before frontend tests,
since private source checkouts do not contain node_modules. Native npm config/cache
and Git global/system config are isolated from host credentials. A clean private
npm installation has been tested locally; it does not prove production build or
deployment compatibility.

Private release checkouts record a credential-free canonical GitHub `origin` for
build metadata. Existing changed origins are rejected rather than rewritten.
Every successful preparation response, including replay, observes the frozen
release branch through the project's saved repository connection. A remote target
that differs from the canonical preparation blocks execution; it never rebinds
the target or repeats publication. Stable `CUAN_NATIVE_RELEASE_SYNC_SHA` and
`CUAN_NATIVE_RELEASE_SYNC_BRANCH` execution context lets the maintained script
check its actual HEAD and branch without fetching with ambient credentials.
Ordinary maintainer execution still fetches and compares origin. These markers
are not authorization, publication evidence or task acceptance.

Production readiness still requires saved publication-verifier configuration,
pinned SSH trust, actual worker tool availability and a real autonomous code plus
deployment benchmark through independent verification. Local protocol tests do
not establish those outcomes.

Publication scope distinguishes a narrow product behavior limit on the `inspect`
command from whole-task release limits. A prohibition explicitly limited to
running deploy during inspect is not a whole-task deploy ban when the approved
scope separately requests deployment. Without that separate request it remains
restrictive. Any additional whole-task prohibition still wins. This changes no
saved instructions, existing attempt contract, approval or acceptance evidence.

Cuan build and test tooling is independent of the development hosting platform.
The standard frontend tests validate documentation and components without calling
an external platform renderer. Product operations belong to the `cuan` CLI/MCP;
required capabilities must not depend on a developer platform session or identity.

Worker queue admission saves initial task status, assigned agent and report reset
in the same transaction as the claimable attempt. Revision continuations preserve
the saved report and criteria. Rejected admission rolls back both records. Revision
feedback is saved before publishing queue work. After publication, the launch
handler only observes that attempt: it cannot reset a worker's running or terminal
outcome, report, or checklist. A fast claim is acknowledged using the exact saved
attempt, without starting another process or granting acceptance.

Native process journals retain a bounded, redacted diagnostic tail, and native
workers send process start/end plus that tail to the existing task log transport.
Selected credential environment values are removed before persistence/logging;
raw output is never added to the public process-result JSON. Saved diagnostics
can be re-emitted when observing a completed journal without replaying release.
This is an end-of-process tail, not live line-by-line streaming. Older journals
without diagnostics cannot recover discarded process output.

Release scripts can emit exact newline-terminated milestones such as
`::cuan-release-stage::build` to publish live progress in the existing task logs.
Supported stages are `checks`, `dependencies`, `tests`, `build`, `transfer`,
`activate`, and `health`. The executor forwards only fixed labels, once per
stage; unknown, overlong and ordinary output lines remain private diagnostics.
Milestones are untrusted progress hints, never successful check, publication or
acceptance evidence. Existing scripts without markers keep their prior behavior.

Inbox PM processes use backend-supplied task/project/room snapshots and validated
coordinator actions. Their sandbox masks account/workspace MCP discovery files
with an empty private configuration. This avoids advertising unconfigured Owner
CLI connections or inheriting another context's MCP authority. Provider settings
and authentication remain available, including refresh; host MCP files are not
edited. Missing snapshot evidence remains uncertainty. Worker task capabilities
and Owner CLI connections are separate execution contexts.

Activation-time and post-process publication observations are distinct. The
maintained activation script may record its healthy receipt before the release
script exits. A native worker requests an independent observation with its saved
`process_finished_at` boundary after process completion. The backend validates the
active prepared attempt and observes the registered target again; an earlier
activation receipt cannot satisfy this request. The immutable activation receipt
is retained, and a separately keyed `native_publication_observed` event binds the
exact process finish. Completion and delivered-source verification require that
post-process evidence. Duplicate requests reuse the matching final observation,
without rerunning deployment or rewriting the activation timestamp.

For Cuan's own production project, the authenticated native observation endpoint
can identify its responding production build when both registered repository and
expected release SHA match. It does not require an additional SSH connection to
observe that same process. Development builds, other repositories and different
revisions cannot use this path. Active-attempt, preparation, process-finish and
project-revalidation checks still apply; observation never accepts the task or
replaces independent feature verification.

New independent verification snapshots retain the source commit's immediate
parent in their bounded history, including when source and target are identical.
This lets a verifier inspect the actual implementation diff without fetching a
remote or borrowing the mutable developer checkout. Unrelated cache branches,
dangling objects and old build artifacts remain excluded. Existing snapshots are
preserved rather than rewritten while an inspection is active.

Task source identity and production release identity can differ after merge.
Native `result_commit_contained=true` plus verified publication establishes that
the source is included in the observed release. PM and independent verifier
instructions explicitly distinguish this from SHA equality; an original request
for an exact revision remains binding. Source inclusion proves publication only:
feature behavior, independent checks and every other original criterion remain
required. A read-only verifier is not expected to deploy again.

Inspection dependency preparation retains a bounded, redacted npm output tail
when setup fails. The private installer environment still excludes task/provider
credentials, user npm configuration and lifecycle scripts. Review treats that
recorded prerequisite failure as occurring before capture or feature checks;
missing images from an unstarted capture are not a separate implementation defect.
A setup failure never establishes feature correctness or permits editing a
read-only snapshot. Resolve the actual diagnostic before obtaining fresh evidence.

Verifier recovery uses one durable retry per original source/scope. Saved launch
state failure transitions or native dependency-setup failures with empty check
records and no final report can receive a new read-only inspection. The running
target is observed again; source, repository, original authorization and failed
history remain bound. A prepared published source also requires fresh registered
containment when the target changes. Unknown errors, actual failing checks,
repository changes, cancellation and final blocked reports do not qualify for
this setup recovery. Another deployment cannot renew its retry allowance.

Terminal help text is not a browser visual requirement. Inspection classification
keeps command-output descriptions separate from explicit UI, layout, web-page,
image and screenshot requirements. A CLI-only inspection therefore does not bind
a frontend visual recipe or prepare npm dependencies solely because its request
mentions the display of `--help`. Real UI work retains the visual quality gate.

Public-assistant task admission leases the same selected-model/task-slot pool used
by native execution, then releases that observation before queue publication.
Chat routing and legacy chat fallback defaults cannot admit an unavailable task
route. Known cooldowns and occupied task slots keep its saved launch queued,
without provider inference or another failed attempt. The worker leases again at
execution time; admission is not a guarantee of future provider availability.

A capacity-stopped current-outcome inspection can save one capacity continuation
for its exact original stage, even while no route is available. The intent binds
`capacity_recovery_attempt_id`, preserving the original source, authorization and
any prior infrastructure repair. A fresh running target is observed; published
prepared sources retain current registered containment checks. Existing launch
scheduling waits for permitted capacity and revalidates source/scope at admission,
claim and completion. It neither changes model/fallback policy nor accepts the
failed parent, and cannot chain another capacity allowance from its own failure.

Human revision feedback remains limited to 6,000 runes. Saved internal execution
envelopes have a separate 16,000-rune storage bound because they also retain typed
protocol metadata and binding owner constraints. The launch handler accepts the
larger envelope only when it exactly matches a currently claimed, leased,
owner-bound saved launch with a valid typed intent. A marker in user feedback is
insufficient. Size validation grants no source, execution or acceptance authority;
all ordinary checks remain required. Owner execution diagnostics expose the saved
revision length without returning private instruction prose.

Review context also includes validated `native_preparation_history` from terminal
native stages, even when later execution/completion failed. It preserves the
actual attempt status and explicitly identifies preparation-only facts. The
history reader revalidates original scope, source lineage, native event ownership
and attempt timestamps; changed/cancelled context is rejected. This read grants
no execution authority and does not establish deployment, feature correctness or
acceptance. Current publication and independent verification remain separate.

A publication-restricted reconciliation may inspect an existing production outcome
when the original source has validated terminal native preparation, a recorded
publication observation for that prepared target, and current registered source
containment. Its inspection intent binds `prepared_publication_attempt_id`; task
claim/completion revalidate that historical source/repository/receipt. This grants
only read-only inspection, not publication or acceptance. Purely local work,
missing/revoked history, unverified containment and unrelated releases retain the
existing source-verification requirement.
