# Retrying a saved reply

Failed and interrupted replies expose **Try again**. Queued/running replies keep
their existing execution; canceled replies are not restarted by this control.

The authenticated retry endpoint preserves the original message, attachments,
owner, agent and payload hash. A deterministic child request makes concurrent
clicks idempotent. A second failure is retried from that newer failed request.

Known safe failures retain the existing coordinator retry. If recorded activity
has an uncertain outcome, storage marks the child `retry_reconcile`. This starts
a fresh evaluation of the supplied conversation, current task state and recorded
outcomes. It never blindly replays a provider process or approved-plan shortcut.
The original target and attachments remain available beyond the recent20-message
window. Reconciliation stays in effect across later quota failures.

Native chat tools remain disabled. A conversational answer has `action:null`.
For unfinished work explicitly authorized in the original owner message, the
response may request a delegation with `reconcile:true`, an existing project and
the original authorization/task identity. Other mutations, unmarked delegation
and borrowing approval from another scope are rejected. No new project can be
created by this mode.

The application checks current task state before mutation. Completed/canceled
work stays unchanged; active/queued work is monitored. An existing task is reused,
and a new one uses the original message's canonical idempotency key. Both task
instructions and launch revision require inspection of actual current state
before any remaining authorized effect. Uncertain effects cannot be repeated;
ambiguous identity, payment, resource or evidence requires a concrete decision.
Original approval, attachments, acceptance criteria and attempt limits remain
binding. The retry does not itself mark payment or grant access.

The fresh response receives conversation history, registered project IDs and only
this room's task snapshots; unrelated project backlog and code/host inspection
context and the unrelated Cuan contact directory are excluded. Receipts stay in durable
storage for the task; tool-free reconciliation does not mount them or advertise file
inspection. Native tool-free turns run from an empty, trusted reply workspace,
not provider HOME. Direct recovery and customer-draft delivery retain the
120/125-second bound. Tool-free result evaluation has a separate 240-second native
budget and five seconds for process cleanup, respecting the original durable
deadline and cancellation. `INBOX_REVIEW_TIMEOUT` can shorten that budget (positive
duration, maximum `240s`); direct recovery settings do not change it. This avoids
repeatedly discarding unfinished reviews at the short chat-recovery limit.
Finish/format-response is permitted while inspection and execution tools remain
forbidden. A failover starts only when the request has enough time for another
full review stage; otherwise the same saved review waits. No task execution,
acceptance, model preference or routing permission changes. Candidate-only format repair
retains its40/45-second limit and cannot invent a missing action or scope.
Failed recovery remains failed, without fabricated investigation findings or a
second full evaluation after format repair. The prompt documents the complete
delegation payload while native transport uses the ordinary shallow envelope.
Contract diagnostics retain fixed rejection reasons only, never payload values.

Result reviews classified as `remaining_work=verification` first try the native
inspection of the registered project's existing live revision. If that inspection
is not eligible and an ordinary continuation is authorized, its saved revision
always adds read-only repository, no-push and no-deploy limits. The model's original
feedback remains labeled as untrusted evidence to evaluate within those limits;
it cannot grant configuration changes or repeat external operations. If inspection
finds a required change, the worker reports that gap for another coordinator
assessment. Implementation corrections retain their existing approved scope.

Automatic retries retain their existing opt-in, freshness and boundedness rules.
This is an orchestration recovery path, not proof of an external result. The task
must still complete its original acceptance checks and coordinator review.

Validation:

```sh
go test ./internal/db ./internal/server -run 'TestInboxRetry|TestInboxReconciliation|TestInboxUncertainRetry|TestInboxAuto|TestInboxContext|TestInboxDeniedTool' -count=1
cd web
node --test tests/inbox-recovery-controls.test.mjs tests/inbox-tool-budget-recovery.test.mjs tests/inbox-quota-failover.test.mjs
npm run build
cd ..
node docs/evidence/inbox-retry-clarity-check.cjs
```

`TestInboxRetryPostgres` also exercises the SQL implementation using an explicit
disposable `cuan_s25` database. Browser fixtures cover390/1440px, both languages,
light/dark, successful retry acceptance and stale409 responses. These are
controlled provider fixtures, not evidence of native provider recovery.

### Operational completion

A safe stop or a completed investigation does not complete an authorized payment,
access or record change. All operational task reviews explicitly classify
`requested_outcome` as `completed`, `needs_input` or `unfinished`. Missing outcome
proof cannot pass. Agent-fixable unfinished work retains an in-scope revision
rather than being mislabeled as an external user prerequisite. An unmatched identity/payment requires the exact missing owner
information and an attention state, with no repeated operation or new approval.

A bounded policy audit re-evaluates previously accepted reconciliation operations
which lack this classification. It compares the exact task snapshot and latest
attempt atomically, respects cancellation and active launches, and queues only a
review of saved evidence. It never starts a business execution or resets attempts.
Ordinary accepted history is excluded.

Read-only executions that fail after capturing a repository baseline preserve a
bounded post-execution scope observation, even when their execution context has
ended. The runner uses a separate five-second context and runs no project tests
or publication. `repository_scope_observed` records the baseline/final hashes
under that attempt's runner identity. It is diagnostic evidence only: a matching
hash never changes the failed attempt, confirms a final agent report, establishes
project checks or grants task acceptance. Changed/unavailable scope and historical
timeouts remain visible; cancellation does not start a recovery execution.

Inbox message submission acknowledges durable acceptance before hydrating history.
With an explicit request ID, HTTP 202 returns the saved `request` and
`user_message_saved: true`; formatted messages and replies arrive through the
conversation snapshot/stream. Legacy calls without a request ID retain their
existing response. A history outage never converts accepted delivery into failure.
The browser clears confirmed pending delivery immediately. Uncertain transport
responses retry a bounded number of times with the frozen text, attachments,
action and ID; changing composer mode cannot change the pending payload. An
online event resumes the same saved request. New unsent edits remain intact.
Background result reviews use at most one concurrent slot per AI account, leaving
the existing shared account capacity available to task execution and live chat.
