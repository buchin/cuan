# Execution lifecycle and shared UI verification

2026-11-03. This release follows the six-task lifecycle RCA and compares Fural's
local task-detail source with Cuan's task/session routing. Fural was inspected in
source, not claimed as a live browser comparison.

## Checks

- Targeted lifecycle regressions: stale, expired, foreign, superseded and canceled
  reports; concurrent duplicate completion; unchanged task/branch on retry;
  heartbeat fencing; assignment release; per-criterion evidence; shared deadline.
- Real isolated PostgreSQL migration and lifecycle test: saved deadline, atomic
  progress reports, simultaneous completion from two service instances, expiry,
  duplicate rejection, and retained history. `TestExecutionLifecyclePostgres`
  passed. The fixture database was stopped after the test.
- `TestTaskAgentScopedReportAndReviewGate` passed with explicit fresh summary and
  evidence. Task-agent unknown-capability shortcut was removed and covered.
- Final targeted task/runner/server checks passed; server and runner binaries
  built successfully. Full Go suite and Go race testing were not run.
- Frontend: 424 tests passed, TypeScript and production build passed.
- Real Chromium rendering at 1440×1000 and 390×844: Projects, Tasks, Contacts,
  Inbox, task overview and explicit session page. Screenshots inspected; no
  horizontal document overflow. Search height 44px; shared header 56px mobile,
  52px desktop. Found and fixed the old substring height override and HSL/OKLCH
  mismatch during visual inspection.
- Browser interactions: keyboard Home/End on project tabs; original instructions
  disclosure; long title contained in header; incomplete historical result
  approval disabled. Authenticated reads used real production data with local
  built UI; mutations and streams stubbed. This is not a chat execution E2E test.

Local raw evidence: `/home/dev/reports/cuan-shared-ui/`,
`/home/dev/reports/task-rca/sql-lifecycle-results.txt`. Do not infer that historical
failed work was rerun or completed. Reported criterion evidence still requires
independent verification; this release does not claim a new deployment-proof
attestation system.

A browser fixture changed only the read response for the public assistant name
to `@nara`: task guidance displayed that configured name at desktop and 390px,
and the technical account selector was absent. Production settings were not
changed. Start/retry/follow-up target the public assistant ID, including old tasks.
The initial release was safely rolled back after the deployment gate still
expected schema 31; the gate now expects 32 and a regression compares that value
with repository migrations. Native redeploy and public health/readiness passed.
