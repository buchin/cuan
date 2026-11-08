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

Legacy support tasks can now use the same bounded saved-evidence observation
after a native publication-only failure with passing source checks and a complete
worker report. Scope eligibility is separate from execution permission: existing
code workers keep their original lane and permissions. The recovery itself is
read-only and cannot publish or repeat account effects. Native dedicated account
observation, unchanged repository proof and original authorization are required
before review treats the account outcome separately from retained source. A generic
source verifier or manager label cannot substitute for that dedicated legacy
observation. Original account criteria and explicit requested-outcome review remain
mandatory; source-specific requests are excluded. Failed history and retained
commits are preserved, not accepted as deployed implementation. This recovery is
prioritized ahead of another source/live cycle for affected terminal workers.

The extended native recovery test exercises legacy login support through actual
claim/completion, confirms worker claims do not bypass independent evidence,
checks original code permissions remain unchanged while the observer is restricted,
and verifies observation deduplication. Native source-review and original account
operation tests remain passing. Production completion remains unproven.

Attempt 17 exposed a source-information gap: its report named checkpoint
`571b29c`, but native project checks failed before the runner observed the local
checkpoint, leaving task-level source identity at previous `553cd2c`. The next
PM assessment compared that stale source with the running release. Runner failure
completion now observes an existing clean local code checkpoint after failed
native checks, using the same read-only source observation helper as local
handoffs. Failure status and failed check output remain unchanged; dirty source
is not attributed to HEAD. No commit, push, release preparation or publication
authority is created. An end-to-end runner fixture verifies actual completion
contains the new checkpoint and failing check output together. Local-source
dirty/unknown-baseline tests and read-only runner completion tests pass. This does
not retroactively replace attempt 17's recorded evidence or establish delivery.

Read-only diagnostic overlays against the saved production scope confirm login
support eligibility is true and the terminal attempt-18 snapshot is a valid
legacy account-publication candidate (complete report, passing native checks,
bound current source record). No corresponding account-observation launch was
recorded; source verifier 19 ran instead. Recovery now logs its exact gate when
the existing dispatcher diagnostic context is active: candidate, origin binding,
review authority, saved authorization, source intent, saved launch read, existing
launch or admission. Only task/attempt identity and gate codes are logged, never
scope prose, account data, credentials or model output. This is diagnostic work;
the production admission blocker is not yet established. Existing recovery
lifecycle tests still pass.

The deployed gate trace identifies current source verifier 19 as ineligible at
`candidate`. A successful source verifier was not a saved-account recovery
candidate, and the generic source-intent guard rejected it. A native handoff now
resolves only its validated exact developer source and requires that source's
publication-only failure, passing native checks, original support scope and
complete report. Original owner/room/request, cancellation and unchanged-source
proof remain mandatory. This uses the same task/scope-keyed account-observation
launch, with no renewed allowance or implementation replay. Source proof alone
is still not independent evidence of the account outcome or final acceptance.

The lifecycle fixture now performs real native source claim/completion followed
by separate bounded account observation. It verifies the source verifier is not
accepted as account proof, observation deduplicates, native read-only boundaries
remain, and changed scope/source, cancellation, active verification and missing
receipts reject the handoff. Account recovery, manager grant, source-review and
delegation protocol targeted tests passed. Production completion is still pending.

## Production account outcome acceptance

Account observer `att_76f9c8d4689ec7187e31284d4c4b6c95` completed successfully
without source publication. Review `a7f988a1-417c-474f-a468-3cb94b9473dc`
saved `pass / completed / none`; the coordinator marked the original task Done
at `2026-11-09T01:03:19.154276+08:00`. No maintainer retry, manual acceptance or
Dojo deployment was used. This proves native task acceptance and bounded outcome
handoff, not general release readiness or an acceptable twenty-session cost.

The report distinguishes retained test repairs from the original login incident.
Its WIB labels disagree with its UTC/+08 timestamps, and public login screenshots
do not demonstrate an authenticated account session. Historical account success
claims must not be conflated with a new login performed during this audit. The
original visitor timestamp was independently read through the official Cuan CLI.

The accepted PM review already contained a customer draft, but delivery invoked
another model turn. The first delivery failed; its bounded retry eventually
persisted a separate internal `reply_draft` in the owner's conversational style.
The CLI room projection confirms it is internal and usable; no message was sent
to the customer. Delivery now reuses a valid draft from the same accepted review
before provider setup/inference, retaining authorization, draft validation,
prepared-text persistence, user review and existing fallback generation. A native
Inbox lifecycle test with no provider profile confirms completion, exact saved
text, no provider stage, no extra task attempts, and foreign-owner rejection.
Ordinary customer draft delivery and managed delegation tests also pass.

Release `3f8851ebe108d8ef1e3244c879801e455b8f0ca0` passed exact-SHA production
health verification. A fresh read-only production benchmark was initiated in room
`e3e42d8b-6486-41d7-ab1e-308d84d8c1a3`, request
`c906633b-ef85-4d39-8a94-fe273a0e9660`. The natural account-diagnosis request
forbids data/access changes, email/OTP, resets, source/tests and deployment, asks
for independently inspectable facts and a customer reply, and does not prescribe
an internal execution category. The initial CLI send observation timed out;
reading the same room confirmed one running request, so it was not resent.
The benchmark completed at `2026-11-09T01:19:05.348498+08:00`, approximately
80.2 seconds after acceptance. The room has zero tasks and zero execution
attempts. Its trace contains response formatting but no fresh investigation
tool calls: the reply reused accepted historical findings. A separate internal
customer draft is present. This proves reuse without redundant execution, not
fresh account inspection or a new independently verified workflow. The response's
claim that the account is currently normal is stronger than this trace alone
establishes; a public login page returning HTTP 200 is not account login proof.

A follow-up requests a genuinely new read-only, downloadable account report,
with observation time and timezone, concrete sources, and explicit uncertainty
where authentication history is unavailable. It forbids account/data/source/test/
deployment changes and customer contact. Request
`6a97bee3-8b7b-4e12-850c-1a92bffc9c0a` was accepted at
`2026-11-09T01:23:21.630987+08:00`; an official room read confirmed it running
after the CLI observation timed out. It was not resent or manually retried.
It delegated one saved `readonly` task,
`msg_18c7b5ba-ad2f-4544-9159-67c70f37050c`, to attempt
`att_c9fae09071763859a9040c340685f57e`. That attempt timed out after five
minutes at `2026-11-09T01:29:34.086337+08:00`, without a final report; the task
was blocked on observation. This fresh benchmark therefore failed. Saved output
shows reading account data and attempting to interpret access logs, but does not
establish a completed fresh report or authenticated login proof.

The official execution diagnostic exposed another concrete gap: despite the
saved `readonly` category, repository read-only and deny-push limits were false
(deny-deploy was true). A local change binds the category to all three limits,
independent of prompt phrasing. Targeted task policy tests pass. A server
delegation test now refuses acceptance for the legacy `local_terminal` path,
which supplies no native unchanged-repository receipt; that path still needs
reconciliation before this change is released. No acceptance gate was waived,
and the local policy change is not yet deployed.

The local-terminal gap is now repaired locally: native observation runs before
launch and after process exit, persists an attempt/runner-bound unchanged receipt,
and refuses completion on mutation or unavailable evidence. Non-Git project
directories use a bounded filesystem fingerprint without following symlinks;
agents still place report artifacts outside the inspected project. Native runner
baseline/final checks share this observer. Targeted tests pass for unchanged
delegation, managed projects, mutation rejection with file preservation, review
recovery, account outcome recovery, directory mutation/symlink/cancellation, and
existing runner repository checks.

The production task automatically launched attempt two,
`att_3fbe697499e4ca201555072cd22388bb`, without manual retry. It also timed out
at `2026-11-09T01:35:59.008898+08:00`, leaving no final report. Neither the
scope-policy repair nor the local-terminal tests establish a solution to this
separate repeated account-investigation timeout.

Release `da18631fc9b8fac786659ff2e495ee1e246dcf53` passed production health
with matching exact SHA and 37 migrations. A third automatically scheduled
attempt, `att_60669a05d609705aa3898851fd46695c`, started before that rollout.
It saved a downloadable report and final review report, but failed native
verification because registered PHPUnit target
`packages/laravel-mcp-bridge/tests` was missing. Source regression tests were
unrelated to the approved read-only account report. The original failure remains
historical; recomputed execution diagnostics after the policy change are not a
receipt of the policy enforced by the old process when it started.

The third report was downloaded through the official CLI and inspected. It
names actual account fields and observation times with UTC/WIB/UTC+8 conversions;
the supplied session activity epoch matches its UTC conversion. This checks
report content, not independent confirmation of its database observations.
The report contains private account/session details and should not be pasted
into customer chat. No manual retry or acceptance was performed.

A separate local recovery-context fix preserves up to four bounded historical
progress reports from actual older attempts of the same task, with explicit
attempt provenance and a warning that they are neither fresh proof nor new
authority. Previously, a silent retry hid earlier progress from the recovery
summary. Tests retain current-attempt ordering/bounds, reject foreign tasks and
unknown attempts, and confirm earlier checkpoints survive a silent retry.
This context change is not yet deployed.

The fresh task also exposed a lane omission: saved `readonly` account diagnosis
did not enter the existing account-outcome observer route, unlike `ops` account
diagnosis. The local fix includes both explicitly non-source lanes while
preserving historical code-task authority. A native-bound PHP prerequisite
failure on a completed read-only account report can now admit the same bounded
independent observation; it does not waive the failure, reset attempt history,
accept worker claims or repair unrelated source. The existing lifecycle contract
now exercises this exact lane, source-change/assertion/cancel/incomplete rejection,
deduplication, scoped observer claim/completion and native independent proof.
Targeted server/task recovery, operational and classification tests pass.

Release `edc1fda359d4adbe1a90320945e0c0f0de724b8c` deployed both lane recovery
and historical checkpoints; official health verification matched that exact SHA
with 37 migrations. The first post-release task read still showed the third
historical failed attempt and no admitted observer. Deployment success therefore
does not yet prove production recovery or acceptance of this benchmark.

Production dispatcher logs subsequently show `gate=launch_admission` for this
task: eligibility and owner checks pass, but no observer launch is persisted.
A read-only Go diagnostic against the official task snapshot reports 3,656
checkpoint runes before adding observer instructions. The untyped launch only
accepts 6,000 feedback runes. Recovery now keeps reports/criteria in bound task
context and instructs the observer to read `task get` before inspecting retained
artifacts, rather than duplicating those records in a feedback-sized revision.
A long-report lifecycle regression fails against the previous recovery code and
passes with the change, retaining independent proof, no-write scope and dedupe.
Launch persistence failures now log fixed safe codes and rune counts/limit,
without reporting revision prose or credentials. Targeted lifecycle/checkpoint
tests pass. Production acceptance is still unproven.

Release `a95ec5ecefe22ce975b3e6347cb213adfbd480e4` passed exact-SHA health.
The dispatcher automatically admitted observer attempt four,
`att_3b69bc88b664f2b96c5eca671f6ff823`, with a 2,503-rune revision and all
three read-only/no-push/no-deploy limits. It timed out at
`2026-11-09T02:03:29.745887+08:00`, without a final report.

Its output exposes a concrete access-preparation failure: the Dojo wrapper
fails, the agent searches for Deployer, then tries project environment tools.
The native path proves why saved credentials were unavailable: runner merges
the owner-saved project environment, but `SpawnPTY` sanitizes every `DOJO_*`,
password/key variable back out. The saved project guidance directs workers to
`.env`/`ops/bin/dojo-artisan`, whose wrapper additionally requires local Deployer.
No project credentials need to be copied into repo files or borrowed from host.

The local fix gives PTY configuration a separate explicitly delegated project
environment map; runner passes only the claim's saved project environment and
local terminal passes the registered project values. Ordinary custom/ambient
environment still uses the original sensitive-key filter. A real subprocess
test confirms delegated project credentials arrive, untrusted custom credentials
and ambient host/other-owner secrets do not, and ordinary sessions get none.
Targeted terminal, runner repository/read-only and server account/delegation/
mutation-rejection tests pass. This fix is not yet deployed; observer four's
historical failure and single-observer recovery bound remain unchanged.

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
