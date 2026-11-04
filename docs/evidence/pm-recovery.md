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
