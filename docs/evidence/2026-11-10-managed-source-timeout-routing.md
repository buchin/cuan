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
