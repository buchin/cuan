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
context are excluded. Native evaluation gets120 seconds with a125-second outer
bound, respecting the original durable deadline. Candidate-only format repair
retains its40/45-second limit and cannot invent a missing action or scope.
Failed recovery remains failed, without fabricated investigation findings.

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
