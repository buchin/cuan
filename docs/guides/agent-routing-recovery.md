# Recovering agent routing and inspecting waits

Cuan presents one configurable public assistant. Account metadata, usage evidence and runtime execution are separate facts: a saved sign-in or positive quota response is not proof that an inference request will finish.

## Recovery rules

- A provider verification timeout leaves readiness **unknown**, never verified or permanently unavailable. Credential presence is retained. An actual runtime attempt decides whether the route can serve work.
- When the public assistant has no ready brain, Cuan can recheck one stale unavailable, explicitly connected owner profile. Probes have a five-minute per-account cooldown, a bounded context and an exclusive cross-process refresh lease. A fresh ready route avoids needless probes. Login/setup failures, active quota cooldowns and another owner's profiles are excluded.
- Verification preserves the configured model and fallback permissions; it never accepts provider terms, reconnects accounts or overrides quotas.
- Safe capacity waits keep their durable request/message identities. Retry intervals grow from30 seconds to1,2,4 and at most5 minutes. Interrupted/uncertain work is not turned into a capacity retry.
- Claiming a queued request clears the old wait reason, so running work does not falsely retain a previous capacity error.

This adopts the useful Fural patterns of refreshed connection evidence, distinct readiness states and bounded backoff. It is not a wholesale copy of Fural's connection-binding architecture or a claim of complete failure-classification parity.

## Official diagnostics

```
cuan account list
cuan account usage ACCOUNT_ID
cuan account verify ACCOUNT_ID
cuan chat messages ROOM_ID --json
```

List and usage use the configured user's authenticated API. Verify probes an existing sign-in and updates readiness. DTOs omit credential contents and native profile paths; HTTP failure bodies are not echoed. Read-only MCP tools: `list_provider_accounts`, `get_provider_account_usage {account_id}`.

Conversation JSON now preserves `requests` (including state, safe error and recovery metadata) plus task count. Human-readable history includes active/failed request status. An observation timeout is not permission to resend or restart: inspect the same request until authoritative state changes.

## Evidence and remaining acceptance

Production investigation in room23026ea3-26a0-4c4b-99a0-201807f4fb1b found stale unavailable accounts with positive provider usage evidence. Reverification restored one account; a real request then returned three relevant requirement questions after about116 seconds. A later scope turn used read-only file tools, encountered failures, and exhausted its permitted provider route before producing a plan. No task or final artifact was created at that checkpoint.

The complete acceptance remains: Studio idea → Inbox requirement discovery → approved scope → existing/new project resolution → delegated task → independently checked artifact → report to the same conversation. These changes alone do not prove that full goal. Native E2E evidence and targeted test logs are retained under `/home/dev/reports/cuan-chat-e2e` in the working environment.
