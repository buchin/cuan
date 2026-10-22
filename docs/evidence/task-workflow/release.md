# Task session workflow release

Production https://cuan.dojo.cc runs `e294b096d295fef1e65b0f46b25046b855a44835`.
Native checksummed atomic activation is healthy. Previous application release:
`fb2b6ec768be8207a18a2021fc495bd4104d1ece`. Schema23 is unchanged and automatic
migration remains false. Human-authored source commits are pushed.

## Requirement audit

| Requirement | Implemented behavior and evidence |
| --- | --- |
| Prompt-first metadata | Server derives initial title/priority; complete instructions unchanged; metadata/idempotency service and production API smoke passed |
| Immediate creation | Detail appears while launch response is deliberately delayed; background response navigates to the returned attempt's page |
| Dedicated sessions | `/tasks/{id}/sessions/{attemptId}`; strict identity, reload, back, session selector/history, live/saved terminal and mobile report tabs |
| Agent task tools | Built into server executable; private loopback capability scoped to active task/attempt; no extra installation or owner credential |
| Restricted writes | Negative HTTP tests reject another task, human instructions, approval/Done, incomplete result and ended attempt |
| Agent administration | Title/priority/criteria refinement, progress/plan, summary, evidence and verification references; final snapshots retained in activities |
| Truthful result | Real fixture processes report through the CLI; only complete report + successful exit enters Review; missing report, blocker or failed exit stays Blocked |
| Approval | Agent cannot approve Done; existing human evidence/checklist approval and revision gates remain |
| Recovery | Durable request IDs; lost create/launch responses recovered without a second task/process; reconnect only attaches |
| Mobile usability | Eight theme/width variants plus360×640 check with at least180px actual terminal canvas; short screens scroll vertically |
| Data storage | Isolated PostgreSQL actual report activity + metadata/checklist/reconciliation checks passed; no migration needed |
| Documentation | `docs/guides/tasks-workspace.md` includes user flow, scoped tool usage, capabilities and rollback |

## Production checks

- Eight viewport/theme combinations verified real task creation with server-side
  metadata and standalone session pages, history URL changes, reload/back and
  missing-session behavior. All public JS/CSS/font bytes matched the build.
- One exact synthetic task and two explicitly synthetic finished attempt records
  tested historical session UI; all were removed. No provider was invoked by
  production smoke. These records never represented real execution.
- Real PTY/CLI/report/permission tests passed locally and in isolation on the
  production host, using temporary homes/workspaces and memory stores. PostgreSQL
  reporting persistence was verified separately in a dedicated test database.
- The actual deployed executable's `task --help` succeeded without setup, proving
  the agent tool ships in the production binary. No live-provider acceptance call
  was made; provider cooperation is constrained by the reporting gate.
- Private backup validated before release. After smoke,35 retained data tables
  matched exactly (authentication-session rows excluded), and all525 account-profile
  files matched. No retained rows or profiles were changed by verification.
- Production project checkout fast-forwarded to the release commit; no credentials
  transferred. Readiness reports database connected and23 migrations.

Rollback: activate the previous application release while retaining schema23
(`--schema-count 23`). Existing report activities and task fields remain stored;
normal rollback does not run down migrations or replay attempts.
