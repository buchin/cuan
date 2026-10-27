# Auto-Retry Default

Cuan automatically retries a task when the selected agent reaches its usage limit (`quota_limited`), so non-developer users don't see a failed task just because one provider quota is exhausted.

## Behavior

1. **Task accepted.** When a user submits a goal (via Studio or Inbox), Cuan selects the assigned agent and preflights its provider account/model route.
2. **Quota hit.** If the primary account/model is `quota_limited`, Cuan automatically tries another verified & available agent account on the same task — without asking the user.
3. **Max 3 retries.** Cuan attempts up to 3 times within a single view. If every available account/model is quota-limited, execution stops and an informative error is shown.
4. **Idempotent.** Each request carries a UUID/hash. Retries and page refreshes never duplicate the user message or re-run the same CLI execution. A changed request returns `409`.

## Error message

When all retries are exhausted:

> All available agents reached usage limits. Your message is saved. Try again after the limit resets.

## Files

- `internal/server/terminal_handlers.go` — `runTaskAgent` wraps PTY launch in a retry loop (`maxAgentRetries = 3`). `onQuota` detects usage-limit events during execution and triggers the next route.
- `internal/server/inbox_failover.go` — `providerRoutes()` preflights available account/model routes, ranking by remaining quota and skipping `quota_limited` accounts.
- `internal/server/task_output.go` — `taskOutputRecorder.onQuota` scans PTY output for quota signals and calls `StopQuotaAttempt`.
- `internal/terminal/manager.go` — `StopQuotaAttempt()` closes only the attempt whose output reported a usage limit.

## Testing

Targeted tests in `internal/server/task_autoretry_test.go` verify:

- No routes available → `409 provider_unavailable` with no CLI invocation.
- Same request ID retried twice → `409` both times, no duplicate executions.
- Error message mentions quota limits.

## Related

- [S27 Sprint Plan](../../sprints/S27.md) — auto-retry is T1 of Sprint 27.
- [S26: Multiaccount agents](s26-accounts.md) — one agent, one connection, isolated profiles.
