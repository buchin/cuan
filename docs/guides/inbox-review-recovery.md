# Recovering unfinished task reviews

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
