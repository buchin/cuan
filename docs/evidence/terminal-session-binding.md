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
