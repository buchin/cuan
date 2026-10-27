# Tasks usability verification

Implemented after reviewing Fural task-list components and the actual new-task
composer/repository-picker source. Cuan uses Project context.

- Desktop retains a compact table; tablet/mobile use task rows with timestamps.
- Status cues include icons and restrained colors. Inline status and agent
  assignment preserve stale-edit and active-execution guards. Approval stays in
  detail; changing status never launches an agent.
- Active project/priority/assignee chips can be removed individually. Mobile
  search has more space, a Sort menu and a fully visible status selector.
- New task is a simple composer modal: one prompt, Project, optional Agent.
  No required title/priority/checklist form. First line supplies the initial title;
  the full prompt remains the human instructions. Not now saves to Backlog;
  Create & start explicitly launches the chosen agent.
- Browser-tab draft/create key persists through reload. Discard requires
  confirmation. Create failure retries the same key. A lost launch response opens
  the saved task with same-request recovery, without duplicating execution.
- Project-scoped creation uses the project ID directly from the route, including
  while project metadata is loading.

Vite/TypeScript build and targeted Tasks service/HTTP lifecycle tests passed.
Browser fixture passed eight viewport/theme combinations and complete editing,
filter/sort, inline success/failure, prompt creation/reload recovery, explicit
create/start, terminal, revision/review, comments and deletion flows. Real providers
were not invoked by tests. See `browser.json` and the generated screenshots.

No migration is required; schema23 remains installed. A validated private
production backup contains36 table snapshots and507 account-profile files.
Production activation, public asset checks and retained-data verification are
recorded in the release receipt after deployment.
