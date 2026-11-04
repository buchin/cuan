# Retrying a saved reply

Failed and interrupted replies expose **Try again**. Queued/running replies keep
their existing execution; canceled replies are not restarted by this control.

The authenticated retry endpoint preserves the original message, attachments,
owner, agent and payload hash. A deterministic child request makes concurrent
clicks idempotent. A second failure is retried from that newer failed request.

Known safe failures retain the existing coordinator retry. If recorded activity
has an uncertain outcome, storage marks the child `retry_reconcile`. This starts
a fresh evaluation of supplied conversation outcomes and current project/task
snapshots; it does not replay the earlier provider process or an approved-plan
shortcut. The original target remains present beyond the recent20-message
window. Tool calls are disallowed by the existing direct-response runtime
contract and validated replies require `action:null`, so this evaluation cannot
create another task, reopen canceled work, apply a memory mutation, or repeat
an external effect. Reconciliation remains in effect across later quota failures.

This restores the ability to obtain a useful new answer. It does not prove an
unknown external effect succeeded, or restart an execution task merely because
the user retried its chat reply. Any missing verification remains explicit.
Automatic retries keep their existing opt-in, freshness and boundedness rules.

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
