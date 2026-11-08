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

That PM request subsequently completed with a saved `revise / implementation /
replan` assessment and automatically launched developer attempt
`att_d26ef563e5cb20efeb37d9b0ff156f53` (attempt 12). The official execution
diagnostic confirms a started ordinary execution with original read/write scope:
`read_only_repository=false`, `deny_push=false`, `deny_deploy=false`. The new
developer has begun inspecting the retained repository. This proves production
handoff recovery, not completed implementation, publication or final acceptance.

Developer attempt 12 subsequently failed the native publication gate despite
passing registered source checks. Its branch push returned HTTP 409 with an
unconfirmed-push error. The Git adapter discarded subprocess output, conflating
divergence, repository policy, access rejection and network failures. The original
specific cause cannot be recovered from that discarded output; no remote push was
replayed for diagnosis. The adapter now classifies those failures from captured
output into fixed safe messages without exposing raw remote output or secrets.
Unknown/interrupted outcomes still require remote-state reconciliation. Targeted
Git tests cover the subprocess path, classification, redaction and retained native
source push behavior. PM automatically started review request
`08683cc4-b989-4a7e-a72a-ad3681b3650c`; acceptance remains pending.

That review admitted current-outcome attempt
`att_4f6c8d0985cac4a92d192ab17d85f688` on the unchanged running target. It
failed the same native project checks. Planning previously checked only the
selected source attempt's inspection identity, so a new developer attempt could
renew an otherwise unchanged failing inspection. Planning now checks historical
native current-outcome failures across source attempts, bound to the same task,
owner, room, original request, scope hash, repository and target release. A native
runner check receipt is required. The diagnostic is
`running_release_native_checks_failed`; changed targets retain existing recovery.
This blocks redundant observation, not acceptance checks or the original task.
Targeted live lifecycle, source handoff, unpublished outcome, preparation,
infrastructure and evidence-revalidation tests passed, including foreign and
changed-scope negatives. This does not fix the underlying project test failure.

The stopped latest source-verifier launch
`launch_66fdb68984ebb6ccb3f71f853a9214b87d899fb1a9e85fa4d5393c69480563ee`
had no execution attempt: admission lost a race with live attempt 13 and recorded
the exact active-attempt rejection as permanent failure. A temporary reservation
collision now keeps the launch queued without changing the active task. Legacy
unstarted launches with that exact rejection can use the existing once-only
pre-reservation recovery; no own attempt, newer launch, active execution, changed
scope, cancellation or expiry can bypass its existing store guards. Tests cover
the competing completed inspection, one-time recovery and queue response state.
This recovers admission only and does not convert any failed attempt to success.

Production release `6148fa486265732f6733324570e6c43c2c7744c7` passed exact-SHA
health verification. The existing source-verifier launch recovered automatically
as attempt 14 (`att_66fdb68984ebb6ccb3f71f853a9214b8`), retaining read-only,
no-push and no-deploy scope. Its live output then showed a missing Vite runtime,
a failed frontend build and creation of a replacement build manifest. Such a
manifest cannot prove real application behavior. The task remains unaccepted;
any resulting report needs inspection against actual runtime evidence.

Worker guidance also contained unconditional deployment-completion wording for
all auto-deploy sessions, conflicting with the independent read-only stage.
Guidance now separates developer, verifier, coordinator publication and final
acceptance responsibilities; native release gates remain unchanged. It explicitly
forbids manufacturing build/runtime inputs to satisfy acceptance and requires
reporting missing preparation rather than changing a read-only snapshot. This
prompt clarification does not itself enforce filesystem isolation or prove that
the active verifier's generated runtime inputs were genuine. Existing scoped
report, bound-context and standalone-artifact guidance tests passed.

PM delegation guidance now explicitly selects the supported execution category
from the requested outcome. An individual login complaint without an established
source defect starts as bounded operational diagnosis with source/test/deploy
preservation, retaining the account outcome and customer reply requirements.
A discovered source defect requires a concrete scoped handoff under original
authorization, not silent expansion of that operational session. Existing running
task permissions are not reclassified retroactively. This is guidance for future
delegations, not proof that this historical task is complete or operationally
recovered. Protocol and scope-classification tests passed. Two provider-backed
delegation fixtures exited without a final response; a baseline Go overlay using
the unmodified coordinator file reproduced the same managed-project failure.
Those end-to-end checks remain failing and are not reported as passing validation.

Follow-up isolated the fixture failure: its shell substitution took everything
after `Target message ID:`, including later prompt sections containing `/`, and
passed that text as a sed replacement. A diagnostic overlay captured
`sed: unmatched '/'` before the fixture emitted any provider response. The fixture
now extracts only the UUID on the target line and fails explicitly if it is
absent. Both managed-project and clear-instruction delegation tests now pass.
This changes the test harness, not production provider execution.

Developer attempt 15 reported reconciliation of the divergent remote task branch
and successful push of merge commit `553cd2c`; this does not prove deployment or
the original account outcome. The native workflow automatically scheduled the
next verifier. The task remains incomplete until actual independent outcome and
applicable publication evidence are accepted.

New login-support tasks explicitly saved in the operational lane now use the
existing independent account-outcome pipeline, rather than merely relying on a
PM prompt to avoid source verification. Authentication plus account scope is
required; explicit source/schema work remains outside this classification.
Historical code-lane login tasks retain their native source/publication gates
and permissions. The common account-outcome predicate is used consistently by
scope limits, acceptance review, publication review and saved-evidence recovery.
No duplicate verification service was added. Targeted tests demonstrate that an
operational login worker cannot be accepted without independent observation and
that observation retains native read-only/no-replay boundaries. Task-classification
tests reject retroactive restriction/exemption of existing code work and explicit
controller repair. These tests do not establish production completion of the
historical login task or resolve its generated-manifest evidence concern.

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
