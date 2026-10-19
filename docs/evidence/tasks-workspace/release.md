# Production release receipt

Released commit `88f79b1287cd39f1cb1af6370bbb45978ece8c17` to https://cuan.dojo.cc
on2026-10-19. Human commit author: Buchin. Native checksummed release activation
was healthy and retains previous release `d8a09a549ef253004207c84fcccb590905c97d58`.

## Completion audit

| Requirement | Evidence |
| --- | --- |
| Contextual list, sorting, URL filters, mobile/tablet/desktop | Fixture browser workflow and production viewport checks; `browser.json` |
| Read-first detail, explicit edit, separate fields, Markdown, checklist | Fixture save-error/draft/close protection; production create/edit/reload verified |
| Expand and return context | Browser URL/expand/filter/close workflow |
| Schema22→23 preserves data and rollback compatibility | Dedicated PostgreSQL migration/DBStore test; old production application healthy after explicit migration |
| Start/retry without duplicate execution | Real HTTP/PTY lifecycle and request replay tests; lost-response browser recovery uses same key |
| Concrete revision, preserved human instructions, no ghost attempt | Real lifecycle test created exactly two attempts; feedback saved as comment |
| Execution separate from approval | Exit0→Review; evidence/checklist gate; delayed reconciliation preserves Done; production manual approval verified |
| Durable sessions/output, read-only/input/reconnect/search/export | Real PTY watch/input/origin/session checks; browser live/history/reconnect/search/copy/download checks |
| Safe cancel/delete, comments | Browser confirmation and saved-output checks; actual production comment/delete; active-delete backend check |
| Project context and agent selection | Registered harness validation; selected project's local repository used; archived/unavailable repository blocked |
| Documentation | `docs/guides/tasks-workspace.md`, linked from README |
| Native deploy and retention | Validated private backup; atomic activation; exact public assets;35 retained tables and507 profile files unchanged |

## Production validation

- `/healthz` and `/readyz` healthy; DB connected;23 migrations;
  `CUAN_AUTO_MIGRATE=false`.
- Eight width/theme combinations across360,390,820,1440px passed. Saved fields,
  checklist, approval, comments and deletion used the real production API/DB.
- Exact synthetic task cleanup verified; the full table snapshot comparison
  passed. Only authentication-session rows are excluded from retention comparison.
- All built JS/CSS/font assets matched public bytes. Production screenshots use
  only the synthetic task, not private user content.
- Deterministic real PTY lifecycle tests also passed in isolation on the production
  host with temporary homes/workspaces and memory data. This verifies the native
  PTY environment without invoking a real provider or modifying production data.
- Production project checkout fast-forwarded to the released commit; no credentials
  transferred. Source tree was clean before deployment.

No paid/live provider acceptance call was made. Historical output is bounded by
server/terminal retention. Existing user content keeps its original language.
Two obsolete anonymous-launch test expectations are noted in `verification.md`;
the new authenticated lifecycle tests are the launch acceptance checks.

For application rollback, retain schema23 and activate the previous release with
`--schema-count 23`. Do not run destructive down migrations for an app rollback.
