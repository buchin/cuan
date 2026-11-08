# Login task publication reconciliation

## Follow-up: native integration routing

Production release `05ddfda771eccd5679c55e00278947ccdd08873e` passed the
official exact-SHA health check. Task acceptance remains unproven.

A regression fixture reproduced an actual routing defect: with bound independent
source proof, a native failed check on the unchanged running release, unknown
Git ancestry and original write/deploy permission, a PM `revise / implementation /
replan` assessment was converted to `verification`. The classifier previously
recognized implementation only when its revision began with native internal
wording. Native facts now classify the PM continuation and normalize its revision;
reapplying the guard preserves the exact normalized revision. Owner input,
forbidden scope, changed running releases and known containment remain excluded.

Targeted source-review, integration, manager, native-release and execution
inspection tests passed. The source-review fixture first failed with the old
behavior, then passed with the correction. The existing bounded correction-review
queue uses policy v6 to reassess affected saved reviews without rewriting history
or replaying an original worker. Production automatically started request
`a3613650-e53f-4f99-a173-d39dcb934bf7` after release; its result still needs
inspection. No manual task retry or acceptance was used.

Review contract logs now retain sanitized decision enums before guards run.
Unexpected values become `unknown`; private review prose is not logged. This
allows a subsequent routing contradiction to be traced without guessing the raw
PM category from the already-normalized saved assessment.

## Earlier observations

Production code: `c58ee4eeaaf9b8fa26aaf17750d9a0d3f19393c4`, healthy with 37
migrations after the official release command. This task remains incomplete.

Task `msg_174984aa-57f5-4264-a7c6-a63aedea00e1` was blocked after a successful
independent exact-source verifier. The saved assessment requested publication of
an authentication fix. Read-only inspection of retained Git commit
`52d92fccd2019e492d4c314ade211742fffba35b` instead showed only two Feature test
files changed: duplicate `Tests\TestCase` bindings removed, preserving
`RefreshDatabase`. This commit does not itself establish an authentication repair.
The larger recorded branch comparison includes inherited changes.

Source approval without the runner's exact native publication handoff now follows
the existing publication reconciliation/current-outcome inspection route. A valid
native handoff keeps the direct source-review-to-publication route; all native
release authority and acceptance checks remain mandatory. Reviewer instructions
also distinguish causal behavior fixes from incidental setup/tests and inherited
changes, and state that an HTTP 200 login page is not account authentication proof.

Production automatically re-reviewed the saved evidence in request
`7f6b3cf1-4790-4436-afcf-02676b61324d`, changing the remaining work from publication
to verification. The official execution diagnostic now produces a current-outcome
plan for running Dojo revision `0eff9787cbe798248df7c52f4149b997a6016796` with
independent supplemental source evidence. No manual retry or acceptance occurred.

A remaining blocker is concrete: the same bounded current-outcome admission had
already run and failed native Pest checks at the unchanged running revision.
Attempt `att_012a0d13a335c25387b7b211518469ca` retains the failure that
`Tests\TestCase` was bound twice in `DojoMcpServerTest.php`. The existing admission
is stopped, not a live process. Replaying the same observation would not repair
this known source-test defect. The later successful source verifier proves the
retained local test correction, not its integration into the active source.

Next work must reconcile a scoped developer/integration handoff from a typed
verifier's concrete remaining findings, retaining the original source and owner
limits. It must preserve inherited branch work, avoid deploying the whole old
branch merely to satisfy a receipt, distinguish account behavior from project
check health, and keep independent acceptance mandatory. Neither the account's
reported successful login nor local passing tests justify manually marking Done.

Tests passed: supplemental verifier/current-outcome lifecycle including a legacy
source-pass publication assessment, unpublished-result acceptance regressions,
and native release tests. Native release fixture checks verify that a matching
runner handoff remains recognized and its absence is not manufactured by source
approval. No UI changes were made in this follow-up.
