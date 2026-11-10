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
