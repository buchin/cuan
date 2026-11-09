# Native receipt failure preserves source verification

Task msg_d0843350-b9f0-4682-b326-c0adaa2498e8 retained source commit
c84c84b82c6d86b758204d050504e5edfe65ed5a after its developer exited 0 and native
project checks passed (71.585 seconds). The 23m45.617s attempt remained failed
at the deployment receipt gate. Its old claim had prohibited deployment before
the scope parser corrections. PM assessment 6193a631-a74d-4113-a65d-285a42f82c43
correctly requested verification, but admission rejected the saved checkpoint.

Existing publication-reconciliation admission already requires exact native
source identity, runner-authored passing checks, a final report, unchanged owner
scope and a saved PM verification decision. It recognized one native missing
publication error but not the equivalent receipt-required completion error from
internal/runner/service.go. Both exact native diagnostics now share recognition;
the receipt-required message is a task constant used by the runner. Unknown
failures, nonzero exits, cancellation, missing checks/report/source and altered
scope remain ineligible. No failure, checklist or status is rewritten.

The existing complete task admission matrix now runs against both native error
messages; the receipt-required valid case failed before the fix and passes after.
A server integration fixture verifies exact-source isolated verification admission
and preserved failure history for the second error. Existing verification launch
and runner publication boundary groups pass. The server compiler initially ran
out of memory; GOGC=20 GOMAXPROCS=2 and -p 1 completed compilation and targeted
checks (server checks 0.400s). Production admission still needs observation.

The earlier complete original-scope parser correction cee65517e6db7beb978769c254549a9e2b4e0c31
released 16:15:28Z–16:18:33Z (184.37s), exact healthy SHA, 38 migrations and exit 0.
Official execution readback changed deny_deploy to false without rewriting the
original task. The already-running worker survived both supervisor deployments.
Root also sent one bounded steering message during prolonged developer exploration;
this run is not an untouched autonomous workflow proof.

## Rollout and remaining saved-decision admission

Release 55d2886ab3a7d5f10c42f32b683ecea0cf305012 completed
16:28:59Z–16:32:08Z (189.33s), exit 0, exact healthy SHA and unchanged source.
The native diagnostic now recognizes retained source eligibility, but no source
verifier was admitted. Latest persisted PM verification decisions omitted the
optional manager action/reason fields. Generic implementation continuation
requires those fields and therefore also stranded this read-only evidence run.

The existing manager-handoff recovery now recognizes a saved verification
assessment for a natively tested publication-only failure before the generic
implementation-continuation gate. Its original owner authorization, current
review snapshot, ownership-policy input, native source/check/report evidence,
independent-review requirement and unique exact-source launch still apply.
wait, needs_owner, needs_input and cancellation remain excluded. No ordinary
implementation or publication is admitted through this path.

The actual missing-optional-fields fixture reproduced the stranded decision.
Targeted integration tests now cover successful recovery plus wait, owner input,
missing input and cancellation, with source failure history preserved. Existing
manager-recovery and unpublished-source launch contracts pass (1.462s after
compilation). This second admission correction still needs production observation.
