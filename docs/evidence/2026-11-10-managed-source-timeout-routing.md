## Observed failed development and incorrect recovery target

Task `msg_612faba3-721b-4d16-b14a-e8e6b6824f38`, room
`5e7132e6-1ac4-4276-8347-10bb73823e22`, was created through the normal PM
conversation. Developer `att_459135778d16a8eb1a5d446f1d1d2bd0` started at
08:06:11.379 +08 and timed out at 08:31:11.697, exit 124. It retained source
edits and an untracked regression fixture in its existing task workspace, but
had no final report, native project-check result, source handoff, or publication.
Do not credit that attempt as a successful implementation or verification.

The runner recorded canonical admission
`managed-release-claim:att_459135778d16a8eb1a5d446f1d1d2bd0` at
08:06:12.106 +08. It binds the original task/project/room, runner-01,
instructions hash, buchin/cuan and dev. The admitted developer's prompt forbids
push and deployment: publication belongs to the subsequent native publisher.
The original task permits publication; these are different scopes.

Recovery nevertheless queued `launch_9988cc16a6cb732eae119d8a285e85b0a1e8419e02c601698a229fe6cfeb07df`
as `current_outcome_inspection`, read-only/deny push/deploy. At 08:31:23 its
diagnostic reported storage admission blocked. No inspection attempt had started
when this was observed. No retry, cancellation or manual acceptance was applied.

## Routing explanation and required correction

`planLiveResultInspection` treats an execution timeout with no result commit as
eligible to inspect the currently running revision. Its local-only exclusion
uses `AttemptPublicationPolicy`, which describes original approved publication
limits but does not incorporate the separate native managed developer admission.
Consequently this known incomplete source stage enters an existing-release audit
before semantic PM review can send the saved source back for completion.

Inspecting production cannot verify the developer's uncommitted patch. Unknown
effects in ordinary or legacy execution can still justify the existing bounded
current-outcome observation; they must not be globally disabled. The correction
must distinguish authenticated managed source-stage ownership, missing final
source/handoff and actual publication evidence, retain cancellation/owner/scope
checks, and send unfinished work through existing saved PM review/continuation.
It must not blindly rerun external effects, grant new approval, waive original
criteria, or keep retrying a timeout indefinitely.

The queued legacy recovery also needs reconciliation at an actual admission
boundary; changing only the planner will not invalidate an already saved launch.
Preserve the failed attempt, edits and diagnostic history. Prove a fresh managed
source timeout avoids an unrelated production inspection and resumes the retained
workspace through the approved bounded correction path. Missing/foreign claims
must not be treated as authenticated managed ownership. This gap remains open.

## Correction and targeted verification

The canonical managed admission parser is now shared in the task layer; runner
completion retains its existing current-project and publication-scope validation.
The planner rejects a known managed source timeout without a result commit before
observing production. `ValidateLiveInspectionSource` enforces the same boundary
for already-saved launches, including launch admission after a server update.
This does not directly enqueue another developer or accept an incomplete result:
the existing PM review/correction path retains original approval and retry limits.

Targeted tests exercise the planner, actual saved-source validation, malformed
and foreign records, ordinary unknown-effect timeouts, existing admission
integrity, and ordinary inspection lifecycle. All passed:

`GOGC=20 GOMAXPROCS=2 go test -p 1 ./internal/task ./internal/runner ./internal/server -run 'TestManagedSourceTimeout|TestManagedRelease|TestCurrentOutcomeSourceRejectsLocalOnlyPublicationScope|TestLiveResultInspectionLifecycle' -count=1`

No current task status, saved launch, source files or failed attempt was manually
rewritten. Production publication and actual recovery behavior remain to be
observed; local tests are not end-to-end proof.

## Production routing observation

Native release `269b8f86d30f40d2c880a3bb96b2818555b1b7b3` finished
08:41:27.730 +08, exit zero, exact healthy SHA and source unchanged. Runner
undrain/restart succeeded. The existing invalid inspection launch stopped
automatically with its saved-work-preserved diagnostic at 08:40:03; no
inspection attempt ran. The read-only planner then reported
`managed_source_timeout_requires_completion` for the original failed developer.

Ordinary PM review `de0beca7-8879-4130-a208-fc9149a352d6` independently selected
`revise`, `unfinished`, `remaining_work=implementation`, `manager_action=continue`
at 08:42:00.249. It queued existing developer continuation
`launch_0d15a76cd7b0d4c1a2163c09b949b7767d656d2b6da916e0dcb48b086b1a463b`.
No manual launch cancellation, task status change, retry or acceptance was used.
This proves routing correction, not successful completion of the retained task.

The PM explanation incorrectly asserted no code or tests had been written.
Read-only OS/Git observations showed four modified source files and an untracked
regression fixture. Missing final-report/native-check evidence cannot establish
an empty workspace. Review instructions now explicitly preserve that distinction
and require continuations to inspect saved edits before repeating work. Existing
`TestReviewInstruction*` checks pass. This prompt change awaits publication.

The new developer launch remained storage-blocked at about 1.96 billion available
bytes. Registered maintenance was explicitly triggered through `cuan maintenance
run`; it is not an untouched autonomy benchmark. Managed Go build cache was
already almost empty (12 KiB). The host package manager's `apt-get distclean`
removed regenerable package index cache; a subsequent observation showed
2,291,437,568 available bytes. Maintenance was concurrent, so do not attribute the
entire observed difference solely to that command. The admission floor and
source retention were not changed. Confirm actual continuation startup rather
than treating available space as proof of admission.

## Retained-work continuation and bounded steering

Continuation `att_0d15a76cd7b0d4c1a2163c09b949b776` started automatically at
08:44:54.755 +08 with the same task workspace. Read-only Git observations confirmed
the four edited source files and untracked regression fixture survived. Its HEAD
was still the retained `3f867886`; baseline reconciliation remains developer work.

The source-regression fixture remained the active implementation bottleneck. Root
sent one diagnostic checkpoint through official `cuan task steer`, pointing to
the existing native source-review/publication fixtures and their predecessor,
request-ID and timestamp contracts. It explicitly preserved guards, scope,
independent verification and native publication ownership. The command reported
`delivered`, `fallback=false`, and did not create a new attempt. This is root
steering, so subsequent completion cannot be advertised as untouched autonomy.

Later OS observations showed the registered worker and AGY harness alive, with
no Go/compiler or command subprocess active. Saved terminal output had not
advanced beyond 08:54:55. That establishes a quiet live harness, not a proven
provider failure, completed test, or authority to restart it. Follow the same
attempt and its native deadline; retain unknown outcomes honestly.

## Diagnostic fixture reproduction and safe rejection reasons

Root captured the retained diff and untracked fixture through registered read-only
Git inspection, then applied that snapshot to an isolated local worktree based on
`3f867886`. The worker workspace was not modified. The focused
`TestNativeReleaseInspectionEvidenceReuseAndFullAudit` failed before checking reuse:
publisher creation omitted `ExpectedPredecessorID`. Correcting only the local
diagnostic copy exposed invalid delivery/setup evidence. A second brief checkpoint
reported those concrete failures through official steering at 09:07:40 +08.

A later read-only observation showed the worker's fixture changed at 09:06:03
despite absent new terminal output. Therefore quiet saved terminal logs and a
momentary absence of Go subprocesses did not establish a stalled worker/provider.
Retained work changed independently of that log view. No runtime restart was
performed based on silence.

The updated captured fixture passed predecessor creation but still failed native
delivery. Instrumentation confined to the local diagnostic copy located native
attempt timeline validation (a backdated start precedes service-assigned creation)
and deployment-observation validation. These failures remain failures; diagnostic
edits in the copy are not a passing product test or acceptance evidence.

Native delivery validation previously returned the same generic invalid-input
error for these distinct boundaries. It now returns fixed reason codes for attempt
timeline/state, receipt scope/timeline, preparation/delivery shape or absence,
identity/timeline mismatches, activation observation and post-process observation.
The inspection validator preserves only these safe typed reasons. No guard was
relaxed; storage/backend errors are not exposed through the new reason channel.
`errors.Is(err, ErrInvalidTaskInput)` remains true, and messages contain no
source IDs, paths, task data or credentials.

Existing source/preparation/live-proof cases plus a start-before-creation case
assert both rejection and the precise reason, including propagation through the
actual inspection validator. Targeted task tests pass. This diagnostics correction
awaits publication and does not prove lower end-to-end latency or completion of
the source-reuse task.

## Release gate ordering correction

Publishing diagnostics revision `7ca979c9` first passed all 470 frontend tests
and Go compilation, then its Vite build was killed (exit 137). No production
activation occurred. A retry with a bounded Node heap failed earlier: frontend
tests required `web/dist/assets`, which the interrupted build had removed.
The script ran asset-dependent tests before building their inputs. A prior
release's retained dist could therefore conceal this cold-checkout failure.

The release script now installs locked dependencies, builds the exact current
frontend once, tests those assets, then compiles/packages/publishes. It no longer
copies old production assets or builds the frontend a second time. Default Node
heap is bounded at 512 MiB while an explicit existing NODE_OPTIONS remains honored.
No test or source/health gate is skipped. The earlier kill alone does not prove
its external resource cause.

The release integration fixture now makes its test stub require newly generated
assets. A clean-checkout case verifies build/test order, one build and publication
of that output; build-failure and test-failure cases both reject publication before
Go compilation. All seven `scripts/test-release-build-gate.py` cases pass. Actual
publication remains pending until a successful exact-SHA release observation.

## Support corrections published; main task still incomplete

Release `d167292577ed4f3134431c0895b0bbcb9f426a27` completed at
09:28:30.594 +08 (196.787 seconds), exit zero, source unchanged. Production
health matched that exact SHA, with38 migrations; runner restart and undrain
succeeded. It includes the PM uncertainty wording, safe delivery reason codes,
and the single fresh frontend build before asset-dependent tests. This successful
release does not establish completion of the separate source-reuse implementation.

The second developer `att_0d15a76cd7b0d4c1a2163c09b949b776` timed out at
09:19:55.184 +08, exit124, without final source handoff or native checks. Its
retained edits are not absent merely because that evidence is missing.
PM review at09:21:11.917 chose replan/implementation and automatically launched
`att_46bc2577b421832671399f8416045850` at09:21:14.756. That review preceded
publication of the corrected PM wording. The third attempt is active, not
verified, with the original criteria and independent verification still required.

## Valid developer handoff after baseline reconciliation

Third developer attempt produced clean checkpoint
`4a6a8591df05be44d20897e2dac5a7a131179247` and a final review report, but failed
the native pinned-baseline ancestry check before registered tests. This was not
a verifier rejection or absent implementation. PM automatically requested only
baseline reconciliation, preserving original criteria and work.

Fourth developer `att_4e2976f682882b06a89139e649f6b0af` reconciled against
`564859b8a05e7aab342b17372c512e628f0f0b1c`, producing clean checkpoint
`3cb570d34e288918974b94b0585b64ea7244924c`. It succeeded at
09:55:28.010 +08; registered native project checks passed in58.938 seconds.
Independent review and production outcome remain pending, not waived.

Its exact-source verifier was storage-blocked. Automatic maintenance cleared the
idle managed Go cache, leaving2,140,942,336 available bytes, approximately6.24MiB
below the unchanged2GiB floor. Root confirmed no Go build process was active,
then used Go's official cleaner for the separate idle default build cache at
`/home/ubuntu/.cache/go-build`. That cache fell from250,048KiB to12KiB;
available space became2,395,947,008 bytes. No modules, source, Git objects,
logs or receipts were removed. This is another explicit infrastructure recovery,
not an untouched autonomous benchmark.

The same queued exact-source verifier
`att_0cc53b38923b40b35a1f6038ca5e71cb` started automatically at
09:59:02.764 +08 with read-only/deny push/deploy scope, preserving the developer
source and existing launch identity. No task retry or acceptance was fabricated.

## Publication, live inspection and measured duration (continuation)

Independent source review `att_0cc53b38923b40b35a1f6038ca5e71cb` succeeded
at10:06:03.171 +08. Native publisher `att_2fe39d1eca6804df5f0272f0aab75ae6`
ran10:07:42.770–10:17:49.720, publishing healthy exact release
`bf5a602b1586ed8c7130c365d7fbafaf8e1a0a75` containing approved source
`3cb570d34e288918974b94b0585b64ea7244924c`.

Elapsed developer attempts were25m,35m,21m15s and10m25s: approximately92m,
not primarily independent verifier time. Early work grew an oversized publisher
fixture instead of extending existing protocol fixtures; the third handoff then
failed pinned baseline ancestry. Root sent three scoped steering interventions.
This run is therefore not an untouched autonomy benchmark. The actual final
implementation reused existing fixtures and evidence machinery.

Post-publication inspection waited approximately20m for disk admission. With
explicit owner authorization, root deleted74 closed single-link Nginx gzip
archives older24h, after reader and file identity checks, reclaiming472,831,397
bytes. Active logs and the newest24h were preserved. Available bytes increased
to2,200,768,512; the same queued inspection started automatically. Failed earlier
off-host archive copies are not preservation evidence. No source, Git objects,
receipt/history or protected48h source retention was deleted or shortened.
The existing hourly logrotate timer now uses daily rotation plus maxsize20M,
rotate7 and maxage1, preserving existing compression, ownership and reopen hooks.
Candidate native debug validation passed before atomic installation with backup.
At10:46 +08 the last timer run still predated this change; its next execution
was11:00, so automatic execution of the new policy is not yet established.

Live attempt `att_9359b2d3924ed579fa5badf5236e86e3` started10:38:22.809.
Frontend dependency preparation and native visual verification occupied roughly
the first123s despite the original backend/no-browser scope. The exclusion
parser handles comma lists but omitted slash separators between UI and browser
in `Tanpa UI/browser/email`. A minimal delimiter correction plus policy and
server-binding regressions passes locally. It preserves later affirmative
requirements and genuine visual work; it does not remove registered checks.
This correction is not yet published and cannot retroactively alter this attempt.

The live model stage returned in174.177s at10:43:19.772, reporting four passing
criteria. Native checks then began; OS observation confirmed an actual Go test
process in the exact inspection snapshot. A model report or quiet output alone
is not final acceptance. Comparing174s against the previous243s model stage is
only an observation across different runs, not a controlled end-to-end speedup.

The final native check did not pass. It stopped at10:48:20.518 after300.527s;
config/auth/db reported success, but runner had no completed result. The attempt
was correctly saved failed, not Done, at10:48:20.571. OS observation during checks
confirmed the real Go process and owner cache settings. This is a native check
timeout, not proof of failed assertions or absent implementation; its compilation
versus test-execution split remains unmeasured. Preserve this failure and do not
replace it with the model's PASS prose. Automatic maintenance was observed
running after failure at10:48:33; the managed build cache was then12KiB. That
post-failure observation alone does not establish deletion during active checks.

## Follow-up: distinguish command timeout and bound compiler scratch

Release `e9fe692f97b12ea6de2c9488f1010041cf071dca` completed healthy at
10:54:09.848 +08, exact-SHA health matched,38 migrations, runner restarted and
undrained. It includes the slash-list scope correction; that is not acceptance
of the original failed task. Subsequent PM reviews failed their response contract
and bounded format repair; no new saved acceptance assessment was observed.
The earlier unnecessary native screenshots also became PM image inputs, adding
avoidable review work. Do not infer that their presence makes this backend task UI.

The11:00 native logrotate timer executed successfully at11:00:09 +08, establishing
that the already configured hourly automation runs. No new scheduler was created.
Two old, closed Go compiler work trees under `/tmp`, last modified05:53 and08:49,
were then removed only after1h age, expected generated `bNNN` layout, no open
readers and file identity checks. Their file sizes totaled257,057,838 bytes;
available disk increased to2,293,084,160. These were disposable compiler scratch,
not task sources, caches, Git objects, history or receipts.

A read-only OS diagnostic of the exact failed inspection snapshot started with
available disk above the unchanged2GiB floor, same Go settings and owner-scoped
cache under the existing shared build-cache lease. It runs the same fast package
set with JSON timing, bounded300s. It is diagnosis only, never a native task
acceptance receipt. At132s only config had completed, while the process remained
live; the full compile/test breakdown is still pending. The same fast suite on
the local host passed, with runner18.515s (other packages cached); this is not a
controlled speedup comparison between hosts.

The native wrapper currently replaces a command's five-minute timeout with the
same generic inspection failure as an assertion failure. The follow-up correction
retains a distinct native timeout marker and routes it through the existing single
infrastructure recovery. Its saved final report, runner identity, failed exit,
read-only boundary and absence of actual assertion failures remain required.
Cancellation or actual assertion findings cannot borrow that recovery. Native
summary output retains the precise termination reason rather than only exit-1.

The command also owns a separate `GOTMPDIR` scratch directory and removes it after
its compiler descendants stop, on success, assertion failure and timeout. Shared
build/module caches remain intact. Real process fixtures exercise all three exit
paths, verify scratch removal and preserve a separate cached object. Targeted
runner/task/server regressions and the full registered fast Go package set pass
locally. This follow-up is not yet deployed.

## Follow-up production and diagnostic outcome

Release `72f3e2830dbbdbcacf861ca59ce64751f6367c30` finished at11:11:02.394 +08,
exit0, exact-SHA healthy,38 migrations, source unchanged, runner restarted and
undrained. Timeout-cause classification and per-command compiler scratch cleanup
are now deployed. They do not retroactively accept or rewrite the failed attempt.

The bounded diagnostic completed all five packages successfully. JSON package
execution times: config0.016s, auth2.664s, db9.859s, runner36.980s,
extensions0.132s; total package execution49.651s. Diagnostic log birth-to-last-write
was approximately282.876s (11:02:16.163–11:06:59.039). The difference includes
compilation/linking, setup, scheduling and process overhead; it is not a precise
measurement of compilation CPU time. This confirms that an actual assertion
failure is not necessary to explain the earlier300s native check timeout.
The diagnostic is not an independent task receipt and gives no acceptance credit.

Root sent one additional continuation message (room message18, request
`eaf33f74-c2fd-4309-84e4-7cbca71bb79c`) through the official CLI, preserving the
original task/criteria and explicitly denying acceptance based on this diagnostic.
The send observation timed out, but inspection confirmed receipt; it was not resent.
This is another manual workflow intervention, not autonomous benchmark success.
That request failed its action response contract after115.9s; bounded format-only
repair then timed out after44.1s at11:14:27.984. No new accepted PM assessment or
replacement task execution was observed. This is a separate concrete blocker in
coordinator response generation/validation; repeatedly resending the request would
not establish recovery. Original task remains incomplete.

## Coordinator rejection diagnostics (pending runtime observation)

The failed PM continuation exposed a trace gap: action rejection collapsed to
`action payload or response contract rejected`, while snapshot reviews only said
the review contract failed. The rejected candidate is not retained in the public
trace, so these records do not establish its exact invalid field. Do not claim a
specific payload bug from that generic label.

Add fixed rejection categories for unsupported action kinds, missing payload,
task-reopen field/type or identity errors, reply/title transport limits, embedded
markers/local URLs, and common review decision/continuation conflicts. Feed the
same category into candidate-only format repair. No provider values, candidate
findings, paths or customer data are added to trace. Native decoding, action
permissions, original scope and acceptance guards remain unchanged. Tests confirm
invalid fixtures stay rejected, labels omit sentinel private data and each repair
prompt receives the actual category. Targeted diagnostic, delegation, choice and
review format-repair tests pass. A future native trace must establish which
category the real failed workflow produces; these fixtures alone do not prove it.

Contract diagnostics release `055d0ce6739ed08b378d22c63ebdfd771a7488a4`
completed healthy at11:23:54.670 +08, exact-SHA match, runner restarted/undrained.
Official retry of the failed continuation created the deterministic child
`52832ab5-872b-01ba-f9c4-d2aaa0b5aa05`. Trace confirmed native retry_reconcile,
then a live provider process; it ended at11:26:31.948 with signal9 after124.1s,
without a final result or saved application action. It therefore did not exercise
the new rejection categories; do not misreport an observed payload error.

Code inspection also found a concrete remaining schema mismatch in that retry
path: its prompt defines closed reconciliation payload fields, but the actual
provider `--json-schema` still used a free-form object. A real CLI-boundary fixture
now requires the typed payload in the actual schema argument. Against the old
transport, valid answer and reconciliation cases fail; sharing the existing
closed reconciliation schema makes both pass. Tools, unrelated actions and
unscoped delegation stay rejected. This fixes the inconsistent contract without
changing permissions or extending inference budgets. Retry/scope/format targeted
tests pass locally. Runtime improvement for this correction remains unproven.

## Native post-publication incomplete-check recovery

Current-state diagnosis found a separate routing gap: existing infrastructure
recovery covers current-outcome inspection, but native post-publication inspection
only recognizes pre-model stale targets and missing final-report deadlines. A
saved final report followed by an aborted native project check therefore remained
stranded even after its command-timeout marker was corrected for future runs.

Reuse the existing verifier-recovery identity and one-attempt bound for native
post-publication inspections. A legacy generic inspection failure qualifies only
with native runner-authored exit-1 check evidence, a saved implementation report,
matching repository observation and no assertion findings. This establishes an
incomplete/aborted command, not a historical timeout diagnosis. Positive failed
assertion exit, changed/foreign observation, cancellation and missing report still
reject recovery. Original source/delivery/owner/room/approval lineage is unchanged.
A changed running release must contain the old delivered target; native snapshot
checks remain mandatory. The failed attempt is retained and acceptance gets no
credit. Existing reporting recovery retains its own already-bounded semantics.

Tests extend the existing native-delivery fixtures rather than creating another
publisher framework. The real service completion/queue/claim fixture confirms
same-target aborted-check recovery, deduplication, original predecessor and
read-only scope. Task protocol variants cover changed target, typed timeout,
legacy abort and rejection of assertions, foreign records, cancellation and changed
repository state. Required targeted task/server suites pass (server22.035s).

A broader existing visual-evidence continuation test was also run and failed
intermittently: its saved assessment digest no longer matches the task snapshot.
The same failure reproduces on the unchanged production commit in a separate
worktree, including3-run baseline inspection; a single run can pass. Temporary
diagnostic instrumentation was removed. This is an unresolved pre-existing
review-snapshot issue, not a passing broad suite and not justification to weaken
source/visual acceptance. It remains follow-up work after the active backend task.

## Native recovery published and admitted (not accepted)

Two support publication attempts stopped with exit137 while rendering frontend
chunks, before deployment; production stayed unchanged. A second attempt with a
384MiB Node heap also stopped. OS inspection identified an idle local Cuan Vite
preview and its esbuild child, including a deleted dependency executable retained
by that preview. Root stopped that confirmed local preview, releasing its memory;
no remote application was stopped. The subsequent complete native release with
384MiB heap passed and published healthy exact SHA
`464fcbd0f0ee482efa2d4d2f8722c5e877d2b814` at11:58:26.461 +08, source unchanged,
runner restarted and undrained. No tests, fresh asset build or health gate skipped.

The existing delivery scanner automatically saved recovery launch
`launch_85e538972d10e63ce307930ef548235d90cdc4ac9945575e460f5ecbbc9f13e1`.
Root neither resent a PM message nor manually changed the task status. Disk
admission still blocked its dispatch. Root removed two more closed Go compiler
scratch trees, last changed10:48 and10:42, now older1h, with generated layout,
reader and identity checks; file bytes totaled237,998,177. Native journal vacuum
removed94.6MiB of archived journals while preserving active journals with a100MiB vacuum target. The existing12:00 Nginx logrotate timer also completed successfully.
Available disk became2,266,951,680 bytes. Source48h retention, independent Git,
receipts/history and the2GiB admission floor remained unchanged.

The same launch automatically started attempt
`att_85e538972d10e63ce307930ef548235d` at12:00:34.845 +08, with deadline12:10:34.845.
It retains original source3cb570d3, publisher att_2fe39d1e and running target464fcbd0,
read-only repository, deny push/deploy. The agent stage began12:00:38.962, only
4.117s after start, with no frontend preparation or visual recipe in the initial
native stage records. The previous same-task inspection spent approximately123s
before its agent stage. This verifies removal of that preparation waste, not
end-to-end task completion or final acceptance. Outcome and final native checks
are still pending.

The recovered agent stage subsequently timed out at12:05:37.719 after298.756s,
before a final report. The attempt remained failed, exit124. Actual OS observation
showed a Go check for `github.com/buchin/cuan/internal/task`, not the registered
fast suite or `./...`; shared cache variables were inherited. Its completion is
unproven. New native observation records confirm the snapshot was unchanged;
registered checks had not run (empty native summary). The existing single
reporting recovery was automatically queued, retaining both previous failures.

A171,493,598-byte generated Go compiler work tree remained after that model-stage
process stopped. Root confirmed no Go process/readers remained, validated generated
layout/identity and deleted this known terminated execution's scratch. This was
another explicit infrastructure intervention. The earlier deployed scratch fix
covers runner-owned project-check commands; model-started Go commands still need
an execution-owned scratch lifecycle. Do not claim the entire compiler-scratch
problem is solved or shorten source retention to recover these bytes.

After disk availability recovered to2,212,904,960 bytes, that same reporting
recovery automatically started `att_7da2bea1bd728767f0cbea6452bca2b0` at
12:08:44.781 +08, deadline12:18:44.781. Its reporting/outcome and final native
checks are still pending. Do not restart it from quiet output or count the prior
model report/diagnostic as acceptance.

## Reporting recovery exhausted; model compiler scratch fix prepared

Reporting recovery att_7da2bea1 ran until12:13:45.863 +08 and again reached its
agent-stage deadline after300.240s without a final report. It remains failed,
exit124; no further reporting retry is granted. OS observation during that stage
again found `go test github.com/buchin/cuan/internal/task` with the same owner
cache, and actual compiler descendants (rather than a running task.test binary).
The same full task package on the local host completed in0.180s with caches.
This supports compilation/cache cost as a bottleneck; it does not measure exact
VPS compilation duration or prove its unseen check result. Native final checks and
PM acceptance remain absent, so original task is still incomplete.

A legacy-provider module-cache pruning dry run found zero eligible module caches
and removed nothing. No module cache deletion or lower admission/source-retention
limit was used to manufacture capacity. The latest inspected snapshot already has
one packed Git object file, zero loose objects and no reclaimable garbage; do not
claim repacking it would recover meaningful disk space.

The remaining scratch lifecycle is now corrected locally for model-started Go
commands: managed Go executions receive an execution-owned GOTMPDIR, removed before
completion delivery and lease release. An explicit project GOTMPDIR is preserved.
The lifecycle test exercises successful execution, deadline-error return and custom
scratch; its completion HTTP handler observes managed scratch already absent while
custom scratch survives. Existing native fast package tests pass (runner18.015s),
and shared owner cache metadata/release tests remain intact. This correction is
not yet deployed. It prevents another scratch leak but does not itself prove the
original source-reuse feature or solve cache churn under persistent disk pressure.
