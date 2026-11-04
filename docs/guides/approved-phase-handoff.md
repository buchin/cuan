# Approved phase handoff

After coordinator acceptance of a phase, the next phase is selected from the
original accepted Inbox plan. Agent-refined titles and task prose do not select
or authorize another phase. Conversational approvals persisted by Cuan use the
same structured plan envelope as the approval action.

Each child has a deterministic identifier derived from the root task and phase.
Its durable `approved_phase` activity is written by `cuan-plan`, binding the
original owner request, room, project, plan hash, root and parent instructions,
parent successful attempt, child instructions, execution lane and phase number.
Launch authorization rechecks these records after restart. A system handoff
message is a status record, not new user consent.

The parent must be accepted with a successful, uncancelled attempt. A changed
plan, repository binding, owner, project, phase order, instructions or execution
lane cannot reuse the receipt. Lookup/storage errors remain retryable; invalid
authorization is not mislabeled as a temporary storage problem. Existing
queued-launch cancellation checks still apply.

The child inherits the parent's execution lane and publication restrictions.
The original scope is referenced once, without recursively nesting project
snapshots or every preceding child's prompt. Task creation adds its normal
project snapshot; supplied snapshots are not copied into another snapshot.
Plans requiring different execution permissions across phases need an explicit
representation of those permissions; this mechanism cannot grant them by prose.

Inbox reports the actual durable launch outcome instead of claiming automatic
advancement after an ignored error. Request, message, task, receipt and launch
are separate durable steps keyed for replay; they are not one transaction. A
failed step must not authorize execution before its receipt exists. Unknown
legacy approvals are not reconstructed from agent-authored instructions.
Durable launch recovery applies once the launch record exists. Automatic
rescheduling of failures before that record is saved remains separate work;
replaying the handoff uses the same identifiers and can resume those steps.

This is a coordinator/activity-store trust boundary, not cryptographic
isolation against unrestricted host or database access.

Targeted checks:

```sh
go test ./internal/server -run 'TestApprovedPhase|TestApplyInboxReviewEvaluationMultiPhase|TestDetectNextMultiPhase|Test.*Saved.*Launch|Test.*Continuation|Test.*TaskLaunch' -count=1 -timeout 30s
```

`TestApprovedPhasePostgres` additionally requires the existing disposable
`CUAN_LIFECYCLE_SQL_DSN` fixture database named `cuan_s25`. It applies real
migrations including pgcrypto in an isolated schema, validates durable launch
authorization after reconstruction and checks replay without duplicate tasks.
Never run that fixture against production.

### Durable handoff recovery

Before accepting a verified review, the coordinator writes a `phase_handoff_pending`
activity. It binds the saved pass decision and original review identity to the exact
parent attempt, instructions hash, and before/after acceptance evidence digests.
If this first write fails, the original review stays queued with backoff instead of
publishing a final reply. The review may be evaluated again; no work attempt is replayed.

The existing task-update worker reads at most 20 pending intents using a partial
PostgreSQL index. It does not scan accepted tasks or call an AI provider to recover
an already saved decision. Failed recovery is delayed five minutes, for at most 24
hours, and writes a bounded retry record. A finished activity removes the intent
from subsequent pending reads. Restart does not erase either record.

Recovery rechecks the parent attempt, cancellation, scope, evidence and original
approval. If acceptance itself failed, it reuses the saved coordinator decision
through the original review binding and acceptance gates. Changed evidence or
scope stops that intent. Deterministic phase IDs prevent duplicate child tasks.
The handoff is complete only after the durable child launch can be read; launch
recovery then owns execution. A cancelled or stopped launch is never replaced.
The worker does not manually merge, deploy, or change an accepted parent's attempt.

`attemptInboxAutoDelegation` returns persistence errors separately from Inbox prose.
Rejected or changed consent returns `phaseAuthorizationError`. Recovery never parses
reply text to decide whether retry is safe. Storage failure while saving status
cannot falsely claim that a persisted launch did not exist.

This applies to newly recorded decisions; it does not invent handoff approval for
historical tasks. Expired recovery is recorded in the task activity history; a
separate proactive Inbox notice for expiration is not yet implemented.

Verification includes restart after launch persistence failure, acceptance write
failure, retry cooldown, unchanged publication limits, cancellation, scope/evidence
changes, expiry, and real PostgreSQL migrations and launch records.
