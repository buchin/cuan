## Agent completion and progress UI

Fural source inspection confirms that agents may mark implementation Done without human approval (`agent_cli_tasks.go`, `agent_cli_agents.go`). Q&A/opinion/planning requests are coerced to Review.

Cuan now accepts an explicit `task report --state done` with summary, completion evidence and every declared acceptance criterion completed. The current attempt must finish successfully (including intentional reported PTY closure). Answers requesting Done become Review. Exit zero alone does not complete work. Failed/canceled attempts and delayed callbacks cannot approve newer work.

Other changes:
- Incoming left bubble for agent progress, replaced by the real response without a duplicate placeholder.
- All six task statuses remain visible; list refreshes every five seconds while visible and on return/focus.
- Live local terminal leases renew; dead sessions expire without restarting.
- Attempt updates reconcile the task board immediately and validate task/attempt ownership.

## Verification

Targeted Go tests passed for agent reports, explicit Done/Review/answer classification, live/dead leases, stale callbacks, task workspace launch/ownership, actual interactive PTY reporting and task CLI security. The PTY fixture covers Done, Review and Blocked; no real provider was invoked.

Frontend TypeScript/Vite build passed. Chromium fixture covers light/dark at 360, 390, 820 and 1440 pixels, session/result/history navigation and existing acceptance actions. Additional assertions verify six status options, polling Doing → Done without reload, and incoming progress/replacement at 390 and 1440 pixels. See `browser-check.cjs`, `browser.json` and screenshots.

## Production

Deployed to https://cuan.dojo.cc: code `84e65e07ba18f8cec93c86c00ef196ef56214dda` via the native atomic release script. The idle gate passed with zero active requests/attempts; validated database and account-profile backup retained privately. Readiness is healthy, schema remains23, automatic migration disabled.

Authenticated production smoke passed eight viewport/theme checks, current/earlier session navigation, acceptance gates, draft/start behavior and exact deployed asset comparison. Synthetic fixtures were deleted; no real provider or run-agent calls. The first pass timed out while waiting for saved terminal output; a repeat passed without code changes.

Isolated production-host PTY fixture passed Done/Review/Blocked reporting, two interactive turns, final output drain and progress/cancel guards. It used memory stores and a fake provider, without production database access. This proves runner/report handling; real provider adherence to reporting instructions was not tested.

Retention verification:35 existing tables unchanged (login sessions excluded),697 account-profile files unchanged. Persistent project checkout fast-forwarded cleanly to the deployed code. See `production-browser.json` and production screenshots.
