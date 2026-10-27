## Recovering a failed reply

Your original message and attachments stay in the conversation. The failed reply
shows the action supported by its recorded outcome:

- **Try again** starts a new reply attempt for the saved message when the backend
  has evidence that repeating it is safe.
- **Check status** refreshes the conversation without starting work when the
  previous result is uncertain, interrupted, cancelled, or has tool activity.
- **Check usage** opens Settings → Agents when account/model quota is limited.
- **Check agent** opens the same page when sign-in needs attention.

For newly started replies, Cuan automatically makes at most one safe recovery
attempt after a short backoff when a verified final failure proves no unsafe
action occurred. The bubble shows **Recovering reply…** without requiring a click.
Old failures are not replayed when deploying this feature. A newer message,
cancellation, uncertain outcome, sign-in problem, or exhausted quota prevents
this automatic recovery. Manual Try again and automatic recovery share the same
child identity, so racing them cannot duplicate the attempt.

No task, deployment or uncertain tool action is replayed by these controls. Cuan currently makes a fresh provider turn with the
saved conversation context; it does not resume a native provider session or
checkpoint. This differs from alternative Resume implementations.

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
Historical failures without sufficient evidence default to Check status. Automatic
recovery additionally requires the per-turn safe-reply-v1 policy, a final no-effect
proof, a recent finished timestamp, no pending work or later turn in the room,
and no prior child for that message. Dispatcher restarts reuse a queued child
instead of creating another.

Verification includes real PostgreSQL concurrency in an isolated schema (twelve
simultaneous retries produce one child), owner/room boundaries, unknown-result
rejection, one original message/one fake provider execution, and browser fixtures
at 390/820/1440px for lost responses and quota/status controls. No live AI or
production message is sent during these checks.

## Bounded note search and reply timing

A saved-note search with no matches returns a clear clarification without calling
the model again. When matches exist, one short answer-only stage receives the
conversation and retrieved notes without the project workspace or attachments.
A tool attempt in that stage is stopped and replaced by a clear clarification.
This prevents the summary stage from repeating the original repository analysis.

Final timing is recorded atomically with the saved outcome, even when the activity
timeline has reached its normal size limit. An earlier provider-stage duration
is not shown as the total for a historical interrupted reply.

## Conversation titles

Agent conversations can update their title when the recent discussion establishes
an ongoing new topic. The ID, URL, messages and linked work remain unchanged.
Titles stay stable during brief tangents and approvals. A manually chosen title
is retained. Customer conversation names are not rewritten. This uses the normal
reply rather than an additional AI request, and does not start work or change an
approval. If a reply fails, the existing title stays in place.
