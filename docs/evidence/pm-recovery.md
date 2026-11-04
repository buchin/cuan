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
