# Interactive session workspace release

Production: https://cuan.dojo.cc  
Release: `f1e9a5ee89d8abc7e0385377b1f7a67f215d62d8`

## Shipped

- Task Antigravity sessions use `--prompt-interactive` with authorized tool
  permissions, replacing single-turn `-p`. Claude and Codex also launch their
  interactive modes. Inbox retains its separate restricted launcher.
- Browser disconnect and reconnect attach to the same PTY. Sessions are bounded
  to eight hours; task/attempt capabilities expire and revoke on termination.
- Validated final review/blocked reports close only the matching live attempt
  after a two-second drain. Later progress or user input prevents a stale close.
  Actual exit codes remain in history and the durable attempt audit. Intentional
  report termination is distinct from process failure and user cancellation.
  Human approval remains required for Done.
- Session UI prioritizes terminal space: compact title/status/back/detail/stop,
  no mobile global header or bottom navigation, secondary terminal tools folded.
- Detail slides in with session selection, full error, timestamps and reports,
  keyboard focus management and reduced-motion support.
- Explicit Enable input allows terminal keyboard control and a message composer
  targeting the same live process. Completed latest attempts can start a new
  follow-up attempt with retained task context; older attempts stay read-only.
- Existing failed sessions remain historical records; they are not auto-replayed.

## Verification

- Targeted terminal, task service, reporting, task workspace, scoped credential,
  HTTP/WebSocket and cancellation checks passed.
- Real PTY fixture asserts interactive argv, accepts a second turn, invokes the
  actual task capability CLI, retains final output and reconciles review/blocked.
- Tested stale attempt protection, newer user input/progress protection, failure
  after a report, cancellation and absence of a review-ready report.
- The interactive/finalization fixture also passed on the production host using
  an isolated test binary, temporary HOME, fake provider executable and in-memory
  stores. It did not access the production database or invoke a real provider.
- Installed production Antigravity CLI help confirms both interactive and
  permission flags are supported.
- TypeScript/Vite build passed. Browser fixture passed eight width/theme checks
  (360/390/820/1440; light/dark), plus360x640, message composer, follow-up attempt,
  detail drawer, existing launch recovery/history/reconnect/cancel flows.
- Authenticated production UI passed eight width/theme checks, terminal area at
  least600px tall at900px viewport height, hidden mobile bottom navigation,
  detail/history navigation and exact deployed assets.
- One synthetic production task and two finished synthetic attempts cleaned.
  No production run-agent call or provider invocation during smoke.
- Native atomic deployment passed health/readiness. Schema23 retained;
  auto-migrate false, no migration.
- Validated private pre-release backup with no active inbox/task work.
  Post-smoke retention:35 tables and534 account profile files unchanged,
  excluding authentication sessions.
- Production project checkout fast-forwarded from1bd84f1 tof1e9a5e.

## Limits and rollback

Live paid-provider behavior was not exercised. Multi-turn execution and report
cooperation were verified using a deterministic executable in a real PTY,
including on the production host. Claude/Codex interactive launch paths compile
but were not authenticated against live providers.

The report's saved evidence is authoritative; the final report initiates process
closure after the drain. Runtime execution uses the configured service account,
not a newly introduced OS sandbox.

Use the documented native previous-release rollback; preserve schema23.
