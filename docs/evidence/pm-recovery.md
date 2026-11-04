# Cuan project-management recovery — active investigation

Goal: resolve why authorized Inbox work remains unfinished, let Cuan process the
existing work sequentially, verify actual results and report to its originating
Inbox. The goal is active; an isolated successful task does not prove completion.
Canceled work remains stopped; accepted work is preserved. No duplicate tasks or
administrative success flips may substitute for deliverables.

## Initial evidence

Production baseline6103455. Official CLI inventory:162 Cuan project tasks,
100done; remaining records include canceled runs, stale review, blocked/failed
execution and one saved backlog/todo each. Private snapshots live under
`/home/dev/reports/cuan-pm-recovery/`; task snapshots contain private environment
instructions and must not be published wholesale.

Recent examples include audit/deployment, clickable cards and task-button tasks
with reported results but blocked state. Their review requests recorded a
capacity_wait event and then completed with a work rejection. One native final
report intentionally closed its PTY with exit-1 while its attempt was recorded
succeeded. Old evaluation treated any nonzero exit as failure. Older infrastructure
failures and cancellation also consumed raw process-count retry limits.

## First root-cause repairs

- Review-capacity exhaustion retains the same queued review request and existing
  backoff rather than applying a substitute rejection or changing task state.
- Plain Markdown reviews are applied through the coordinator. A content prefix
  does not prove the decision has already been persisted; fallback decisions are
  explicitly tracked to prevent double application.
- Nonzero shutdown is reviewable only when attempt status is succeeded and a
  matching latest final agent_report for that same attempt requests review/done.
  Failed/canceled runs, older reports, explicit blocked reports and missing DoD
  evidence remain unaccepted. Real process exit codes are preserved.
- Business correction limits use the existing classified attempt count rather
  than raw process ordinal. A recorded success spends the business budget even
  if its intentional shutdown code resembles an infrastructure signal.
- Accepted done work is not automatically reviewed/rejected again. Late rejection
  leaves it accepted. Failed acceptance writes never announce successful closure.

Regression tests in `internal/server/pm_review_recovery_test.go` exercise actual
review execution through a fixture provider, capacity preservation/cancellation,
Markdown decision persistence, matching-report provenance, unchanged DoD guards,
retry accounting and final-state preservation. Existing Scrum/review/capacity
regressions pass. This is fixture evidence; production recovery remains required.

## Approved avatar follow-up

User separately approved square avatars to distinguish identity from round task
progress. Shared avatar styling and Inbox/account surfaces now use square identity
shapes; existing36px triggers/40px account menu avatars are preserved. Pie SVGs and
status dots remain circular. Real390px touch and1440px desktop screenshots in
light/dark were rendered and inspected under the private visual evidence directory.

## Remaining verification

Deploy and inspect production; authorize a scoped resume through the official
Cuan chat interface on the same task, then observe worker execution, evidence,
coordinator acceptance and same-Inbox report. Audit the rest of the authorized
backlog and resolve its actual prerequisites. Do not mark the overall goal done
merely because this first repair or a pilot succeeds.

## Customer-room task attachment repair

Production98c54f2 deployed healthy, with server/runner on the same release,
PostgreSQL accepting connections and7.4GB free disk. While verifying, the owner
reported a customer Inbox showing one working task but no attachment. Official
CLI confirmed a running task and an internal agent reply referencing that task.
The owner Inbox gated attachments on agent-room type, while orphan detection
already treated the linked reply as attached. This hid the card entirely.

Internal agent replies now render the same task attachment in the owner Inbox.
The visitor channel and its internal-message filtering are unchanged. Public
customer messages do not become eligible for this internal task attachment.
Real390px touch/1440px light/dark browser regression checks verify exactly one
attachment beneath the internal delegation, Detail preserving the Inbox route,
and no Terminal for a completed task without a live session. Screenshots were
rendered and directly inspected. Reproduce with
`node docs/evidence/customer-task-card-check.cjs` after the frontend build.

## First live resume: retained branch, missing worktree

The original audit resume request `aac9bded-33d0-4788-8b99-05e6d5a785c5`
completed its coordinator phase and attempted to launch the same task, without a
new task. Execution did not start: the original Git worktree directory is absent,
but its branch and Git worktree registration remain. No verified retirement
receipt exists. The application correctly refuses to pretend the missing worktree
was deliberately retired; uncommitted files cannot be assumed recovered.

The launch reached stopped while the board remained queued. `FinishLaunch` now
atomically marks a still-queued task blocked when its latest launch stops and it
has no queued/running attempt. PostgreSQL uses the task advisory lock shared with
native attempt reservation/cancellation. Older launch outcomes, backoff, active
attempts, canceled/done work and stale lease tokens cannot overwrite current
state. Memory and isolated PostgreSQL exercise the same seven scenarios.
Targeted server queue recovery and Inbox delivery tests also pass.

This repair does not recover lost files. Next: provide a recovery path preserving
retained branch/index evidence and clearly separating verified reconstruction
from missing uncommitted work; then resume the same audit. General sequential
backlog recovery is still unproven and remains required.

## Evidence-preserving workspace reconstruction

An authorized task continuation with concrete revision and a recorded previous
attempt can now reconstruct a missing isolated worktree from its retained
committed branch. Ordinary task viewing still fails closed when no reconstruction
record exists. Invalid retirement receipts are not bypassed. Reconstruction uses
a separate stable branch/directory; the original missing path, branch, registered
Git metadata and index are untouched. Available staged changes are exported as a
private binary patch and original index before execution. Private evidence and
encrypted identity receipt are written exclusively and synced to disk. Failures
leave evidence in place instead of overwriting or creating another replacement.

Task guidance explicitly says unstaged/untracked files are not proven recovered,
previous checkmarks are historical, staged changes need inspection and every
original acceptance criterion needs fresh evidence. Resume validates repository
identity and recovery branch; a second external deletion is not silently reset.

Project tests verify untouched original branch/index, staged patch preservation,
no invented untracked recovery, restart preserving new pending files and refusal
to rebuild another lost copy. Server integration exercises approved delegation,
process loss, missing worktree, same-task reconstruction, actual fixture execution,
review evaluation, original scope and exactly one same-Inbox outcome. Existing
pending-work recovery and review/capacity regressions pass. Fixture reports use
review before coordinator acceptance; accepted done work must not be re-reviewed.
Production audit recovery remains to be demonstrated after deployment.

## Historical persona routing held the next instruction

After reconstruction deployment c261607, the same-room queue exposed a legacy
agent_sm request repeatedly waiting for capacity ahead of the owner's new
agent_cuan continuation. A current owned account reports available model capacity;
legacy persona configuration must not pin historical Inbox work to its old
connection when the editable public assistant exists.

Runtime agent resolution for known sm/dev/support/cuan aliases now prefers the
current public assistant in both Inbox coordination and task execution, while
preserving saved request/task identity. Custom agents and fixture identities are
not remapped, and a disabled public assistant is not bypassed through a legacy
persona. Regression tests cover existing conflicting legacy records, editable
public name/connection, custom identities and disabled-state preservation; facade,
failover and missing-worktree flow regressions pass. The pending production
requests remain the same: no resend/replacement is required after deployment.

## Live Heru task: terminal answer without a persisted final report

Owner reported working state after the terminal printed an answer in room
8c899836-c96c-4f28-a747-2bab7c14f5bf. Official task/log inspection confirms attempt
att_bfe5a865bd07f002bbc070367b2455d2 remains running with renewable runner lease,
but task summary/verification and matching final agent_report are absent. The
latest terminal output ends with a user-facing answer and interactive prompt.
Application logs also show task-agent access returning401 after deployment.

Reporting capabilities were stored only in server memory. A surviving detached
runner retained its original token across server restart, while the server lost
its grant. Runner read-only report recovery could read already-saved reports but
could not authorize the agent's missing final write. Polling renewed the execution
lease while waiting for a valid report; printed prose alone is not acceptance.

Newly issued grants now persist as private encrypted attempt-bound records under
the server's stable Cuan configuration home. Records bind the token digest,
task/attempt/agent, expiry and artifact root; writes are synced before issuance.
Restart reload validates the record and existing active-attempt checks. Arbitrary
same-length tokens, copied ciphertext under another digest and cross-task access
remain rejected. Cleanup revokes disk and memory. Tests simulate memory loss,
actually submit a final report, verify persistence and exercise denial/revocation.
Already-issued pre-repair tokens have no durable record and cannot be invented;
that live session requires a scoped reporting recovery retaining its files.

## Dispatch and explicit continuation

History-result inspection no longer precedes queued conversation dispatch. It
runs as a single bounded background scan; accepted done attempts are skipped.
Only actual pending/recent recovery rooms are discovered from durable requests,
and idle/backoff rooms do not reserve provider slots while reading history.
Memory and isolated PostgreSQL discovery tests cover idle-room exclusion and
recent-failure recovery. A blocked history query integration test proves a new
conversation completes without waiting for that query. An older broad SQL
fixture additionally failed in its subprocess sandbox setup; this is recorded
separately from the passing new isolated discovery contract.

A fresh accepted owner continuation can exceed the automatic revision allowance
without changing original scope/authorization. Automated system reviews and
replayed old approval remain bounded. Regression exhausts three business failures,
then verifies that the freshly authenticated continuation starts the same task
while its original authorization ID remains unchanged.

## Mobile customer header clipping

The owner's screenshot showed contact name clipped above the fixed-height chat
header. Customer email's44px copy target was nested in a wrapping text row; name,
email and customer label exceeded the shared header height. The mobile identity
now has two text lines (name/email), copy is a separate header action, and the
redundant Customer label is omitted. Desktop retains inline name/email. Copy
feedback uses the same-size icon, so no width/height jump occurs. Real390px touch
and1440px light/dark browser fixtures check name/email bounds, long labels,44px
copy target, actual clipboard value, stable header height and task-card actions.
Screenshots were rendered and directly inspected before deployment.

## Approved quiet internal-visibility indicator

User approved replacing repeated internal banners with a small eye-off marker.
The reusable `InternalVisibility` component now appears on internal bubbles,
owner-only customer-room task cards and private attachment previews (including the
opened file dialog). Public message/file surfaces are unmarked; draft attachments
follow the composer mode. Amber rings/banners on sent bubbles were removed.
Message/backend filtering remains unchanged. Real390px touch and1440px desktop
light/dark fixtures verify public/internal distinctions,44px targets, click and
keyboard explanation, viewport bounds, Escape/focus return and native-dialog
portal layering. Screenshots were directly inspected. Build and targeted header
regressions pass; visitor filtering remains covered by backend tests.

Live Heru recovery now has a new final report with summary/verification saved on
attempt att_660e570b4664acc669aa3056ef791936. The attempt exits0 but is rejected by
auto-deploy compliance because earlier code changes were not proven deployed.
This is a separate scope/compliance issue, not continued loss of report access.
Do not claim the original installation/live criteria fulfilled without evidence.

## Attempt-bound revision publication limits

Live Heru attempt3 saved a final report but was rejected for missing deployment,
despite the explicitly authorized reporting-only continuation prohibiting push
and deploy. Runtime and server compliance checks parsed only original task
instructions; the narrower revision lived in the durable launch and was ignored.

Publication limits now combine the original task with the revision for that exact
attempt via task_launches.attempt_id. A newer launch cannot change an older run's
restrictions. Claim payloads carry limits; runner prompts, automatic commit/push,
server push authorization, completion compliance and final-report auto-apply all
honor them. Restrictions can only narrow defaults, never authorize publication.
Missing live-install acceptance evidence remains a separate DoD gap.

Memory and dedicated PostgreSQL contracts verify exact-attempt binding and
unchanged original restrictions despite a newer launch. Runner integration
verifies an auto-deploy-enabled project emits a no-push/no-deploy claim and accepts
execution completion without demanding prohibited deployment. A real local Git
fixture verifies the automatic publication path preserves pending files and HEAD.
Targeted server reporting/push/reconstruction regressions pass. Production proof
on a new continuation remains required after deployment.

## Compact empty composer

The owner reported a three-line customer placeholder making the empty mobile
composer tall. Public/internal customer placeholders are now short localized
strings (Write a reply / Internal message; Tulis balasan / Pesan internal).
Empty autosizing uses the one-line minimum rather than placeholder scrollHeight.
Typed content still grows to the existing140px cap and clearing returns to one
line. Real390px/1440px light/dark browser checks verify both modes, bounded growth,
shrink after clear and the existing internal-visibility/header/task-card flows.
Screenshots were rendered and inspected before deployment.

## Autonomy constraint clarified by the owner

The owner explicitly clarified that the outcome is a capable Cuan, not a backlog
completed by the external development session. Manual status changes, individual
resume prompts, external deliverable implementation and repeated owner-like
instructions must not substitute for Cuan's own durable orchestration. Earlier
manually initiated recovery pilots are diagnostic evidence only; they do not
prove autonomous project management.

Continue through product-level fixes: durable recovery of already-authorized
intent, same-task execution, evidence-based acceptance, bounded correction,
capacity/dependency recovery and automatic advancement to the next authorized
item. Direct intervention is reserved for necessary diagnosis/system repair or
an actual external prerequisite. Keep canceled work stopped and distinguish an
unresolvable prerequisite from an agent-fixable failure without repeatedly asking
the user to operate tasks.

Current acceptance concern confirmed through GitHub: Heru task's referenced PR9
remains OPEN, mergeCommit/mergedAt are null. Its report's HTTP200/live-readiness
wording alone does not prove that task commit f628a6d is deployed. Correct the
systemic evidence gate and autonomous correction path; do not manually rewrite
that task into apparent success or use a fresh resume prompt as the solution.

## Automatic recovery of stopped pre-launch workspace checks

The coordinator now revisits a latest stopped launch with the exact known
missing-workspace error, a concrete saved revision and an unexpired approval.
It validates persisted owner messages, cancellation and unchanged scope before
atomically requeuing that same intent. A durable recovery receipt limits this to
one check; reserved attempts, newer launches, active execution, canceled work and
accepted completion cannot be replayed. Migration33 stores receipts with launch
cascade deletion. No production task status or owner prompt is injected manually.

The new server flow fixture exercises a real Git worktree loss, a stopped saved
revision with no reserved attempt, ordinary Inbox delivery recovery and launch
dispatch, actual fixture execution/reporting, and coordinator acceptance in the
same conversation. It verifies one task, two total attempts, one original side
effect and one verified final outcome; staged evidence is preserved and missing
untracked work is disclosed. Targeted authorization tests reject missing approval,
system messages, foreign owners, future messages and canceled requests. Memory
and isolated PostgreSQL contracts cover stale observations, changed scope, newer
intent, expiration and consumed receipts. These tests prove this recovery path,
not general autonomous backlog management or deployment evidence correctness.

Production automatically consumed the workspace recovery receipt and reserved a
new attempt for the original audit task, without an owner continuation or status
write from this session. Final verified outcome is still pending. The initial
release check incorrectly expected migration32 after applying33 and rolled back;
services remained active on the previous application release. Updated activation
schema expectation and its existing safety tests, then successfully reactivated
release27a7683 with healthy33-migration readiness. The deployment entrypoint now
runs activation safety tests before building/transferring so this mismatch fails
before production mutation. Do not interpret successful activation as completion
of the audit task or the overall autonomy goal.

## Separate customer reply attachment

Customer reply drafts are now separate attachments below the internal report,
with one preview, an internal visibility marker and a footer action. The footer
has no extra vertical padding; the button retains44px touch height. Using a draft
loads the public reply composer without sending. Extraction preserves trailing
internal task links in the owner report but excludes them from the visitor draft,
and retains subsequent sections. Stored messages are unchanged. Navbar unread
counts are explicitly exempted from the global square-corner CSS rule.

Focused parser/privacy regressions and the real frontend fixture pass. The
fixture verifies one draft preview outside the bubble, footer geometry, public
composer loading and circular single-digit unread badge. Mobile390px dark and
desktop1440px light screenshots were rendered and visually inspected under
`/home/dev/reports/cuan-pm-recovery/reply-draft/`; light/dark combinations pass.
Deployment proof validation remains separate unfinished work: this presentation
change does not validate historical claims in generated reports or draft text.

## Fresh owner correction cycles and concrete runner evidence

Production audit recovery reached a genuine execution, then failed the project
verifier: its retained test script referenced extracted local packages no longer
present in that workspace. It was not accepted or manually marked complete.
Its failed attempt still had only the generic error in coordinator review input;
the actual TestSummary was omitted. Review and saved recovery context now include
bounded runner verification output, and changed verifier evidence participates in
the review digest. This diagnostic data remains untrusted input, not commands.

A historical task's lifetime attempt total also blocked correction after a fresh
accepted owner continuation. Budget calculation now recognizes only a persisted
same-task launch tied to a real accepted owner message, matching scope, project,
room and owner, preceding the current attempt. That launch begins a new bounded
cycle; system messages, unrelated chat, canceled consent, future launches and
changed scope do not reset it. History is retained unchanged. Existing pure
infrastructure/cancellation exclusions remain in effect. Targeted tests verify
both cycle reset and exhaustion, conservative non-owner cases, verifier evidence
propagation and normal cancellation/continuation/recovery regressions. The review
policy digest changes once to permit existing unfinished results to be evaluated
under the corrected policy without injecting owner messages.

UI release3440fa7 is healthy with schema33. Actual production390px/1440px draft
screenshots were rendered and visually inspected; separate preview, compact44px
footer, internal marker and removal of internal links are verified. Historical
report claims remain unproven by this presentation change.

## Legacy assistant alias at durable reservation boundary

The production coordinator autonomously reviewed the failed audit again and
requested a concrete correction for the missing test packages. Its launch then
failed before reserving an attempt with `create attempt: Invalid task input`.
Official read-only inspection showed the saved launch and assignment used the
historical agent_sm alias, while execution resolved to agent_cuan. Storage
correctly rejected that mismatch. The dispatcher now resolves the public
assistant and saves the canonical assignment before attempt reservation, keeping
scope, consent, lease and cancellation checks intact.

A bounded recovery receipt also supports this exact pre-reservation error only
for known historical persona aliases, with a present enabled public assistant.
Custom-agent invalid input is not replayed. The original saved launch is reused;
no task or owner prompt is created by this session. Real server fixture tests
exercise stopped alias launch recovery through dispatch to exactly one canonical
attempt. Shared memory and isolated PostgreSQL contracts verify one-shot recovery
and rejection of unrelated errors; cancellation, scope changes and missing-worktree
regressions pass. Production completion remains unproven until observed.

## Card action alignment

Task preview and customer draft attachments share CardActions. Actions align at
the footer right edge with no vertical footer padding and44px minimum touch
height. Browser checks cover completed cards and working cards with a matching
live terminal session, alongside independent draft actions, at390px/1440px in
light/dark. Screenshots were visually inspected under card-actions and
card-actions-working in the private evidence root. No secondary explanation text
was added.

Production7ac82fd is healthy. Actual mobile390/desktop1440 screenshots confirm
right-aligned task/draft footer actions. The audit's previously rejected saved
revision now has one queued attempt on agent_cuan, automatically reserved from
its original launch; no manual prompt/status intervention occurred. It has not
yet reached a verified final result. Other actual project work remains active;
queued reservation is not evidence of completed execution.

## Single owner of Inbox outcome announcements

Runner failure/infra/DoD-review notifications were bypassing the coordinator and
sending generic owner-intervention options before the coordinator determined
whether the issue was agent-fixable. Inbox-linked tasks now use the coordinator
for those outcome messages, matching the existing success-announcement rule.
Runner completion state, activities and completion events are still persisted;
tasks without a source Inbox retain their fallback notification. Behavioral tests
cover failed, interrupted and canceled attempts, state-before-event ordering,
normal fallback delivery, and successful worker results awaiting acceptance.

Review instructions now request one to three user-facing sentences describing
outcome, concrete remaining issue and next action. Technical diagnostic output
and detailed revision steps remain in task evidence/revision. Existing structured
and legacy review parsing continue to pass targeted regressions. Live model
response clarity still requires observation; a prompt change is not proof that
all future replies meet the product standard.

## Grounded customer outcomes instead of repair templates

Accepted customer tasks previously received a hardcoded visitor draft claiming
that video playback or general service access had been restored, regardless of
the task's actual outcome. That template is removed. The customer-room result
uses the coordinator's accepted explanation and an actual saved draft from report
evidence, with internal task links kept outside the draft. An audit or unresolved
provider outage no longer invents a repair, release or visitor response. Historical
stored messages are unchanged. Parser, customer-result, Inbox review, same-task
recovery and cancellation regressions pass. This prevents new template claims;
it does not establish trustworthy deployment provenance for past acceptance.

Production83de458 is healthy and owns Inbox failure messages through coordinator
review. Official account diagnostics show one verified account, five quota-limited
accounts and two requiring login. Official runtime inspection confirmed a busy
runner with fresh heartbeat and output. The audit revision remains queued; this
is not a completed task, and no account preference, cancellation or task status
was manually changed to force it through.

## Balanced mobile Account navigation

The mobile Account avatar is28px with a quiet inactive surface and soft active
accent. All mobile icons share28px alignment slots while keeping their20px
artwork, so labels share the same baseline and touch targets remain56px.
Current section color and semibold text distinguish the active item; inactive
icons retain their section colors at reduced opacity, with no full-tab box.
Desktop avatar sizing is preserved. Real390px light/dark Projects and Account
screenshots plus1440px desktop were rendered and visually inspected under
account-nav. Browser assertions verify28px avatar geometry, aligned labels,
active aria-current, icon opacity and minimum touch height.

## Current acceptance audit — overall goal remains active

Productiona4a6f02 health matches the exact release SHA. Loaded (not skeleton)
Projects and Account pages were rendered and inspected at390px, plus loaded
Projects at1440px. Mobile Account is28px with aligned labels and distinct active
state; desktop remains36px. The new grounded customer result path is shipped.

Still required for the full project-manager outcome:
- Observe the original audit revision through real execution, concrete checks,
  coordinator acceptance and Inbox delivery; its latest observed attempt is queued.
- Demonstrate the full approved Cuan backlog advancing sequentially, not just a
  single recovered launch. A Dracin completion followed by a Dojo running task is
  observed handoff evidence, not independent acceptance of all their criteria.
- Replace keyword/prose deployment proof with trusted task/attempt/commit-bound
  live release evidence. Heru PR9 remains open and unmerged; genericHTTP200 does
  not establish deployment of its task commit. The native activation script still
  pre-marks task deployment before checking health and requires correction.
- Audit narrowed owner intent across automatic revisions. Exact-attempt policy is
  implemented, but a later generated revision may omit a prior owner's additional
  no-push/no-deploy limits. Preserve that authorization lineage without importing
  restrictions from a newer unrelated intent or silently expanding approved scope.
- Correct classification of audit-only work: inference currently sees snapshotted
  project metadata and incidental deployment words, which can select the code lane
  and require irrelevant project tests. Do not waive actual required checks; prove
  the execution scope and real changes before selecting verification.
- Observe real concise coordinator responses and absence of premature owner-help
  templates after failure. Fixture tests and prompt text are not universal proof.

No production backlog statuses, owner resume messages or visitor replies were
manually written in these recovery fixes. Keep canceled work stopped and retain
original files, approval and evidence while repairing the product itself.

## Scope-only execution classification

Execution and migration classification now exclude the project guidance snapshot
using the same marker parser that preserves workspace binding. Project goals,
standing instructions, auto_deploy defaults and environment key names remain in
the execution prompt, but cannot turn an audit into a code task or make every
project task a schema migration. Deployment-status nouns do not count as a deploy
action, and an explicit no-deploy restriction does not select the code lane.
An actual deploy command and real schema modification still select code.

Creation-path tests cover audit, deployment-status comparison, explicit deployment,
contact sync and genuine migration under deliberately code/migration-heavy project
guidance. Task concurrency and migration lane regressions, runner lane tests and
queued launch/recovery server flows pass. Existing saved execution categories are
not silently rewritten. Therefore this prevents new misclassification; old task
scope and verification selection still require evidence-backed recovery rather
than a manual category/status edit or bypass of required checks.

## Carry approved continuation boundaries into automatic corrections

The same persisted-owner launch lookup now supplies both review-cycle budgeting
and the binding scope for newly saved automatic corrections. A reviewer-generated
revision is appended after that approved continuation instead of replacing it.
This preserves full read-only/no-push/no-deploy feedback in the launch record,
runner context and existing exact-attempt publication policy. A genuine fresh
owner turn remains distinct; system, unrelated, canceled, future and changed-scope
messages cannot supply the owner basis. Storage failures stop preparation rather
than silently dropping constraints. Oversized combined revisions fail existing
validation instead of truncating approved instructions.

Tests persist a real accepted owner continuation, queue a system correction behind
an occupied project lane, inspect the saved launch's full read-only boundary and
both publication prohibitions, and verify a new accepted owner turn is independent.
Cancellation, queued launch, missing-workspace recovery and prior budget regressions
pass. Existing already-reserved launches are not rewritten by this change: their
historical authorization lineage and completion evidence still require audit.

Classification release6654889 is healthy on the exact production SHA. The original
audit's latest attempt remains queued at this observation; no final result or
full backlog acceptance is claimed.

## Native activation no longer pre-completes deployment tasks

Removed the native activation's pre-health writes that set attempts succeeded,
moved tasks to review, supplied success prose and stopped launches. A healthy
binary must now expose exactly the target SHA (missing SHA is a failure), with
existing readiness and HTML checks still required. After successful activation,
an optional task/attempt-bound deployment_verified receipt records that external
fact without changing work state. Deterministic receipt IDs make repeated writes
idempotent; an attempt belonging to another task is rejected. Missing evidence
never fabricates task acceptance, and receipt-storage failure still permits
worker drain cleanup without rolling back a healthy application release.

All19 activation safety tests pass, including failed-health/no-receipt ordering,
missing binding and exact SHA requirements. A real isolated PostgreSQL fixture
calls the actual receipt writer twice, checks the structured receipt, rejects a
foreign task binding, and compares full task/attempt/launch records before/after
byte-for-byte. Existing shared launch contracts also pass in that database.
Private log: native-receipt-postgres.log. This repairs native publication facts;
the runner's keyword-based deployment-proof consumer and generic external
provider attestation remain unfinished and must not be described as solved.

## Local worker completion also awaits Inbox acceptance

Broader focused delegation regressions exposed local reconciliation mapping an
agent's final done report straight to task Done. That bypassed coordinator review
and, because accepted work is protected, prevented the expected review from ever
being queued. Memory and PostgreSQL local reconciliation now return Inbox-linked
work to Review; tasks without an Inbox source preserve their existing behavior.
Only coordinator acceptance closes Inbox work. Late reconciliation cannot undo
an already accepted result. Replaying the same completed launch while review is
pending now correctly says already handled and never creates another attempt.

The synthetic owner-message authorization fallback in delegation is removed.
System review without the actual accepted owner approval cannot create a launch,
change assignment or reserve an attempt. Authentic continuations remain supported.
Runner notification tests were updated to the existing coordinator-owned message
contract, rather than expecting premature multi-room owner-help templates.

Memory and isolated PostgreSQL contracts compare review versus done behavior and
late-callback protection. Real local provider/process fixtures now assert Review
before dispatcher review and Done only after coordinator acceptance. Focused
legacy/topic delegation, managed-project, no-extra-planning, replay, queued launch,
cancellation, owner-budget and authorization flows pass. No historical production
Done states are rewritten or manually reopened by this change.
