# Single-toolbar session workspace

Production: https://cuan.dojo.cc  
Release: `29770b2daeb39f5b698effca247ead1bed3bdf02`

## Shipped

- One session toolbar on every screen size. Duplicate Task session/New task
  global header hidden; terminal controls render in the session toolbar.
- Desktop sidebar defaults to icons during a session; expanding it does not
  change the ordinary navigation preference. Mobile bottom navigation stays
  hidden while viewing a session.
- One native terminal input for live sessions. The separate live message
  composer is removed. Keyboard toggle enables/disables input explicitly;
  reconnect still returns to read-only.
- Connection indicator is compact. Secondary search/copy/download/reconnect/
  follow controls open from the ellipsis menu; Escape or an outside click closes
  it. Detail remains a slide-in panel.
- Terminal fills the workspace without card margins or rounded corners.
- Initial task prompt keeps the user's request plus a short session guidance
  pointer. Reporting rules, original instructions, nonempty criteria and prior
  work/revision context live in a private per-attempt file. Existing project
  instruction files are not modified.
- Guidance distinguishes greetings/questions from implementation requests,
  requires file work in the explicit workspace, and prohibits investigating the
  server binary to discover the reporting contract.
- Antigravity receives the explicit workspace via --add-dir in addition to cwd.
  Reporting --help includes an optional JSON example; metadata/checklists are
  optional for simple results. Final report/approval/cancellation gates retained.

## Evidence and verification

The user's prior hello-world task report was read for diagnosis. It recorded
creating a Python app in the account-profile scratch directory. This release
addresses that observed scope/workspace detour through explicit guidance and
workspace registration; it does not delete or relocate the user's existing work.

- TypeScript/Vite build passed.
- Targeted task-agent, workspace, CLI, manager, PTY and recovery tests passed.
- Reporting fixture checks private guidance permissions/cleanup, short visible
  prompt, no credential in guidance, actual guide readability from the PTY,
  explicit cwd/add-dir agreement, documented reporting JSON and two turns.
- Isolated native PTY tests passed on the production host with fake provider,
  temporary HOME and in-memory stores; no production database/provider calls.
- Installed production CLI supports interactive, permission and add-dir flags.
- Fixture browser checks passed 360/390/820/1440px light/dark plus360x640:
  single toolbar, terminal region at least750px at900px viewport, icon sidebar
  expand/collapse, native input without second composer, report drawer, filters,
  session history, same-request launch recovery, reconnect and cancellation.
- Authenticated production checks passed eight width/theme combinations,
  single toolbar, collapsed desktop sidebar, no duplicate live composer,
  history navigation, missing-session handling and exact deployed assets.
- Production smoke created and cleaned one synthetic task/two finished attempt
  records. It did not call run-agent or any real provider.
- Native atomic release passed health/readiness. No migrations; schema23,
  auto-migrate false.
- Validated private backup before activation; no active work at the gate.
  After smoke,35 retained tables and631 account-profile files matched snapshots
  (authentication sessions excluded).
- Production project checkout fast-forwarded fromf1e9a5e to29770b2.

## Limits and rollback

New prompt guidance applies to new attempts. Existing terminal recordings remain
unchanged. Live model adherence to the proportionality/workspace guidance has
not been exercised by an automated paid-provider call; deterministic tests prove
the delivered guidance and actual tool/PTY plumbing.

Use the documented native previous-release rollback and retain schema23.
