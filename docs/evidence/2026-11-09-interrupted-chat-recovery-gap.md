# Interrupted chat recovery: confirmed gap

The successful manual recovery recorded in `2026-11-09-artifact-workflow-latency.md`
does not establish autonomous recovery. Repository inspection confirms why:

- `InboxRecoveryAction` classifies every interrupted request as `check_status`.
- `InboxAutomaticRecoveryCandidate` accepts only `try_again`, with a final
  `recovery_safe` proof, opt-in policy and a two-minute freshness window.
- Both repository implementations recheck this policy before creating a child.
- Dispatch discovery includes active requests and recently finished failed
  requests, but excludes terminal interrupted requests.
- Manual retry already creates `retry_reconcile` for uncertain results. This
  re-evaluates current saved state rather than replaying the old response.

Simply adding interrupted requests to the candidate filter is insufficient:
repository creation currently overwrites the child's first step with
`retry_auto`, whereas `InboxRetryNeedsReconciliation` recognizes only an initial
`retry_reconcile`. Such a change would lose the reconciliation contract.

## Required implementation contract

Use a distinct, versioned opt-in for newly started owner turns. Preserve old
interrupted history and its unknown outcome. Permit at most one fresh child with
the same saved message and deterministic retry identity, sharing manual retry
idempotency. Retain the reconciliation marker through persistence and dispatch.
Do not automatically recover canceled work, incomplete traces, newer steering,
active executions, ambiguous visitor authority or legacy failures. Check the
saved application/task state before issuing any further action.

Recovery must be discoverable after process restart, with a bounded eligibility
window based on actual terminal observations. A stored deadline alone is not
proof that the provider process has stopped. Test both SQL and memory behavior,
manual/automatic races, dispatch discovery, marker preservation, and native
no-duplicate execution. Prove a real production timeout recovers without an
operator retry before claiming this reliability gap is closed.

## Verification

2026-11-09: targeted existing DB recovery, idempotency and dispatch tests passed:

```sh
GOGC=30 GOMAXPROCS=2 go test -p 1 ./internal/db \
  -run 'TestInbox(AutomaticRecovery|RetryAtomic|RecoveryPolicy|ManualRetry|Dispatch)' -count=1
```

This records the confirmed root cause and implementation gates. It does not
change runtime policy or claim the automatic reconciliation fix is deployed.

## Executor-return evidence repair

Further inspection found that `ListInboxRequests` can expire a running row before
its executor returns. Previously `FinishInboxRequest` then ignored that row,
losing evidence that processing actually returned. SQL and memory finalization
now retain one `execution_returned` observation on an already interrupted row.
The interrupted state, unknown-outcome error and original messages are preserved;
a late reply is discarded. The observation survives snapshot compaction. It does
not itself authorize automatic recovery or establish successful external effects.

Targeted DB tests passed, including expiry-before-return, duplicate finalization,
no late reply, no automatic retry from the receipt alone, and trace compaction:

```sh
GOGC=30 GOMAXPROCS=2 go test -p 1 ./internal/db \
  -run 'Test(ExpiredInbox|ExecutionReturn|Inbox)' -count=1
```

The disposable PostgreSQL tunnel on localhost:15432 was unavailable during this
check. SQL runtime parity and native production recovery remain unverified;
this change has not been deployed.
