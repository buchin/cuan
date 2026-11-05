# Recovering unfinished task reviews

## Inspecting an implementation already present in production

A retained implementation whose final report was followed by `Project verification
failed` can receive one automatic read-only inspection per source commit and
approved instructions hash. This requires the original authorization, a finished
exit-zero noncanceled source attempt, reported acceptance evidence, and a running
build whose embedded repository matches the registered project. Canceled tasks
and attempts, missing source commits, incomplete reported criteria, prohibited
tests and investigation-only scopes are excluded.

The inspection uses a separate detached snapshot of the exact running commit.
Git verifies that it contains the task source commit. Its object store is private;
bounded history avoids copying unrelated cache objects. Missing revisions are
fetched only through the project's registered connection. Preparation preserves
the old task's files, index and refs. Dirty or unowned snapshots are rejected,
never reset. Old runners skip this execution mode before reserving it.

The native runner verifies snapshot identity before execution, strips project
environment and automatic deployment, and enforces the read-only fingerprint.
After a final report it independently runs the fixed project checks and checks
the fingerprint again. A typed receipt binds ancestry, exact result commit and
fresh check summary to this attempt. A successful inspection enters Review,
never Done directly. Independent review still checks every original criterion,
feature evidence and any required visual/publication evidence; the running build
does not prove that this task deployed anything. A changed running release or
missing receipt rejects acceptance even when the review model says pass.

The original failed attempt remains failed. The durable source/scope key prevents
deployments or restarts from creating repeated inspections. If the first live
inspection itself finished with a final report but failed its fresh fixed project
checks, one further inspection is available after the running commit changes.
It retains the original implementation attempt and binds the failed inspection
and its target commit. Saved runner-authored failed-check evidence is required;
missing reports, cancellation and other failures do not qualify. The native
runner verifies that both the original source and the previous inspected commit
are ancestors of the new snapshot target. Older workers skip this follow-up
before reservation. Its separate durable key allows at most one such recovery
per source/scope, regardless of later deployments; it never resets correction
budgets or rewrites old failures. Original criteria and independent acceptance
remain mandatory.

Immutable snapshot or
authorization failures stop rather than remaining queued forever. No production
task needs to be manually resumed to activate this reconciliation.

Targeted coverage: `TestLiveResultInspectionLifecycle`,
`TestLiveResultInspectionPostgres`, native runner inspection tests, and project/Git
snapshot tests. The notice is rendered by `review-outcome-check.cjs` at 390px and
1440px; use `CUAN_REVIEW_VERIFYING=1` and `CUAN_REVIEW_OUTCOME` to inspect its text.

A completed provider response is not necessarily an applied task decision.
Historical reviews sometimes produced only a revision message, or stopped because
of quota/timeout. The task then stayed in Review even after capacity recovered.

The dispatcher now reconciles eligible terminal reviews from the current task
snapshot. It does not replay the historical provider prompt, tools, user message
or execution attempt. The synthetic review is bound to the exact latest attempt
and current evidence digest and runs under the existing tool-free review schema.
A parsed outcome still passes execution, scope and acceptance guards before it
can mark work accepted or delegate a correction within the original approval.

Recovery requires the saved original user message and accepted Owner identity.
Missing/canceled authorization, canceled work, active attempts and pending launches
are excluded. A legacy completed review without the current evidence marker gets
one fresh evaluation. Failed/interrupted reviews get the same fresh snapshot path,
with at most one further retry for unchanged evidence. Deterministic request IDs
and atomic request acceptance deduplicate concurrent scans and restarts. The old
request and its terminal state remain as history.

Administrative status changes do not reset this budget. New actual report,
checklist or runner verification evidence has a new digest and can be evaluated
again. Current completed reviews with unchanged evidence are not repeated. This
policy does not reset the execution correction budget or invent a new approval.
Capacity waits keep using the existing queued/backoff behavior.

## Verification

`go test ./internal/server -run 'TestStalledReview|TestInbox.*Review|TestApplyInboxReview|TestPMReview|TestTaskCancel|TestSystemReviewCannot'`

Fixtures cover concurrent scans, failed/interrupted/legacy-completed source
reviews, current snapshot binding, applied acceptance, bounded retry, new versus
administrative evidence changes, missing/canceled authorization, cancellation and
new execution. No production task status or message is changed by the tests.

A reply turn checks at most8 account/model routes. Exhausting that bounded check
while allowed routes remain is a capacity wait, not a permanent failure. The same
request stays queued with backoff. Persisted cooldowns let the next turn reach
remaining routes without repeating the failed ones. Genuine non-quota errors and
unknown tool effects retain their existing failure/safety behavior.
`TestInboxBoundedQuotaRouteCheckKeepsWorkPending` uses10 explicit model routes:
8 fail with quota, the request retains the queued sentinel, and the next turn
succeeds on a previously untried model. No task execution is replayed.

## Launch persistence and truthful outcomes

A tool-free review must not disappear when its correction launch cannot be saved.
The review application observes typed launch-preparation/write errors through a
local context value. Transient errors retain the same Inbox request in `queued`
with the existing bounded backoff and 24-hour deadline. No final reply is stored.
The next evaluation rechecks current approval, task identity, cancellation, scope
and correction budget; the deterministic launch key deduplicates a write whose
outcome was uncertain. This can evaluate the review model again, but does not
replay execution tools or the original user turn.

This retry signal is installed only while applying a validated review correction.
Ordinary chat/tool turns are not automatically replayed after a launch failure.
Request conflicts, invalid launch input and a deleted task are terminal outcomes,
not transient storage failures. Once a launch exists, its existing durable worker
owns pending execution recovery.

Continuation prose is shown only with the exact review launch saved as queued,
claimed or started. Failed admission instead shows its actual result. Exhausting
the correction budget produces a concise stopped outcome, even if the model
promised to continue. This does not raise the retry limit, reopen canceled work,
rewrite old messages, or claim that historical blocked tasks are resolved.

Targeted checks include `TestInboxReviewLaunchStorageOutageKeepsOriginalRequestQueued`
with a real Inbox lifecycle and fixture provider, transient versus permanent
launch failures, recovery under the same request key, no extra execution attempt,
and suppression of a false continuation promise at the correction limit.
`node docs/evidence/review-outcome-check.cjs` renders the stopped outcome and task
card using the real frontend at 390px and 1440px with isolated API fixtures.

## Attempt-scoped report evidence

Task summaries and completion notes survive retries so saved work is not erased.
They are not automatically evidence that a new execution did or verified anything.
Review input now labels the provenance of reported data and the matching attempt
ID. Fallback report selection uses only the current task/attempt, ordered by event
time and ID, rather than whichever report happens to be last in an activity array.
A progress report supplies progress, not inherited final verification.

If a failed/retried execution has no current report, its reported result and
verification stay empty. Retained task fields and the most recent earlier report
are supplied separately as bounded historical reference, with the earlier attempt
ID. A legacy single successful execution without report events keeps its task
snapshot explicitly unbound; it does not receive an invented report attempt ID.
Existing execution, cancellation, scope, deployment and acceptance gates remain
in force. No stored reports, task fields, files or attempts are erased or rewritten.

A completed review that predated this policy and misattributed an older report
gets one fresh tool-free snapshot evaluation under `report-provenance:<digest>`.
The synthetic input carries `Report provenance policy: v1`; a failed/interrupted
snapshot has at most one additional retry. Original authenticated owner approval,
cancellation checks and execution correction limits still apply. Deterministic
keys prevent repeated scans/restarts from creating more reviews for unchanged
state. This does not reset a task's execution budget or restart its old process.
The pre-existing evidence digest remains unchanged, preserving pending phase
handoff bindings and reviews already in flight.

Production inspection on 2026-11-04 found that both remaining blocked attempts
had zero current report events but had earlier reports. The preview task still
had task-level summary and verification fields. This demonstrated misattribution
in review input; it did not establish either attempt's underlying process failure.

Tests cover retained historical context, missing-current reports across terminal
states, progress versus final reports, foreign-task exclusion, stable ordering,
legacy snapshots, canonical fresh-review binding, and one-time reconciliation
without a new execution attempt.

## Saved project acceptance context

Review receives the complete approved task scope (within the task service's
24,000-character input limit), plus the frozen project goal and instructions as
separate fields. It does not receive the raw project snapshot or its environment
credentials. This prevents a long task from silently losing acceptance rules
under the former 9,000-character review truncation. Invalid saved guidance is
reported explicitly and cannot pass acceptance; current settings must not replace
historical approval.

The reviewer must distinguish actual application screenshots and an inspection
report from mockups, HTML reports or layout calculations when visual inspection
is required. Passing project tests alone does not prove that visual requirement.
This is a review instruction, not an independent screenshot authenticity check.
Previously accepted tasks are not reopened or executed by this change.

## Runner verification provenance

Each new runner completion atomically records a `runner_verification` activity
with the exact task and attempt identity, including an empty summary when no
verifier output was produced. Concurrent or repeated completion cannot create a
second accepted record. No schema migration or historical rewrite is required.

Review receives current runner output, its attempt ID and provenance separately
from retained historical task verification. Recovery prompts likewise label
unbound task-level `test_summary` as history and require checking it against the
current approved baseline. An old failed project test is not automatically the
cause of a later exit-2 or timeout failure. Missing or invalid attempt records do
not inherit earlier checks. Legacy summaries remain available as history; this
change does not reopen accepted work or reset correction budgets.

Regression coverage includes empty current output with retained old failure,
foreign-task and previous-attempt records, deterministic ordering, invalid
records, server review/recovery projection and real PostgreSQL concurrent
completion persistence.

## Durable review assessments and visible remaining gaps

Before applying a nonfinal task review, Cuan saves one
`coordinator_review_assessment` activity tied to the review request, task,
attempt, original instructions hash and evidence digest. The assessment records
the decision, factual finding, explanation and concrete revision. It is not task
acceptance: independent execution, repository scope, publication and original
acceptance gates still apply. Repeating the same review uses its stored
assessment rather than replacing it with a different model answer. Changed scope
or evidence cannot inherit that assessment.

If storing the assessment fails, task state and execution remain unchanged; the
existing durable-storage backoff retains the same review request. At an exhausted
correction budget, the user sees the factual missing evidence plus the stopped
outcome. The technical revision remains in task evidence and subsequent recovery
context. The model's explanation cannot promise a new execution in that branch.
Legacy responses without the factual field retain the previous concise fallback.

A successful attempt with its own final report but no saved assessment may get
one fresh authorized tool-free snapshot review and one bounded retry. This
recovers the lost evaluation, not extra correction budget, previous provider
tools or permission to publish. Canceled/retired tasks cannot queue a new review
or be reopened by a stale pass. Known stale cancellation reviews avoid provider
inference and preserve the stopped outcome.

Verification includes persistence failure with unchanged task state, exact-review
replay, changed evidence/scope, exhausted budgets with a concrete gap and no
execution promise, canceled successful attempts, one bounded legacy repair and
real PostgreSQL receipt persistence alongside phase-handoff regressions. The
actual frontend was rendered and inspected at390px and1440px using isolated APIs
with a factual missing-screenshot example; this is UI rendering evidence, not
proof of the original task's visual acceptance.

## Inspecting the current outcome without a historical source commit

An approved managed code task that failed without producing a source commit
may receive one read-only inspection of the exact running release, provided its
registered repository matches that release. Native adaptive/hard execution timeout evidence qualifies. A native project
verification failure (exit0 plus the exact machine failure reason) also qualifies
only with the same attempt's latest final implementation report. Progress-only,
foreign-attempt and non-implementation reports do not qualify. Cancellation, unrelated failures and inspection
chains do not qualify. The request identity binds the original failed attempt
and approved instructions; deploying another release does not reset its budget.
Older runners cannot reserve this inspection.

A legacy `local_terminal` attempt with retained failed status, exit124, an empty
reason and a valid started/finished interval can also receive its first bounded
current-outcome inspection. Its cause stays unknown: exit124 alone is never
relabelled a native timeout or success. A second narrow legacy shape qualifies:
recorded Failed on `local_terminal`, no exit code, finish receipt or error reason,
and valid creation/start timestamps preceding the new observation. Its cause and
stop time remain unknown and are explicitly included in independent review. This
does not repair historical metadata or resume the old process. Other missing
terminal timestamps, different runners, cancellation and known unrelated errors
remain excluded. This uses the
same source-attempt/scope identity and does not reset a previously used inspection
or correction budget. Original authorization and all independent verification
requirements still apply; no historical implementation effects are replayed.

This inspection establishes current behavior, never authorship or ancestry of
unknown historical work. The failed attempt stays failed. The agent must
reproduce the original issue, verify every original criterion, and supply actual
required UI screenshots. It uses a private exact-commit snapshot and cannot
repair code, publish or replay external effects. Locked dependencies and ignored
verification output may be prepared there without modifying tracked source,
lockfiles, index or refs. The native runner independently checks unchanged source
and runs fixed project checks after the final report. Success enters independent
review; current-release identity and green tests alone do not establish feature
correctness or waive any original publication requirement.

Verification claims persist their actual native execution bound atomically with
reservation: at most ten minutes for inspections/verification, thirty seconds
for artifact finalization, and never longer than the ordinary task budget.
Deadline display, completion guards and abandoned-runner reconciliation therefore
share the same immutable deadline. Queue waiting does not consume execution time;
this change does not rewrite deadlines of already claimed historical attempts.

For a current-outcome inspection that requires visual evidence, the runner
prepares locked npm dependencies in the owned snapshot before dispatching the
agent. Only root or `web` manifests with modern npm lockfiles and credential-free
HTTPS package sources qualify. Linked/shared dependency directories, local/Git
sources and embedded credentials are rejected. Installation uses `npm ci` with
lifecycle scripts, audits and funding queries disabled, an isolated temporary npm
home/config/cache, no provider/task credentials and a two-minute bound within the
existing inspection deadline. The runner revalidates the exact clean source
commit afterward. Setup is not feature verification, does not reset recovery
budgets and does not resume an old failed attempt.

Inspection dispatch includes the native snapshot directory and expected HEAD.
Historical diagnostics and reported results remain context, never fresh evidence.
Agents capture actual output and exit status for targeted checks sequentially;
Go test runtime limits do not bound compilation, and elapsed time alone cannot
diagnose a deadlock. Required screenshots must be rendered and inspected against
the current snapshot; copying old screenshots does not verify the current UI.
An unfinished check requires an honest checkpoint with the missing evidence
before the deadline, not acceptance or another automatically restarted attempt.

## Reports saved before an inspection timeout

Live-result and current-outcome inspections use the same bounded staged-file
recovery as ordinary verification. If an inspection times out after saving a
report in its dedicated session output directory, Cuan validates its original
launch binding and restores only that attempt's encrypted artifact grant. The
normal file allowlist, size/count bounds, owner association, cancellation and
scope checks still apply. Symlinks and private files are not delivered.

Recovered files are attached once to the original Inbox conversation and recorded
as `unreviewed`. Their evidence digest permits independent review of the saved
findings. Delivery does not manufacture a final agent report, successful attempt,
passing checks, unchanged-source receipt or current-release acceptance. The old
failed attempt stays failed and no inspection/correction budget is reset.

Coverage: `TestLiveInspectionRecoversStagedReportWithoutCompletion`,
`TestLiveInspectionStagedReportPostgres` and the existing ordinary-verification
staged-output regression tests.

## Revalidating a changed rejected result

A successful current-outcome inspection can still be rejected because original
features are missing. That finding belongs to the inspected commit; it is not
permanent evidence that every later release is missing the same feature.

Cuan permits one additional read-only inspection when all of the following hold:

- The original owner approval remains valid and the task is not canceled/done.
- The first inspection succeeded with runner-bound project checks and an unchanged
  repository receipt, but its saved coordinator assessment requested implementation.
- That assessment names concrete repository source paths, and at least one changed
  between the inspected commit and the currently running revision in the same repo.
- No prior changed-result revalidation exists for that original source attempt.

The allowance is keyed to the original task/source attempt, not the new release.
A deploy alone cannot renew it. Immutable Git comparisons preserve pending work
and create no snapshot for unrelated changes. The launch, preparation and runner
validate the rejected review, predecessor, exact source/target and original scope.
The revalidation has the existing bounded inspection deadline, no production
execution environment, and no code changes, push, deploy or external effects.

It must report fresh evidence for every original criterion and undergo ordinary
independent review. It does not reset the implementation correction budget, turn
the old failed execution into a success, or accept a result from file changes alone.
Failed inspections and an already used revalidation cannot chain into another one.

A reclaimed inspection launch may already own the latest queued/running attempt
because its initial response was lost. The dispatcher checks the exact stored
attempt-to-launch binding before comparing the predecessor. It reconciles that
launch without creating another attempt or publishing a stale-admission error as
an execution failure. Once an attempt exists, its actual result governs task
outcomes; a stopped launch's old admission diagnostic is not a second outcome.

## Predecessors of delayed automatic retries

Automatic infrastructure, storage and provider recovery requests retain their
source attempt in their deterministic request identity, including legacy saved
requests. Queue admission rejects the request when a newer attempt has replaced
that checkpoint, or the source is no longer a failed/interrupted uncancelled
execution. A lost response with an already-recorded attempt reconciles that exact
receipt instead of treating it as a new retry. User continuation requests retain
their separate approval contract.

The predecessor is checked again atomically when reserving the attempt, under the
same task/attempt locks in PostgreSQL or the memory-store lock. Two supervisors
cannot reserve two replacements for the same predecessor. This does not revive
cancelled tasks, reset any correction budget, replay completed effects, or change
historical execution metadata. Obsolete launch diagnostics stay in history.

API startup preserves a launch bound to an active uncancelled execution; it does
not requeue that admission or say its worker stopped merely because the API
restarted. The exact old restart diagnostic may be cleared for that still-active
receipt. Unrelated diagnostics, terminal tasks and failed executions are not
converted into successful or active work. Ordinary worker reconciliation still
owns process liveness, leases and final outcomes.

## Historical managed footers and publication acceptance

Older managed tasks ended with a system-generated “do not deploy unless
authorized” footer. New tasks already omit it. Execution policy, worker prompts
and reviewer scope now normalize only that exact footer in the generated managed
envelope (including its conversation and authorization fields), when the original
scope already requests deployment/publication to production. Clone/setup work,
local audits, quoted commands and package-only releases do not acquire a deployment
requirement from project defaults. Genuine owner
prohibitions elsewhere remain binding. Stored instructions, project snapshots and
authorization hashes are unchanged; normalization grants no execution permission.

Publication is a separate acceptance fact. For project code results subject to
auto-deploy, an open PR, local commit, passing tests, screenshots or model approval
cannot replace a trusted receipt for the exact attempt/commit, the exact running
project build, or an independently verified snapshot of that running project.
Feature correctness and every original criterion still require their own evidence.
A different project's build or an unrelated production SHA never proves delivery.

A bounded audit revisits only coordinator-accepted historical managed tasks with
the obsolete footer and still-valid original approval. Missing publication returns
that exact accepted snapshot to review with a compare-and-swap check; original
attempts, files, source commit and correction limits are retained. Direct owner
completion, cancellation, newer work, verified publication and unrelated accepted
history are preserved. Re-evaluation gets one durable review identity and bounded
review retry under the new publication policy; it does not silently rerun code or
permit merging an obsolete alternate implementation.

## A successful branch with an unverified published outcome

A historical managed task affected by the corrected publication policy can have
a successful implementation attempt and an open, unpublished branch while the
requested feature already exists in another running revision. Its first bounded
current-outcome inspection may inspect that running revision when the original
scope explicitly required publication, the exact source has a final implementation
report and an unverified-publication audit, and its successful exit and terminal
interval are recorded. Cancellation, missing reports, unrelated failures and
already verified source publication do not qualify.

The source branch commit and successful attempt remain unchanged. The inspection
establishes neither source ancestry nor deployment of that branch. It uses the
same source-attempt/scope request identity, private exact-revision snapshot,
unchanged-source checks, fixed project tests, ten-minute execution bound, empty
environment and publication prohibitions as other current-outcome inspections.
An unsuccessful inspection does not gain another allowance after a new deploy.
Independent review must check all original requirements against fresh evidence;
publication context identifies the actual inspected running commit separately
from the historical unpublished branch. This does not apply to clone/setup work
that never requested live publication.
