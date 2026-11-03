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
