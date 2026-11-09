# Native project-check recovery

The telemetry workflow in room `67909438-7e90-49af-bfb8-6c40a003bac4`
retained source `8cd1d1e6c40ea6574cfc93f2cbd6436c59cba316` but its
native `test.sh --fast` gate failed. The recorded failure was
`TestSupplementalVerifierUsesPrivateExactCommitWorkspace`: a different completion
was already retained. This is not evidence of a feature or deployment failure.

## Root causes and correction

The fixture placed its workspace directly under the test temporary root while
the completion outbox deliberately lives beside the workspace directory. Its
outbox consequently escaped test cleanup into shared `/tmp/runner-completions`.
Recycled HTTP server ports and fixed fixture identifiers could collide with an
old, different result. The production idempotency guard correctly preserved the
old result. Nesting both verifier/finalizer fixtures inside their own temporary
workspace tree keeps their outboxes owned by test cleanup. Their mock completion
endpoints now return matching native acknowledgments, and tests check delivery.
No production outbox is deleted and no completion identity check is weakened.

The publication review guard also converted a concrete developer-test correction
into release reconciliation when deployment evidence was missing. A current,
runner-bound failed project check now precedes publication classification. It
requests a bounded developer correction with the actual diagnostic, retaining
original scope, history, independent verification and publication requirements.
Inspection/verifier launches, cancellation, unknown failures, foreign records,
passing checks and missing current reports cannot use this source-correction path.
The existing review evidence digest revisits only affected saved failures.

## Validation and limits

Targeted verifier/finalizer fixtures passed three consecutive runs. Targeted
server tests cover native failure priority, model-pass rejection, stable repeated
classification and rejection of foreign/historical/canceled/incomplete evidence.
Existing publication and prerequisite review tests passed. The project fast gate
also passed on the final source (`GOGC=30 GOMAXPROCS=2 GOFLAGS=-p=1
./scripts/test.sh --fast`).

These checks do not establish production recovery or task completion. The same
retained telemetry task must still correct its source, pass independent checking,
publish and produce live evidence without manually forcing its status.

## Lean follow-through

Production review `8b7dbada-9ed5-4de0-a7dc-d18fe53a0f9d` spent 191.2 seconds
in its provider process merely to request the already-known test correction.
The corrected publication guard classified implementation, but its embedded
6000-byte diagnostic plus instructions exceeded the delegation contract and
admission returned `Invalid delegation`. The task remained blocked; this is
not successful recovery.

Native check correction now keeps a short failure-tail excerpt and refers to the
complete immutable native record through the existing bound task tools. A long-log
regression goes through the real review parser and existing correction queue,
including duplicate application. The lifecycle selects this deterministic
handoff before provider inference. It never accepts the source, changes scope,
resets correction budgets or bypasses native launch admission. Unknown outcomes
and independent verifier findings still need their ordinary assessment.

Production `b7b9f4c69d51434fe5e4e8b4756e84f54ea2b865` passed exact-SHA
health verification. Request `491cdff6-2726-42c7-a325-df9fe18e2d42` ran from
17:51:58.930 to 17:52:09.113 (+08), recorded `native_check_correction`, and had
no provider stage. The same telemetry task automatically entered developer
attempt `att_50ac926908cecd2779da1e162aafb8ef` at 17:52:10.338. This proves
the correction admission and removes the prior 191.2-second provider decision;
it does not yet prove that the corrected feature passes independent/live DoD.

## Separate visual coverage finding

Room `a86b4638-8afc-4fea-a542-d07d52880488` received Mermaid and terminal
screenshots during its post-release check although its approved scope is the
Inbox list indicator. `scripts/verify-ui.sh` invokes `web/scripts/verify-ui.mjs`,
which always captures that broad set. Its component manifest does not include
`InboxSubPanel` or `TaskProgress`. Those generic captures alone do not establish
the requested indicator's visual correctness. This backend correction does not
claim to repair that coverage/attachment selection gap.

The post-publication verifier report also left deployment incomplete solely
because its role forbade deploying, despite identifying the published containing
snapshot. The runner now gives this role a point-by-point DoD checklist mandate
and exposes its actual native post-publication stage, source/target containment,
repository and observation time after snapshot checks pass. It explicitly
distinguishes that identity evidence from feature correctness and remote health.
Current-outcome inspection of an unpublished source never gets this declaration.
Existing real snapshot tests cover both cases, failed checks, source mutation,
native image transport/tampering and unavailable evidence. No acceptance gate is
relaxed. Already delivered native screenshots are no longer offered for redundant
reattachment. This does not yet fix the registered visual recipe's coverage.

Direct inspection of the delivered native `inbox-390.png` confirmed it shows the
chat/composer, not the requested Inbox list indicator. Its existence therefore
cannot prove mobile alignment of the indicator. Preserve this negative finding
when assessing the task rather than treating a generic capture as visual DoD.

## Role routing and repeated preparation

Developer attempt `att_50ac926908cecd2779da1e162aafb8ef` completed with native
checks passing on source `6485a9dd36f2f01a9b6bc1f2234590e6e21490df`.
PM request `796c30aa-8bf9-4cea-ac07-f1f64158dc4d` requested independent checking
but retained only decision/explanation/revision, with no `remaining_work`.
The legacy unclassified revision path started ordinary writable attempt
`att_c444b37073ae3bbd1840651025f86d65` instead of a typed verifier. This is
another routing failure, not evidence that a verifier ran. A steering message
was delivered to preserve source, avoid reimplementation/deployment and report
only actual findings; native independence remains outstanding.

Ready tested developer commits now use the existing deterministic handoff
pattern to request mandatory independent verification before provider inference.
The existing source-verifier admission still checks original authority, native
source evidence and launch identity. An unclassified revision on a tested
implementation also defaults to read-only inspection instead of writable
execution, excluding explicit source rejection and owner/external decisions.
Regression tests exercise the real saved assessment/launch path and prove exact
source identity, one verifier launch and no acceptance without independent work.

Setup inspection found that npm preparation ignored the managed owner cache
already leased by the runner and created/deleted a new download cache every time.
It now accepts only the internally selected managed cache from that lease;
private installer HOME/configuration and separate node_modules remain unchanged.
The native npm offline fixture installs a real integrity-locked tarball into two
separate workspaces from one warm cache, checks the source lock is unchanged and
proves lifecycle scripts did not run. This removes repeat downloads; it does not
claim that all installation/materialization cost or this Go task's latency is
eliminated. Git snapshots already reuse retained directories and bounded local
history; Go caches already persist across these roles. No shared mutable Git
checkout or node_modules symlink was introduced.

Release `27dc12833f6c80ff588786a0c466a451e6fe1ddf` passed exact-SHA health
verification at 2026-11-09T10:14:44Z. It includes the role handoff, verifier DoD
stage context and shared npm-download preparation. The already active legacy
ordinary execution is preserved rather than being retroactively labeled an
independent verifier. No production source-verifier transition is credited until
the actual typed launch and source binding are observed.

At 18:18:26 (+08), task telemetry entered native verifier
`att_cf7169eb686500ecef880fd4d05b5e5b`, bound to developer attempt
`att_c444b37073ae3bbd1840651025f86d65` and exact source
`712d56e004d3767671dd64939836ad57d0972e89`. Official `task execution`
confirmed `intent_kind=verification`, read-only repository and denied push/deploy.
The agent stage began at 18:18:33, about seven seconds after runner start.
This transition used the existing validated native developer handoff and its
synthetic source-review assessment `ef0ce011-36d1-4889-aa3c-66058d8c99e8`;
it is not evidence of a model Inbox request or of the generic handoff path being
exercised. Verification remains active and no independent pass, publication or
Done is credited yet. The short observed preparation also means cloning/setup
cannot explain the earlier multi-minute PM/role-routing delays in this task.

## Independent pass and publication blocker

Verifier `att_cf7169eb686500ecef880fd4d05b5e5b` finished successfully at
18:25:38 (+08), preserving source `712d56e004d3767671dd64939836ad57d0972e89`.
Its delivered report records 14 targeted telemetry/context tests (3.81 seconds),
the project fast suite and build, including nil/success/failure DB status,
snapshot consistency and RFC3339 observation time. Source approval was saved,
and native release attempt `att_21259371d216591d560facde755d0116` started
automatically at 18:26:31. It failed two seconds later before model/release
execution with generic preparation-unavailable output and no diagnostic logs.
The feature is not deployed or Done.

Remote observation showed `dev` at `ce52b096` and the task branch still at older
source `6485a9dd`. The reviewed source had been rebased to `712d56e0`. Native
source push only attempted a regular push to that existing task branch, which
cannot publish rebased history. A bare-repository regression reproduces this
failure mode. Production's generic preparation error does not establish which
preparation step failed, so this is a confirmed publication gap, not proof that
all causes of that production attempt have been resolved.

When the task branch already points elsewhere, source publication now preserves
it and uses `refs/heads/cuan/verified-source/FULL_SHA`. Existing matching refs are
read/reused; mismatched refs fail. Creation uses an empty-expected-value lease,
so a concurrent writer is preserved. No task/default branch is force-updated.
Bare Git tests cover rebased, already published, conflicting and concurrent
references; project access/protected-branch tests and the existing full native
initial-preparation contract passed. This change is committed separately from
any future recovery policy. Existing failed launch history must not be reset;
the retained verified source still needs bounded native preparation recovery
and live deployment evidence. Preparation diagnostics also need a concrete
stage code rather than the current generic error.

## Bounded native preparation continuation

The recovery intent binds the failed preparation attempt explicitly alongside
the unchanged source/reviewer/owner/repository/branch/manifest identities. It is
admitted only for the native failed completion stating that no model or release
process started, with complete exit/timing metadata and no cancellation. The
original source and independent approval are revalidated. A deterministic child
key permits one continuation per source, never another recovery of that child.
The ordinary claimed-launch/predecessor checks still admit the new attempt;
the failed predecessor is never reopened. A newly observed successful
publication instead goes to inspection and cannot trigger another deployment.

Core tests cover source/owner tampering, cancellation, unknown exit, process-stage
failure, typed recovery admission, preservation of failed history and rejection
of a retry chain. The server queue test drives a real native failed completion
and durable continuation, proving the original source/review stay bound, no PM
capacity is consumed and repeated scans create no extra attempt. Existing native
source, publication and claimed-launch tests remain required.

Preparation now persists a bounded native failure-stage code and source identity
without Git/SSH stderr or environment values. The runner retains only recognized
codes in its completion; merge-conflict classification is unchanged. Native
tests verify runtime/source-identity/source-push stage records and reject
unrecognized/secret-bearing transport diagnostics. This improves future
diagnosis; it does not invent a stage for the already generic failed attempt.

## Production recovery and publication observed

Framework release `2560ae084e0d3b08c9cd2a69944acd9b26af57cb` passed exact-SHA
health at 2026-11-09T10:50:59Z. Native preparation recovery automatically created
attempt `att_63b05dd489d23b5bbedfbc57c2262530` at 18:49:23 (+08), keeping
source `712d56e0` and the existing independent review. Preparation succeeded:
the immutable source ref contains that exact SHA, the old task ref was retained,
and the target branch was reconciled to merge `7d6b9285b52a111958b88bf28c96e66d6403a337`.
No developer or source-verifier execution was repeated by this recovery.

The native process ran checks, dependencies, release tests, build, transfer and
exact production health observation, finishing at 18:57:41.650. Attempt completion
succeeded at 18:57:42.111. Its backend native delivery receipt binds the original
source, target, manifest, owner scope and process timestamps. Production health
matched `7d6b9285`, with 38 migrations. This proves publication, not final DoD.

Post-publication inspection initially queued behind storage admission. The
installed maintenance job completed successfully but storage remained about
1.9 GiB free, below the 2 GiB admission floor. Registered project SSH credentials
and pinned host trust were used only to inspect host storage and clean disposable
OS package caches via `apt-get clean` and `apt-get distclean`. No application
source, task evidence or business data was removed. Logs older than 14 days were
checked but none qualified; no log deletion was performed. Available bytes were
then 2,333,130,752. This operational cleanup does not prove recurring storage
pressure is solved; maintenance eligibility and protected retained snapshots
remain follow-up work.

The same queued launch resumed automatically as read-only live inspection
`att_dcefcf8253510250e69490bc3ae0374e` at 19:06:49.384, binding original source
`712d56e0`, delivered target `7d6b9285` and release attempt `att_63b05dd...`.
The source and release failures remain in history. This final inspection is
still active; no final DoD acceptance or task Done is credited.

## Task acceptance and scope waste found

Live inspection `att_dcefcf8253510250e69490bc3ae0374e` completed successfully
at 19:14:51.595 (+08), with unchanged repository evidence, passing native project
checks, source containment and live-result receipt. It reported the deployed
database cases, snapshot consistency, observation timestamp and active healthy
release. PM request `efdabb2d-4912-4b4b-ac09-f7aa2cbe9814` accepted at
19:16:46.078. Official task state is Done, and all seven saved DoD items have
evidence. No manual task-status update was used.

This is not evidence of a lean or fully scope-correct run. Seven attempts and
platform interventions were necessary. In addition, the live inspector wrongly
received the generic native UI recipe and produced eight unrelated UI captures
despite the task's explicit browser prohibition. PM then read those images before
acceptance. They are not telemetry evidence. Preserve this violation in the
history rather than counting it as required validation.

The visual classifier failed to strip `Jangan mengubah komponen UI` as a mutation
prohibition. Its existing English/Indonesian negative-UI vocabulary now handles
component wording and touching UI, while positive UI inspection exceptions still
require vision. Native recipe binding also respects explicit English/Indonesian
browser prohibitions independently of the ability to read attached images. Tests
cover the actual production sentence, backend variants, positive visual duties
and browser restrictions. The old captures remain historical; no source/evidence
is deleted to hide the waste. A fresh production workflow must prove this scope
fix and the other corrections before claiming autonomous readiness.
