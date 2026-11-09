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

## Scope guard release and fresh lean-workflow observation

The official `cuan release run` deployed
`c373cde26a127a1ee38db5ef1305445b1469a9e5` from `dev` on November 9.
The retained CLI result records 11:24:07–11:27:25 UTC, exit zero,
unchanged source, 38 applied migrations and production `/healthz` healthy
with that exact commit. This proves delivery of the scope guard, not its
effectiveness in a fresh verifier execution.

Fresh room `7dda3de9-a7fe-4cab-b0eb-3aa6cca87a40` requests one actual
backend/tooling task to identify and reduce repeated dev/verifier dependency
setup. Original constraints explicitly prohibit UI changes, browser automation,
screenshots, emails and history/source/evidence deletion. It requires immutable
source identity, independent verification and native publication if code changes.
The request explicitly permits a factual no-change finding if no useful safe
optimization exists; it does not require manufacturing a cache implementation.

The send command reached its local observation timeout, but official room
readback confirmed saved request `05ea3319-aa82-4742-8883-f71ba9cca5d1`
running with runtime session `b710d407-c08f-4a5d-96fc-aac9de468c55`.
No duplicate send or retry was issued. The analysis trace records 86,736 prompt
bytes and 41,587 context bytes. These are not token counts, cost estimates or
proof of the cause of latency. At this observation there is no delegated task
or completed provider response; the fresh end-to-end proof remains pending.

Code inspection also confirms DoD assessment already uses an LLM evaluator
(`inboxReviewEvaluationInstruction`) with native attempt/source/publication
evidence gates. An external judge would need to improve or replace that
assessment, rather than add an otherwise identical review stage. No new judge
dependency or acceptance bypass was introduced.

## Storage recurrence during the fresh task

The PM completed request `05ea3319-aa82-4742-8883-f71ba9cca5d1` without
format repair and created task `msg_6fb49fb4-6247-4b21-880e-e230a0cd2759`.
Acceptance-to-final persistence took approximately 98 seconds; provider process
time was 84 seconds. The task remained queued before any worker attempt because
available production storage was below the native 2 GiB admission floor.
An official maintenance trigger completed successfully, but did not recover
sufficient bytes. Success of the maintenance job is not proof of capacity.

Production allocated-byte observations, in decimal GB, found `/opt/cuan` about
43.0 GB, including data 36.8 GB, backups 5.7 GB and releases 0.40 GB.
The data/projects traversal reported about 21.6 GB, including verification
snapshots 8.23 GB, task workspaces 7.38 GB and release workspaces 3.05 GB.
These are point-in-time directory observations, not additive independent
reclaimable estimates: pack files can have hardlinks across roots. The whole
filesystem scan took about 143 seconds, further supporting asynchronous bounded
inventory rather than recursive scans on interactive/admission request paths.

One old task repository had 44 packs, 4,555 loose objects and about 1.07 GB of
Git metadata. Under its existing exclusive task execution lock, native Git GC
with pruning and both reflog-expiration classes disabled reduced that directory
to about 93 MB. Before/after SHA256 of sorted unique Git object IDs remained
`fdfe7acb7b68b495f51d1edcd8eaa88e6f1c5ceb594c09e1e6cf5f0e72f62531`;
the show-ref hash also remained identical. However filesystem available bytes
did not improve: observed pack link counts of 3 and 7 explain why one-directory
repacking cannot be credited as equivalent physical reclamation. Do not repeat
mass GC on that assumption; it can materialize additional private packs.

The unmanaged service-user Go build cache was separately identified at
488,951,808 allocated bytes. No open readers were returned by the scoped lsof
inspection. Official `go clean -cache`, as ubuntu with an isolated environment
and that exact GOCACHE, reduced it to 12,288 bytes. Filesystem available bytes
then rose from 1,778,290,688 to 2,265,710,592. This is a one-time OS cache
recovery, not proof that native maintenance covers legacy caches or that the
autonomous workflow is reliable. Backups, unique sources and history were kept.

## Fural source comparison (read-only)

Local Fural source inspected on November 9 demonstrates reusable registered
member/repository workspaces (`service/service_workspaces.go`), a node-wide
cache for exact pinned platform-managed CLI payloads with ready markers and
restricted hardlink destinations (`sandbox/harness_cli_node_cache.go`),
Statfs-based interactive usage with asynchronous tree-size snapshots
(`service/system_usage.go`), and at most two concurrent recursive scans
(`sandboxnode/executor_disk_admission.go`).
Its scoped ephemeral sampler cleanup defaults to ten minutes and is called at
runtime startup (`sessionruntime/home_sample_cleanup.go` and
`cmd/session_runtime/main.go`). Rebuildable-path exclusion in
`sandboxnode/storage_move_rebuildable.go` is an explicit storage-move option,
not evidence of universal automatic cache deletion. Native ext4/ZFS accounting
records source and byte semantics. No Fural production configuration was probed;
these findings must not be presented as measured production savings or as a
reason to add a Fural dependency to Cuan.

The same fresh task automatically started attempt
`att_d5470bf99f98d16fcea8112a7efcee4e` after cache recovery. Its actual runner
log at 19:41:47 (+08) still announced frontend dependency preparation for the
backend task. The capture-recipe guard did not fix the separate generic vision
classifier: it matched the word screenshot inside `jangan gunakan otomasi
browser atau screenshot`. The shared negative-capture vocabulary now recognizes
that compound prohibition, and both original-scope and generic image-requirement
classification strip it before interpreting positive screenshot requirements.
Actual image attachments and later positive image-inspection requirements remain
eligible. Targeted server tests cover both developer and live-inspection claims,
the real Indonesian constraints, English variants and positive/attachment cases.
The already-started attempt retains its historical classification; no restart or
manual completion was used to make it appear corrected.

The official release of the second classifier fix,
`6ac7bf385883f9be6fd52b90ce68cecf9032cd3d`, completed November 9 at
11:47:29 UTC. Release duration was 187.5 seconds, exit zero, source unchanged,
38 migrations, and production health matched that exact commit. Readback then
showed the same developer attempt still running with a renewed lease; deployment
did not require restarting or resending this task. Final acceptance remains
unproven.

Saved log timestamps bound irrelevant frontend preparation to 19:41:47.952
through the agent-start stage at 19:42:22.497 (+08), approximately 34.5 seconds.
This is observed preparation overhead, not a measured future speedup and not
evidence that every npm installation takes that time.

Further comparison found that Cuan already installs AGY once under the service
user's `.local` prefix (`scripts/setup-agent-runtime.py`), verifies its downloaded
binary against the provider manifest (`scripts/install-antigravity.py`), and
selects separate account homes through `AntigravityProfileEnvironment`.
Consequently Fural's managed per-member CLI deduplication is not evidence that
Cuan needs another binary cache: no equivalent per-account CLI duplication has
been established. Prioritize the measured workspace/dependency paths rather than
adding that mechanism speculatively. Account-specific credentials/configuration
must remain separate from shared executable installations.

## Persisted terminal animation overhead

The running developer attempt also exposed a narrower bug in the existing
`ptyDiagnosticCollector`: its recognized-status prefix normalization required
at least four characters. Actual retained records contain animated `R`, `Ru`
and `Run` prefixes, so each short frame reset the deduplication state and
permitted the next frame to be stored again. This is diagnostic-log overhead,
not evidence that the model repeatedly executed the displayed command.

Normalization now accepts nonempty prefixes of the same known generic statuses,
preferring the current status when a prefix is ambiguous. It does not broaden
suppression to arbitrary thought text, commands or errors. Original terminal
bytes still go to terminal viewers; only persisted diagnostic records change.
The existing 1,000-cycle test now starts prefixes at one character; a new replay
uses actual ANSI redraw shape across 1/7/64-byte and whole-input transport
frames, requiring one generic status plus the concrete command and two distinct
identical errors. All `TestPTYDiagnostics` tests passed, including split quota,
UTF-8, unterminated-output and specific-work preservation cases. Production
savings remain unmeasured until a new execution uses the updated runner.

The PTY filter release `e6d58a226310fa481673e9cf8368731875c6c630` completed
November 9 at 12:06:20 UTC in 202.4 seconds, exit zero and source unchanged.
Production health matched the exact commit, with 38 migrations. The still-active
developer attempt was not restarted; its old diagnostic behavior is historical,
not a production test of the newly released filter.

The fresh dependency task did not achieve its requested result. Attempt
`att_d5470bf99f98d16fcea8112a7efcee4e` timed out at 20:06:38 (+08), exit 124,
with no saved agent progress/final report. Its native agent stage lasted
1,455,846 ms. One Owner steering intervention had requested a factual checkpoint
and a narrower decisive check, preserving scope and the permitted no-change
finding. Delivery was acknowledged and logged, but no checkpoint followed. This
cannot be described as an autonomous success or a completed optimization.

Existing native reconciliation automatically queued a read-only
`current_outcome_inspection` launch
`launch_67e95b8cef9a603ef79075092b129b339deed167ab72a6c9dec2feba9e1f5bf8`.
It has not started because storage is again below admission capacity. The failed
attempt remains failed; no manual task acceptance was applied. Source inspection
confirmed persisted encrypted task grants already exist, so absent reports alone
do not prove deployment lost the worker's capability.

Further allocated-byte observations found roughly 5.58 GB in provider profiles.
One large profile held about 1.09 GB in provider history/brain/log data and
469.5 MB in `.local/share/cuan-releases`; three profile staging roots together
held about 1.006 GB. No profile had a `.local/bin` Cuan executable in the sampled
case, and these were release-build/staging outputs, not evidence of duplicated
AGY installations. Existing staging pruning requires a matching verified
installed counterpart; it must not be relaxed merely to erase unknown or unique
files. No provider conversation, brain, credential, backup or staging directory
was deleted during this investigation.

## Development backup retention and actual capacity recovery

The native activation path writes one private PostgreSQL custom dump per commit,
but did not apply the separate `backup.sh` timestamped-archive retention policy.
Production had 816 recognized native dumps. An initial conservative 30-latest /
14-day proposal was reduced after the Owner clarified this is a development
installation: retain only the three newest dumps plus current/previous IDs.
The simplified preview nominated 813 dumps / 5,077,295,104 allocated bytes,
retained three and ignored 12 unknown/nonqualifying entries. Nothing was deleted.

The latest release dump `e6d58a226310fa481673e9cf8368731875c6c630.dump` was
restored using the maintained isolated restore-probe script into a fresh private
PostgreSQL instance under ubuntu. It produced 49 public tables. No TCP listener,
Cuan application process, email job or production database connection was used;
the temporary instance was stopped and its copied data removed. This is an
actual restore drill, not merely a successful `pg_restore --list`.

The prepared pruner defaults to preview. Apply serializes with activation,
requires known current/previous links, private single-link regular custom dumps,
validates the two newest restore-point formats and every candidate before any
deletion, and rechecks each removed file's identity. Unknown files/directories,
links, public files and malformed-header data remain untouched. Four retention
tests passed. Existing maintenance integration is explicitly opt-in through
`--backup-keep`; it persists the installation setting and does not enable deletion
for fresh installations by default. All five installer tests passed.

The Owner subsequently approved cleanup and enablement explicitly with `Gas`.
The maintained installer was applied with `--backup-keep 3` using the registered
project SSH connection and pinned host identity. No application restart was
needed. The root-owned persistent configuration contains count 3, and the
existing maintenance unit references pinned tool set
`46484aa29081da0e40db84577affeef75566ee5458cc048e095abbb49bca96a4`.

Official `cuan maintenance run` started the registered job at 20:24:21 (+08).
It completed successfully at 20:25:35, logging 813 eligible and 813 removed,
5,077,295,104 allocated bytes nominated, three retained and 12 ignored. Available
filesystem bytes reached 6,530,834,432. There are four `*.dump` filenames at the
top level because the unknown/nonqualifying entry is deliberately retained;
do not conflate a suffix count with the three validated native restore points.

The same queued read-only recovery automatically started
`att_67e95b8cef9a603ef79075092b129b33` at 20:25:39, targeting running commit
`e6d58a226310fa481673e9cf8368731875c6c630`. Its original failed attempt remains
failed. Initial logs proceed to agent start without frontend dependency setup
or registered visual capture. This is fresh evidence for the negative-screenshot
classification fix in the actual recovery path, not final DoD acceptance or proof
of a completed workspace optimization. The recovery remains read-only, without
push/deploy authority.

The recovery saved a real review report at 20:29:57 (+08), including delivered
artifact `13bc90da-1387-448c-aceb-68a9f08f1749` (`verification_report.md`).
It explicitly left four of six criteria incomplete: no completed dependency
analysis, reuse implementation, targeted reuse tests or final verified delivery
was produced by the timed-out developer. Isolation and original prohibitions
were reported satisfied. Its native agent stage returned after 258,763 ms, then
the runner started project verification. This is a truthful negative assessment,
not a task success or evidence that the developer optimization exists.

Actual recovery logs also show arbitrary typewriter-animated progress sentences
still displacing factual output despite the fixed short generic-status filter.
The collector now groups only spinner rows with explicit cursor-up redraws whose
clean text is prefix-related, retaining the most complete original row and
flushing it when the sentence/output changes or execution ends. Ordinary output,
commands and repeated errors remain distinct. Quota recognition still observes
each original input frame before diagnostic grouping; live PTY bytes are unchanged.
New tests replay 100 complete typewriter cycles across byte-split transports,
require complete progress text and distinct factual errors, and preserve changed
sentences and immediate quota detection. All `TestPTYDiagnostics` checks pass.
This refinement is not yet deployed or credited with production savings.

## Negative inspection decision did not reach the developer

Recovery `att_67e95b8cef9a603ef79075092b129b33` completed successfully at
20:33:53 (+08), exit zero. PM request `bd7f230f-4d15-4224-a460-ada4d203d740`
saved `decision=revise`, `remaining_work=implementation`,
`requested_outcome=unfinished`, and `manager_action=continue` at 20:35:24.
Thus the model did choose continuation; the application handoff stranded it.

`localSourceCorrectionAllowed` admitted only a failed legacy inspection after a
timed-out developer with deployment prohibited. It rejected a successful
read-only inspection that correctly found the implementation still absent, and
also rejected the original task's existing deploy permission. The typed-inspection
guard then blocked ordinary execution and displayed only the unmet outcome.

The same bounded correction path now admits a successful negative inspection
only with current native passing project checks, current final report and
unchanged-repository evidence. It still requires a source-less current-outcome
inspection tied to this task's timed-out, uncanceled original execution, matching
saved PM implementation decision, exact revision, owner/room/instruction lineage
and matching original publication restrictions. The new developer uses original
scope; the completed verifier remains read-only. Legacy failed local-only
recovery remains supported. No acceptance or source verification is waived.

Tests exercise both originally allowed and prohibited deployment, missing native
checks/report/unchanged evidence, running/canceled inspection and changed revision.
They recover the saved decision twice, verify one queued continuation, claim the
actual separate developer attempt and assert it retains the original deployment
limits without typed inspection/verifier intent. Original timeout history remains
failed. Targeted legacy recovery, changed/canceled-source and manager-handoff
checks pass. Production recovery remains to be observed after release.

Release `8d8647c18aa1ad2c1f1c9086e7c78a61a1e891a5` completed at 12:47:08 UTC
on November 9, exit zero, source unchanged and production health matching the
exact commit. It includes the negative-inspection handoff, complete-redraw
diagnostic grouping and the opt-in backup-maintenance installer. The persisted
backup count remained 3 and the installed unit still invokes pinned backup
retention; observed available bytes were 6,132,158,464 after release activity.

The existing saved PM decision automatically admitted separate developer
attempt `att_f4563b9240647c044af2476e8e14293d`, created at 20:45:28 (+08) and
started at 20:45:45 during activation. Official execution diagnostics report
ordinary `execution` intent with original read-only/push/deploy restrictions all
false, no typed inspection/verifier intent, and the same task/room. No new chat
send, retry, manual status update or additional steering was used for this
handoff. Prior source timeout and successful negative inspection remain intact.
This proves recovery of the stranded decision, not completion of the still-
running dependency optimization or fully autonomous readiness.

## Steering transport could acknowledge a write without PTY delivery

Fural's AGY seed uses a text write followed by CR Enter after 120 ms. Cuan's
interactive and detached steering paths sent LF; its terminal split a trailing
keypress after only 25 ms. These are distinct keys in a raw TUI, so this was a
concrete mismatch, not proof of how a particular old model turn behaved.

A new real raw-PTY test then reproduced another failure even with CR: the
socket-only sender closed immediately after writing, while the bridge first
wrote retained scrollback. A peer close could terminate that history write
before the bridge read stdin. Existing socket tests checked only that the
sender returned no error and did not establish delivery. Thus an earlier
`delivered` acknowledgement is not evidence that the provider received the
checkpoint request; the actual contribution to the original timeout is unknown.

Steering now sends CR in all three paths. The shared terminal input writer uses
a separate Enter write after 120 ms. Socket steering sends a framed stdin
request with a random correlation ID, drains the history stream and waits for
the matching acknowledgement after the full PTY input write. It rejects failed,
missing or foreign acknowledgements and never resends automatically on uncertain
observation. Old detached workers may receive the framed input but cannot issue
this acknowledgement; keep that uncertainty visible until those sessions end.

The real Linux PTY fixture uses noncanonical input with CR/LF translation
disabled, accepts only CR as submit, and verifies exact multibyte/multiline input
including leading whitespace and literal JSON-shaped text through the actual
detached socket bridge. Missing/foreign/rejected receipt tests pass. The complete
terminal package and targeted detached-steering, heartbeat and final-report-stop
runner regressions pass. This fixes transport semantics, not an AI-consumption
receipt or proof of autonomous task completion. Production rollout is pending.

### Independent counterexample after dependency-reuse verifier pass

Production release of root commit `2bdd54c8995e44dd4f495db9c06d1e9948f80218`
finished 2026-11-09 13:14:55Z after 198.1 seconds with exact health and 38
migrations. This does not publish native task source `fe821357daa5ea8d2bcd7a4b9475f25632d8117b`.

Native verifier `att_f6c79b2e890788f9015ed8fdd0a0f7e4` succeeded at
21:18:36 +08 and accepted isolation based on existing tests. Root independently
read dependency_reuse.go from that exact Git commit through registered SSH and
ran its unchanged functions in a temporary Go package. Counterexample:
create dev/dependency.js containing `original`; call seedWarmDirectory(dev,
verifier, "fingerprint"); os.WriteFile verifier/dependency.js with
`verifier-write`; read dev/dependency.js. TestSeedMustNotMutateDeveloper FAILED
in 0.008s: developer content became `verifier-write`. The os.Link fast path
shares writable inodes. Existing symlink tests do not establish isolation.
No application source, real dependency tree or task workspace was changed by
this fixture. The verifier pass is therefore insufficient for the original DoD.

Native release attempts 5 and 6 both failed before any model/release execution:
`Native release workspace preparation is unavailable; no model or release
execution was started`. This is a separate preparation failure, not proof of a
production deployment failure. Correction evidence was sent once to the original
room through official CLI; CLI timed out but room read confirms saved message
`a2d31ae7-6ea2-4e6f-9765-8bfef2f493a3`, request
`038281e7-d9e0-41b5-a9dc-405cb672e22a` queued. No resend, manual Done or
publication of the rejected optimization. Corrective dev and independent
verification remain pending; no claim of end-to-end speedup or autonomous Done.

Preparation diagnosis follow-up: several preflight/runtime error responses had no
machine-readable code, so the runner collapsed them into the same generic
unavailable message. Added bounded native stage codes for those existing branches
and recognition on both 409 and 503; raw server/provider text is never copied.
Runner transport tests pass, including all newly named 503 stages. The real
server missing-publication-config fixture passes with the exact code assertion.
Initial server compilation was killed by memory pressure; retry with GOGC=20,
GOMAXPROCS=2 and -p 1 passed. This diagnoses future failures; it does not establish
the lost historical preparation cause or fix it by assumption.
Native corrective developer `att_dea6ef01e90f6b80a1dcc907308bf6f9` is now running
on the original task; official logs confirm it is examining the hardlink issue.

Diagnosis release `5ca49aa0fe86ac2fe32cc59001e771c867e5a04a` was pushed
and deployed through official `cuan release run`; exact production health matched,
38 migrations, exit 0, source unchanged. Started 13:28:14Z, finished 13:31:32Z,
198.2 seconds. Full initial native preparation scenario group subsequently passed
(16.998s), covering approved merge/reuse, conflict, revoked/wrong source, missing
publication configuration/runtime/access and live-observation cases. Corrective
dev remained running after deploy; no forced acceptance or restart was requested.

Verifier instruction correction: the real false-positive isolation review showed
that the generic existing instruction (developer claims are not proof) was not
sufficient. The native verifier role now explicitly inspects what passing tests
cover and uses the smallest relevant counterexample when a material guarantee is
uncovered, with fixtures outside the protected checkout. Isolation is explained
as observing independence under mutation rather than different paths. It forbids
a duplicate full suite or unrelated checks. This is a prompt correction, not a
claim of proven model compliance; the next independent review must establish
that behavior. Existing exact-source/private-workspace runner regression passes.
Native corrective dev completed source 469f1a93 in 7m3s including 34.179s native
checks; the queued follow-up original-scope correction is running as
att_1762ec8bc1f13ceeac29e6a66052052b. No forced Done or extra task was created.
