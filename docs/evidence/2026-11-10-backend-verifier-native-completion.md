# Backend verifier dependency task: native completion

Task `msg_d0843350-b9f0-4682-b326-c0adaa2498e8` in room
`753c1a1d-dfaa-4904-a018-03c493e81473` reached native PM acceptance at
2026-11-10T04:20:34.954874+08:00. The official task API reports `done`,
10/10 checklist items complete, and saved assessment request
`9fabfd0c-4b16-4c04-aa5e-6a0cb09e3acc` with `decision=pass`,
`requested_outcome=completed`, `remaining_work=none`. No operator set Done or
filled the checklist to obtain this result.

The accepted source is `b92e18f61bf3945710c25b93aa01cc42283b451f`.
The native release is `8a420703e883af817dc657851cf3ccb6fd53b5f3` with a matching
healthy production receipt. Final independent inspection binds both identities.
The containing release SHA differs from the developer checkpoint; neither is
substituted for the other.

## What actually needed correction

The initial dependency optimization passed its fixtures but still seeded frontend
dependencies whenever an executable `scripts/verify-ui.sh` merely existed in the
source, even with `RequiresVision=false`, no bound visual recipe and backend-only
checks. The old backend fixture did not commit that optional script, so it did
not exercise a Cuan-like repository. An operator counterexample failed on the
then-running source, without executing a script or browser.

Steering arrived near the prior inspector's final report and was not incorporated
there. One factual owner chat message preserved the counterexample in the same
task. PM returned that task to developer execution; no replacement task, scope
expansion or renewed technical approval was required. This is an intervention,
not an untouched autonomous discovery.

The corrected predicate no longer infers dependency need from an unbound script.
RequiresVision, an actually bound visual recipe and explicit frontend checking
scope retain their existing dependency paths. The behavioral fixture now commits
the optional script before snapshot preparation. Exclusive lease ordering,
source integrity and manifest/lock fingerprint checks remain in place.

## Observed native chain

| Stage | Attempt | Observed elapsed time from saved timestamps |
| --- | --- | --- |
| Developer correction | `att_600bce148cb79506dbeed90772896b0e` | 805.731 s |
| Independent source verifier | `att_5f099ca1b8f81ca1a1f09936b82abca4` | 409.262 s |
| Native release | `att_f9afa01786624ecfd8c1213c5c14c9e2` | 470.010 s |
| Independent running-release inspection | `att_dd5aa3d5e11f685334e20f321fd625b4` | 404.537 s |

Source verification reported 5 targeted suites passing in 1.848 seconds. Final
inspection reported 13 cases passing in 3.415 seconds and exact production health.
Those test-body durations exclude compilation, provider work, preparation,
reporting and queue waits. They are not end-to-end speed measurements.

Earlier routing/contract corrections are documented in
[retained-source publication routing](2026-11-10-inspected-publication-routing.md).
Production exercised its one bounded review-format repair: an invalid review was
repaired in 33.6 seconds without re-investigation, retaining classification and
admitting a separate developer. Later source review, native release and final
inspection advanced through Cuan's own coordinator.

## Storage recovery and limits

Final inspection initially remained queued with the official diagnostic
`Server storage is temporarily full`. Maintenance ran successfully, but retained
protected warm/unaccepted snapshot copies and could not restore the 2 GiB floor.
The floor and the 48-hour accepted-source retention deadline were not lowered.
No acceptance metadata, source history or unique task evidence was forged or
removed to reclaim bytes.

Under the standing server cleanup authorization, the native release pruner removed
three obsolete payloads after a dry run, preserving active/previous releases.
A closed PostgreSQL incident archive was then preserved off-host in the operator's
private workspace. Its 525,110,933 bytes and SHA256
`dfca3f242f4704a8f3f2e7d6eae3d1d5c20f26aed5a5b425f499cafee7861e98`
matched before removing the VPS copy; a private receipt records both locations.
Available disk then measured 2,274,938,880 bytes. The same queued launch resumed
automatically, without an operator retry or status update.

This recovery was manual OS administration. It does not establish that Cuan can
autonomously resolve every storage-pressure condition. The completed task had 14
attempts, earlier operator fixes/steering and historical prohibited screenshots
(documented separately), so it is not a fresh autonomous speed or scope-compliance
benchmark. The final accepted changes and later attempts stayed backend-only.

## Remaining work toward the broader goal

- A fresh useful task still needs an untouched end-to-end observation.
- Storage maintenance needs to avoid indefinite accumulation of redundant
  materialized copies while retaining Git objects, receipts, unique files and
  reliable restoration. Existing failed history must remain failed.
- Build/provider/preparation time still dominates the reported test-body time.
  There is no defensible overall speedup percentage from this run.

The broader PM autonomy, lean execution, UX and modularity objective remains open.
