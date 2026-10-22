# Production release

Released `f4c8220f1575970e85f26ee288b7488563350eab` to https://cuan.dojo.cc
on2026-10-19 using the native checksummed atomic deployment. Previous release:
`88f79b1287cd39f1cb1af6370bbb45978ece8c17`. No database migration required.

## Verified outcomes

- Desktop table rows are compact; tablet/mobile retain readable task rows and
  timestamps. Mobile search has more room, with status selector and Sort menu.
- Filter chips support individual removal. Inline status and assignment support
  stale-edit errors and preserve approval/active-session gates.
- New task is a simple modal composer inspired by Fural: Project selector above
  one prompt, optional Agent and Create action inside. No separate required title,
  priority or checklist fields. Initial title comes from the prompt's first line;
  the complete instructions are preserved.
- Not now saves to Backlog. Explicit Create & start launches the selected agent.
  Browser fixture proved one attempt after lost launch response and reload;
  recovery used the same request ID. Create-response-loss recovery also retained
  the original create key after reload. Project defaults and discard protection
  passed.
- Vite/TypeScript build, targeted service/HTTP lifecycle tests and eight browser
  viewport/theme combinations passed. Existing detail/session/terminal/revision/
  approval workflows passed the fixture regression checks.

## Production smoke and retention

Actual production SQL-backed create, edit, reload, inline status changes,
review/approval, comment and confirmed delete passed. Eight responsive/theme
combinations passed. All public JS/CSS/font assets matched the release build.
Only exact synthetic task rows were created and cleaned; no live provider call
was made. Explicit create/start was verified with deterministic fixture agents,
not a production provider.

Validated pre-release private backup contains36 table snapshots and507 profile
files. After smoke,35 retained tables matched exactly (login sessions excluded)
and all507 profile files were unchanged. Schema23 remains installed and automatic
migration stays false; readiness is healthy with database connected. Persistent
project checkout fast-forwarded to the release commit without credentials.

Public screenshots and `production-browser.json` contain only synthetic task
content. For application rollback, activate the previous release while retaining
schema23 (`--schema-count 23`).
