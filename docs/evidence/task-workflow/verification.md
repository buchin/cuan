# Task workflow verification

- Vite/TypeScript production build passed.
- Targeted task service, metadata/idempotency, CLI, authenticated HTTP, real PTY,
  terminal-manager and chat-to-task regression checks passed.
- Scoped-report tests reject cross-task access, unscoped credentials, human-body
  edits, Done, missing result evidence and reports after attempt termination.
- Actual fixture processes used the self-contained CLI to report over HTTP.
  Successful complete reports enter Review after exit; missing reports, explicit
  blockers and nonzero exits stay Blocked. Human instructions stay intact.
- Dedicated PostgreSQL migration/storage test passed with actual report activities,
  summary/evidence/checklist persistence and late-reconciliation preservation.
  Existing schema23 suffices; no new migration.
- Browser checks passed360/390/820/1440px and light/dark themes. Detail opens while
  launch is pending, then navigates to a standalone session. Reload/reconnect,
  report panels, missing-session state, cancellation/history, same-key recovery,
  and save-only draft recovery passed. See `browser.json`.
- Isolated PTY/report tests also passed on the production host with temporary
  homes/workspaces and memory data. No provider calls or production-data changes.
- Private production backup validated with36 table snapshots and525 account-profile
  files; zero active inbox requests/task attempts before release preparation.

Production activation and public smoke are recorded in `release.md` after deploy.
