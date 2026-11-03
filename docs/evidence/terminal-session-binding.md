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
