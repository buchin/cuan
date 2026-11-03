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

## Studio delegation accepted and queue diagnosis

Approval request cd7af092 completed, creating exactly one task
msg_734e0ee4-1269-4bb9-ad4b-1ae5012fe753 in Cuan project
proj_4978b0b914eaaf0d588df037977fb652. Attempt
att_b8dd61816a77b4a3c4d039191a67f96a is queued; no replacement requested.
Snapshots103/104/106 confirm binding and lack of execution start.

Official MCP read-only database diagnostics107/108 show runner-01 busy with an
existing running attempt in another project; heartbeat/lease are renewed. That
other task reports done but its attempt has not finished. It was not canceled or
restarted to make room for this test. Queue waiting is therefore backed by current
capacity evidence, not an assumed worker outage.

Launch confirmation now distinguishes a queued attempt (Work queued) from a
started attempt (Work started) after the launch was accepted. The interruption
recovery success recognizer and chat boilerplate suppression recognize the queued
confirmation. Targeted server delegation/launch tests passed. This wording follow-
up is not deployed yet. Full Studio-origin execution/artifact/acceptance pending.

## Report confirmation across server restart

A code-level recovery gap was reproduced: report confirmation depended only on an
agent capability kept in server memory. An HTTP401/403 after server restart made
confirmation return false even when the bound worker report was stored. Added a
read-only runner-authenticated report endpoint scoped to task, latest running
attempt, assigned runner, unexpired lease and non-cancellation. It returns only
summary, verification note, checklist and same-attempt report activity metadata;
no instructions/environment/token or replacement write grant.

Runner confirmation falls back to this endpoint only on capability401/403 and
only with configured runner credentials. A progress-only report does not become
completion. Tests cover grant-loss recovery, preservation of evidence, runner
mismatch, unauthenticated access, finished attempts and omitted private scope.
Targeted server+runner tests pass. This is a reproduced reliability defect, not
proof that it is the sole production queue cause. Existing older detached workers
are not hot-patched or canceled by this change.

## Release transfer timeout

Deployment c9df9e5 local command reached its300second limit before activation.
Production current symlink and both service binaries remainedfeec9aa and active.
No remote scp/rsync/activation process remained. Local payload44MiB, remote staging
28MiB and target release directory empty proved an interrupted transfer, not a
failed activation. Do not claim this release deployed or restart a live activator.

The maintained deployment script now transfers payload via compressed rsync with
partial-file append verification. Existing versioned staging, manifest checksums,
atomic activation and rollback gates remain. Shell syntax check passed; next full
deploy must verify transfer and activation in production before success is claimed.

## Successful release and production inspection

Native rsync3.5.0 and its Alpine runtime libraries were installed only under
`/home/dev/.local/rsync-native` from the official Alpine package repository, without
changing the ambient compatibility wrapper. With that binary selected on PATH,
maintained deploy script completed in372seconds, preserving all manifest/atomic/
health gates. Production7b7eef5e56906a9c1cf86e5e2de15468d4741982 health matched;
both active service /proc/MainPID/exe paths matched release binaries. Evidence:
`/home/dev/reports/cuan-native-rsync-release-deploy.log`.

Real authenticated production browser screenshots390/1440 were captured and
inspected for the Studio-origin task. Card showed Queued, folder/status indicator,
Detail only,0Terminal buttons and no horizontal overflow. Evidence:
`production-studio-task-render.json` and `production-studio-task-{390,1440}.png`.

## Queue time incorrectly counted as execution time

Snapshot114 contradicts the earlier assumption that the same queued attempt would
be retained: initial attempt att_b8dd618 was reaped for an execution deadline even
though started_at was null. Automatic coordinator review queued replacement
att_443069f2d613c0df324134fb110fab5d within the same task. No execution duplication
is established, but queue waiting incorrectly consumed an attempt and caused a
misleading failure/revision message.

The zombie sweeper now skips queued attempts: claim sets the immutable execution
deadline when work actually starts. Regression checks cover aged queued attempts,
legacy queued deadlines and actual running deadline expiry. Targeted tests passed.
This queued-deadline fix is committed separately after production7b7eef5 and is
not deployed yet. Full Studio-origin result verification remains pending.

## Expired lease on an already accepted task held runner capacity

Official MCP diagnostics116 showed runner idle;117 found no competing active Cuan
project task/launch. Diagnostic118 found a remaining running attempt assigned to
runner-01 with both lease and execution deadline expired, on a task already done.
The ordinary CompleteExecution fence correctly rejects changing a final task, but
lease cleanup reused that path and silently retained the running attempt forever.
Its code-lane count therefore prevented new claims despite the idle heartbeat.

Added an atomic narrow cleanup for expired running attempts on done tasks only.
It fences runner ID, lease renewal, cancellation and attempt status; preserves the
task status, timestamp, summary and verification; records an explicit lease-cleanup
activity. Memory and PostgreSQL stores use it before ordinary expired-lease flow.
Coordinator review ignores this bookkeeping-only interruption of accepted work;
the completed preview retains its accepted summary. No success exit is fabricated.

Behavior tests prove expired accepted lease cleanup, renewal/wrong-runner/active-
task refusal, idempotence and actual next queued code-lane claim without reopening
the accepted task. Targeted tests pass. PostgreSQL production path still needs
release/observed recovery verification; no direct database edits were made.

## Verified capacity recovery in production and real terminal input

Released24530aac7a352aeb8e0bd52f60a2d215dc7a784b successfully (health SHA matched;
log `/home/dev/reports/cuan-accepted-lease-cleanup-deploy.log`). Snapshot120 confirms
the old expired accepted-task attempt interrupted with the cleanup marker while its
task remained done. Snapshot119 confirms Studio attempt att_443069f started at
03:22:25 +08, proving the stale lease had held the runner lane and the deployed
PostgreSQL cleanup released it without a manual database mutation.

Actual authenticated production terminal opened through the task card at390/1440.
Handshake session matched att_443069f2d613c0df324134fb110fab5d. Initial probe pressed
Ctrl-L before readiness and did not transmit; it is not passing input evidence.
Second probe waited for the connected handshake, then transmitted Ctrl-L through
stdin and received additional PTY output in both viewports. Results:
`production-native-terminal-ready-result.json`; real screenshots inspected at
`production-native-terminal-{390,1440}.png` in the private E2E directory.
Only a redraw control was sent, no new work instruction or shell command.

This proves real live PTY interaction, not lossless disconnected input. Code audit
still shows onclose disables stdin, so the unsent-input buffer alone is insufficient
to prove input retention across disconnect. Native UI resize/reconnect stability
needs further verification. Terminal showed a failed report command; snapshot123
still running with progress, and124 has no delivered checklist yet. Do not mark
Studio-origin completion or the full goal achieved.

Rendered fixture now additionally verifies that an accepted result summary remains
visible despite a cleanup error in latest_attempt. Screenshots390/1440 inspected at
`/home/dev/reports/cuan-accepted-result-card/`; browser checks passed.

## Studio acceptance and reconnect verification

The same Studio-origin room 2a35dcdd-93a5-4cb2-aeff-73cad784c2b7 now
contains the delivered checklist and coordinator acceptance. Snapshot129 in
`/home/dev/reports/cuan-chat-e2e/129-studio-accepted.json` confirms task
msg_734e0ee4-1269-4bb9-ad4b-1ae5012fe753 done and its attempt succeeded.
The official CLI downloaded attachment f7c3fb43-1711-4971-ac53-f2500aca432e;
independent reading confirmed 109 Indonesian words and all five agreed points.
SHA256: d53de6c3ef070bdbd6bad79e3d88888d4daf6f27af911a855bbf6b7ee42a0c66.
Production attachment previews inspected at390/1440 show all content without
horizontal overflow. No duplicate work was created for this verification.

Unexpected disconnect now retains unsent input for the active interactive session.
Stale availability callbacks cannot change a replacement socket; confirmed missing
sessions disable further typing, and retry exhaustion explains pending input.
The browser fixture forces1012 closure, types Ctrl-L offline, reconnects to the
same attempt and asserts exactly one flush without replaying prior input at both
390/1440. Screenshots inspected in `/home/dev/reports/cuan-terminal-reconnect/`.
This is controlled reconnect evidence, not a production provider-disconnect test.
The separate production PTY evidence above proves real stdin/output.
Frontend419 tests passed; frontend build and browser harness passed.
