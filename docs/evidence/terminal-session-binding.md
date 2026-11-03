# Terminal session binding checkpoint

The terminal now rejects reconnects to a replacement local session, selects the
explicit runner attempt socket, and validates that an attempt belongs to the
requested task and is running before attaching. Runner handshakes use the same
`session_id` field as local terminal handshakes.

Unsent browser input is bounded and retained for the same task/attempt generation.
A successful WebSocket handoff is never replayed; this is not an acknowledged or
exactly-once delivery protocol. Changing task or attempt discards unsent bytes and
ignores the old attached session identity. Disconnected input has no local echo.

Validation:

- `go test ./internal/terminal ./internal/server -run 'TestHandler_(Reconnect|WebSocket)' -count=1`
  passes terminal tests; no server tests matched this filter.
- `cd web && npm run typecheck && node --test tests/terminal-input-buffer.test.mjs`
- `cd web && npm run build`
- `CUAN_VISUAL_OUTPUT=/home/dev/reports/cuan-terminal-session-binding node docs/evidence/accounts/model-picker-check.cjs`
  passes browser fixture checks at 390px and 1440px, including stdin frames.

Screenshots were rendered and inspected. The existing mobile terminal composer
clips its placeholder/controls and still needs a layout correction; this checkpoint
does not pass the complete UX quality gate. Browser fixtures do not prove a native
provider terminal works in production. No deployment was performed while the native
E2E attempt was running.

Native E2E task `msg_874b8618-742b-4239-85d5-e59bf6598e8b` has a third running
attempt `att_6ebc59aa51ce3c67f138e34526e80d59`. The actual guide artifact and final
Inbox result remain unverified. Continue this task rather than creating a duplicate.

## Follow-up: mobile composer and Inbox header

The terminal composer now gives its text area a full mobile row, with 44px action
controls below it, and retains the single desktop row. Mobile input uses 16px text.
Build and fixture browser checks passed. Rendered screenshots at 390/1440px were
inspected in `/home/dev/reports/cuan-inbox-header-cleanup/` and the earlier mobile
placeholder clipping is resolved. Production native session verification remains
pending.

The Inbox header overflow menu was removed: its only action duplicated the
existing Mark all as read button, and it was empty with no unread conversations.
The existing button remains conditionally visible when there are unread messages.
The fixture checks assert the redundant Inbox menu is absent. Conversation row
menus remain available.

## Contacts toolbar cleanup

Removed the list-level Contacts overflow menu and its sole Reload contacts action.
The existing visible-page 30-second refresh, window-focus refresh, post-change
refreshKey and error-state Try again remain. Per-contact action menus remain.
Build and browser fixture passed; screenshots at 390/1440px were inspected at
`/home/dev/reports/cuan-contacts-cleanup/contacts-clean-{390,1440}.png`.
This change is not deployed yet.

## Native E2E result and reporting corrections

The third attempt succeeded at 2026-11-04 01:50:40 +08. Inbox message 17 delivered
`studio-to-result-id.md`, attachment `e783e6d7-ed93-4444-a085-6801b5dbe03c`;
message 18 reported completion in the same conversation. Downloaded via official
CLI and read independently: 2952 bytes, 406 whitespace-delimited words, all six
requested stages, weekly sales recap example, Indonesian nontechnical prose.
SHA256: `aa9ca0299d66719ad5322423f68c18a8211dafe15c2f9667b830bed21635f52a`.
Stored privately in `/home/dev/reports/cuan-chat-e2e/verified-studio-to-result-id.md`.
The runner reports a Markdown-only single-file diff. That report alone is not an
independent audit of no push/deploy/external effects or all guide UI claims.

The non-code evaluator previously fabricated fixed RCA, reply-draft and no-side-
effect evidence. It now lists recorded checklist evidence, identifies this as an
acceptance-evidence completeness check and states the independent-verification
limits. Unrun workspace checks are no longer marked true in this branch. This does
not add a semantic artifact verifier; that broader goal remains incomplete.
Completion messages preserve the result, offer revision in the same conversation,
and no longer append generated promotional recommendations or invent verification
when a summary is missing. Existing production messages are not rewritten.

Added official `cuan task logs TASK_ID ATTEMPT_ID` with explicit session selection,
authenticated request, cursor, bounded record count/body and no error-body output.
Targeted authenticated, invalid-range and failure-response tests pass.

## Approved Inbox refinements

Conversation rows remove repeated agent badges, generic `general` subjects and
agent presence dots (connection readiness is not presence). Titles stay one line,
previews two lines; unread uses one count and selected backgrounds are softer.
Known launch-error boilerplate uses a human-readable saved-work preview. Task-card
actions have a footer with a muted divider and 44px touch targets. Long messages
collapse based on measured rendered height (nine text line-heights), recalculate
on resize/font load, and have an accessible expand/collapse control without a fade.
Sibling task cards remain visible. Legacy embedded cards prevent text collapsing.

Frontend build, targeted runner/CLI tests and browser fixture checks passed.
Screenshots at 390/1440px were rendered and inspected in
`/home/dev/reports/cuan-inbox-refinement/`. This fixture is not production Studio
entry or a native production terminal intervention/reconnect test.

## Final-acceptance ordering regression

Later production state (snapshot 80) contradicted the initial success: the Inbox
coordinator changed the task to blocked at 01:56:18 +08, after the runner had
announced completion at 01:50:40. The runner had replaced the original detailed
native report with the non-code evaluator's generic note. The guide is delivered,
but end-to-end acceptance is therefore not achieved.

Inbox-sourced successful execution now remains in review until the coordinator
accepts it, suppresses the premature runner completion announcement and preserves
an existing native verification note. Standalone completion keeps its prior flow.
Regression checks cover concurrent duplicate completion (one transition, no early
announcement), preservation of native evidence, and succeeded attempt versus task
review state. Targeted runner and Inbox review tests pass. This fixes ordering and
evidence loss; the existing production task is not manually relabeled as passed.

## Evaluation progressive disclosure

Inbox report messages now render a plain historical-review summary and a 44px
Review details action. Full original report evidence opens in SlideDrawer instead
of a StatusReportCard nested in the message bubble. Historical summaries direct
readers to the task card for current status rather than claiming an old evaluation
is current. Build and browser fixtures passed; mobile390 and desktop1440 summary
and drawer screenshots rendered and inspected in `/home/dev/reports/cuan-review-summary/`.

## Deployment and footer spacing follow-up

Release ee60f86f867f5d1a058fb58b7e15f5739d015cb7 deployed successfully. Healthz
matched the release; both active server and runner /proc/MainPID/exe paths matched
its release directory. Deployment frontend gate passed419 tests. Evidence:
`/home/dev/reports/cuan-inbox-refinement-deploy-retry.log`.

Restored the native E2E report with independent artifact observations in the task
complete_note, leaving task status blocked. Same-room review request
bb4529d3-2669-4832-99c8-d3ae2e957bec was running at snapshot84, with explicit no new
attempt, no recreation and no external-effects instructions. No acceptance claimed.

Approved footer spacing follow-up removes compounded card bottom padding, keeps
12px content spacing and uses4px footer vertical padding with44px action targets.
Build and rendered browser fixture passed,390/1440 screenshots inspected at
`/home/dev/reports/cuan-footer-spacing/`. This spacing follow-up is not yet deployed.

## Re-review after restored evidence

Request bb4529d3 completed with a narrative review but did not change the blocked
task: ordinary chat review is separate from the trusted automatic acceptance path.
The reply cited administrative task state as a gap and referred to the main dev
checkout, which does not independently establish the attempt workspace state.

Automatic review dedup previously allowed only one completed review per attempt,
even after its evidence was corrected. Completed reviews now allow a new trusted
review keyed by a digest of summary, original verification note and checklist.
Status/timestamp changes do not create a new digest. Identical evidence deduplicates;
decisions bind to the current digest. Legacy snapshot recovery is retained. The
review prompt distinguishes board state from acceptance and does not require
publication unless the approved task requires it. Targeted server Inbox review,
review binding and delegation tests passed, plus new evidence-revision tests.

## Accepted native result and account/navigation follow-up

Release34b31a59cacaafed58722ac0025f3be0706ebaa6 deployed successfully with matching
health SHA (`/home/dev/reports/cuan-rereview-deploy.log`). After re-reviewing restored
evidence, automatic Inbox request10c73583-32a5-4acf-afb6-884e95c899e3 completed;
message22 reported acceptance and snapshot89 confirms task done, using the same
third attempt. This is acceptance of the native Inbox-created documentation task,
not proof of the full production Studio UI entry or native terminal intervention.

Approved UI follow-up: account trigger avatar36px and menu avatar40px, name14px
medium/email12px, minimum56px trigger. Shared page headings20/24px and Inbox metadata
12px sans-serif. Unread count gets a muted filled badge. Navigation icons and
normal status dots use shared low-chroma light/dark tokens; labels/surfaces remain
neutral. No new theme-selection preference added.

Build and419 frontend tests pass. Browser fixture verifies36px avatar/minimum56px
trigger, menu opening, no viewport overflow and existing Inbox/terminal interactions.
Screenshots390/1440 in light/dark rendered and inspected at
`/home/dev/reports/cuan-wayfinding-type/account-menu-{light-,}{390,1440}.png`.
This is a shared-component refinement; no claim that every legacy10px label in
all application pages was audited or removed.

## Production Studio UI entry verified

Production browser UI submitted the distinct checklist idea from Studio and
navigated to Inbox room2a35dcdd-93a5-4cb2-aeff-73cad784c2b7. Initial UI attempts
were gated by setup loading and did not submit; the successful run waited for the
actual Send button to enable and clicked it. Receipt and390/1440 screenshots are
stored privately in `/home/dev/reports/cuan-chat-e2e/production-studio-*` and
`studio-native-receipt.json`. No duplicate task or conversation was created by retry.

Request97aba364-5a3d-4383-85ac-e230ef23fcc3 completed with two clarification topics:
reader and preferred checklist format. Task count remained0. Requirements supplied
in the same room: nontechnical reader,5 practical points, Indonesian<=200words,
one Markdown file in existing Cuan project, attach result; no code/config/test,
push/deploy/external messages. Explicitly requested a plan before approval.
This second Studio-origin scenario is still in progress.

Approved spacing follow-up keeps8px sidebar inset, removes permanent account-active
background, sets composer gutters12px mobile/16px desktop and reduces Inbox search
vertical spacing. Build and browser fixtures passed;390/1440 screenshots rendered
and inspected under `/home/dev/reports/cuan-gutters/`. Not deployed yet.

## Studio plan approved; task-card terminal availability

Studio-origin request a9cc4b8c completed with a plan and checklist, without creating
a task. Plan covers five practical acceptance points, Indonesian<=200words,
one docs/guides/checklist-penerimaan-id.md in existing Cuan project, Inbox delivery,
no code/config/test or publication/external effects. Approved via same-room chat
and requested exactly one delegated task with monitoring and verification.
Approval receipt:97-studio-approval.json in the private E2E report directory.

Task attachment preview now checks runtime availability only for a running current
attempt, polls every5seconds while mounted/running, and hides Terminal for queued,
finished/canceled or missing/mismatched sessions. Runtime endpoint now recognizes
an actual listening decoupled runner attempt socket (previously only local-manager
sessions). Explicit attempt attach remains fenced server-side. This is availability
detection, not a production stdin/intervention/reconnect proof.

Project gets a folder icon (FolderGit2 only when task repo_url identifies GitHub);
status has a small semantic icon or working StatusDot in a muted badge. Backend
regression with a real PTY/socket verifies queued/finished rejection and current
running session recognition. Build and browser fixture pass. Queued/completed
screenshots390/1440 inspected at `/home/dev/reports/cuan-terminal-availability-final/`.
