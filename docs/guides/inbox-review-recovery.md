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
deployments or restarts from creating repeated inspections. Immutable snapshot or
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
