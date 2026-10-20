## Recovering a failed reply

Your original message and attachments stay in the conversation. The failed reply
shows the action supported by its recorded outcome:

- **Try again** starts a new reply attempt for the saved message when the backend
  has evidence that repeating it is safe.
- **Check status** refreshes the conversation without starting work when the
  previous result is uncertain, interrupted, cancelled, or has tool activity.
- **Check usage** opens Settings → Agents when account/model quota is limited.
- **Check agent** opens the same page when sign-in needs attention.

A retry is not an automatic restart. No task, deployment or uncertain tool action
is replayed by these controls. Cuan currently makes a fresh provider turn with the
saved conversation context; it does not resume a native provider session or
checkpoint. This differs from Fural's Resume implementation.

## History and duplicate prevention

Each failed attempt can create at most one child attempt. Repeated clicks, network
retries, page reloads, or multiple tabs return the same child. The original user
message, attachment snapshot and owner/agent binding are reused. Previous attempts
remain stored and appear under **Detail → Previous attempts**; the conversation
shows the latest attempt instead of stacking failure bubbles for one message.

The same queue limit and FIFO dispatcher apply. The backend enforces ownership,
conversation scope, failed-state eligibility, and outcome policy. UI state alone
never authorizes a retry. A delayed/lost retry response can be reconciled using
Check status, or by repeating the same retry endpoint safely.

## API and verification

Owner-authenticated endpoint:
`POST /api/v1/chat/conversations/{conversation}/requests/{failedRequest}/retry`.

No message text or new idempotency key is submitted. The backend derives a stable
child request ID from the failed request, and returns `202` when newly accepted
or `200` for an existing child. Unknown requests return `404`, ineligible
attempts `409`, queue capacity `429`, and unavailable storage `503`.
GET/SSE request snapshots include `recovery_action`.

Retry insertion is serialized with the existing room advisory lock in PostgreSQL;
the request and its previous outcome are retained. No migration is needed.
Historical failures without sufficient evidence default to Check status.

Verification includes real PostgreSQL concurrency in an isolated schema (twelve
simultaneous retries produce one child), owner/room boundaries, unknown-result
rejection, one original message/one fake provider execution, and browser fixtures
at 390/820/1440px for lost responses and quota/status controls. No live AI or
production message is sent during these checks.
