# Task session workflow

Authorized directly by the user, outside Scrum. Repository: Cuan.

## Acceptance

- Prompt-only creation derives initial metadata server-side, preserves full human
  instructions, and keeps durable create-request recovery after metadata changes.
- Create opens the persisted task immediately; background launch shows progress,
  automatically navigates to the returned session only while the user stays on
  this task, and leaves truthful errors/recovery without duplicate execution.
- Sessions have standalone URLs, reload/back/history navigation, live/saved output,
  retained controls, mobile report tabs and desktop report panel. Browsing does not
  spawn work; a missing session never falls back to another run.
- The self-contained server binary provides a task-bound reporting CLI. Capabilities
  are private, expiring, tied to the active task/attempt, revoked on exit and cannot
  access other tasks, human instructions, project/assignment or Done.
- Agents refine metadata/criteria and send plans, progress, summary and verification.
  Only a successful process with this attempt's complete review report enters
  Review. Missing reports, blockers, failures and cancellation remain Blocked.
  Earlier evidence cannot qualify new work; human approval gates remain in force.
- Targeted service/CLI/HTTP/real PTY checks, browser workflows and isolated PostgreSQL
  verify behavior. Native production deploy uses private backup/idle/health gates,
  exact-asset smoke, synthetic fixture cleanup and retained-data audit. Document
  usage, limitations and application rollback on schema23.

## Evidence

Local targeted tests and fixture browser workflows passed. Dedicated PostgreSQL
round trip verified actual report activity persistence and Review reconciliation.
Production release evidence is recorded after activation in
`docs/evidence/task-workflow/release.md`.
