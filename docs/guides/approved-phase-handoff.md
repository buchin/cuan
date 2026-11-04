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

### Recovery boundary

`attemptInboxAutoDelegation` returns persistence errors separately from the Inbox explanation. Rejected or changed consent returns `phaseAuthorizationError`; callers must not infer retry eligibility from reply text. Every replay revalidates the original approval and deterministic phase identity. This error boundary alone does not schedule recovery: failures before the durable launch exists still need a persisted handoff job. The compatibility wrapper currently preserves the existing synchronous behavior.
