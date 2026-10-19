# Direct task creation release

Production: https://cuan.dojo.cc  
Release: `1bd84f11d75c8e03fbb2e6d50d98f165add0ab96`

## Result

New task keeps one prompt, Project, Agent and Create. With an agent selected,
Create navigates immediately to the session's Starting page and replaces it with
the confirmed attempt. There is no task-detail intermediate step. Project and
Agent preferences persist; an available verified agent is preferred initially.
No agent remains an explicit save-only option.

Pending launch request IDs survive reload. Reload does not automatically replay
a launch; Check session uses the same request ID. Leaving during launch does not
redirect back. Returning after completion resolves the cached matching attempt.
Existing agent report and human approval gates are unchanged.

## Verification

- TypeScript and Vite production build passed.
- Deterministic browser fixture passed eight viewport/theme combinations
  (360, 390, 820, 1440px; light/dark), plus 360x640 terminal usability.
- Verified direct Starting without task drawer, confirmed Running transition,
  preference retention, interrupted-response recovery, no replay on reload,
  same request ID, leaving during launch, returning to the matching attempt,
  save-only mode, retained prompt/create key, session history, reconnect and cancel.
- Production authenticated UI smoke passed eight viewport/theme combinations,
  minimal composer, prompt metadata, saved session history, reload/back/missing
  session behavior and byte-for-byte deployed assets.
- Production smoke used one synthetic task and two synthetic finished attempt
  records; all were cleaned. No run-agent request or provider invocation.
- Native atomic release activated successfully; readiness reports database
  connected and schema23 applied. Auto-migrate remains false; no migration.
- Private backup validated before deployment; no active inbox/task work.
- After smoke,35 retained database tables and525 account profile files matched
  their pre-release snapshots. Authentication sessions excluded.
- Production project checkout fast-forwarded from e294b09 to1bd84f1; no existing
  workspace changes or credentials overwritten.

## Limits and rollback

Direct launch behavior is verified with deterministic API/terminal fixtures,
not a live paid provider. Production verifies the released UI and existing
session records. Existing ten-minute task runtime and explicit human result
approval still apply.

Use the documented native previous-release rollback; retain schema23.
