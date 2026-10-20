# Quota-aware account/model routing

Implemented automatic preflight for new Inbox turns and task launches, explicit owner permissions for backup accounts and alternate text models, read-only model quota lookup, bounded Inbox failover, durable cooldowns and cross-process account leases. Same agent/message/task identity and primary bindings are retained. Active task processes are never automatically replayed.

Targeted Go tests cover:

- Same-agent backup reply; known-primary cooldown skip; same-account Sonnet when Gemini is zero; highest reported quota selected before execution; alternate-model permission and model cooldown checks.
- Opt-in off by default; owner/auth validation; preference enable/disable; stale/disabled profiles; login error and context cancellation; terminal/unknown tool activity stopping retry; read-only tool retry; exhausted backups and bounded attempts.
- Durable request execution produces one user message and one agent reply across failover, and replaying its request ID does not launch another CLI.
- New task PTY runs in the selected backup home with the selected --model argument, retains agent identity, records route activity and preserves launch idempotency.
- Model quota HTTP request/parser/cache, unsupported model filtering, unchanged credentials for ordinary usage checks, atomic refresh and rejected/busy refresh safety, owner separation and exclusive account leases.
- Existing runtime validation/redaction, account ownership API, task workspace revision/replay and task argv/request lifetime checks.

Frontend build and isolated browser tests verify backup preference defaults/persistence/revocation/failed-save state, alternate-model permission, per-model 0%/80% display and mobile/desktop layout. Fake HTTP/CLI fixtures never invoke real models or spend customer quota. Production smoke checks settings/API/assets/health without user messages, task launches or model invocations. As requested, backup and alternate-model permissions were enabled on the two existing owner accounts. Their normal expired verification state was normalized when saved. Credentials and bindings were preserved during policy changes. A subsequent production diagnosis found one expired OAuth access token; bounded safe refresh was added and separately verified.

Known legacy test `TestS264TimeoutAfterAcceptanceSafeRetry` uses raw CLI stdout and old Indonesian error labels rather than the current NDJSON runtime contract; it fails independently of routing and was not changed in this task. The new `TestInboxFailoverSingleDurableReply` exercises current request deduplication through the HTTP endpoint with a valid runtime fixture.

User guide: ../../guides/provider-failover.md.

Production release 0618dbd: both connected Google AI Pro accounts returned actual per-model quota snapshots (13 and 2 eligible text-model entries). No model inference was invoked. Backup/model permissions are enabled on both owner accounts. All 35 retained application tables matched backup. Across 732 existing profile files, only the two expected OAuth access-token updates occurred; refresh tokens, credential identity/extra fields, primary bindings and other profile files were preserved. Readiness healthy on schema23. Token recovery was atomic and limited to the normal expiry/401 path.
