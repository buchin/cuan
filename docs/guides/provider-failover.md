# Automatic AI account and model selection

Cuan keeps the same agent, conversation and task identity while selecting an allowed AI account/model behind the scenes. This release supports Antigravity accounts and the Gemini/Claude text models reported by its provider. Claude Code and Codex keep their existing launch behavior.

## Set up once

Open **Settings → Agents**. For each connected private account:

- Enable **Use this AI account as a backup** to let your other agents use it for Inbox replies and new task sessions.
- Enable **Allow available text models on this account** to permit Cuan to choose another available Gemini/Claude text model. Different models have different capabilities and usage limits.
- Use **Check model usage** to inspect the provider's reported remaining percentage and reset time for each model.

Both permissions are off for existing accounts until enabled. The primary agent/account binding is never changed. Another owner's accounts, disconnected profiles, disabled agents and legacy service profiles are excluded from the backup pool. Disabling backup participation remains possible when sign-in expires.

## Automatic behavior

A new Inbox turn or task launch performs the same provider preflight automatically. The usage button is optional; it is not required before sending a message or starting a task. Usage is cached for 30 seconds, and each account's HTTP request has a three-second timeout. Usage lookups and the complete preflight have an eight-second budget. At most the primary and seven eligible backup accounts are inspected. This is an availability snapshot, not a reservation or guarantee of future quota.

Cuan ranks allowed account/model pairs by reported remaining percentage, preferring the primary account on ties. A Gemini limit does not block a different model with reported available quota on the same account when model fallback is enabled. Unknown quota is never treated as 100%. If usage is unavailable, Cuan can still attempt the configured/default model on an account that is not known to be limited; it does not invent a different model or quota percentage. Legacy profiles retain their existing default behavior but are not shared as backups.

Each connected account profile supports independent concurrent sessions. By default up to **4** Inbox/task sessions can share that profile; they still share provider quota. A long task does not reserve the account exclusively. Set `CUAN_ACCOUNT_MAX_SESSIONS` on the service to 1–16 if needed (invalid/unset values use 4), and apply the same setting to all server processes. Each running session holds one OS-backed slot until it ends. Slots release on process death. An account is skipped only when all of its configured slots are occupied. Provider or account/model changes are visible in Inbox **Detail** and task activity. Account labels, credentials and raw provider diagnostics are not included in these activity messages.

## Recovery and limits

Inbox can try at most three distinct allowed account/model pairs within its existing request deadline. Only validated success produces a reply or triggers approved application-managed task delegation. Failed partial responses never create duplicate messages or tasks. A repeated request ID still refers to the same saved request.

Automatic retry is limited to usage-limit failures and activity that can safely be repeated: no tools, or explicitly recognized read-only tools. Terminal commands, unknown tools, permission-denied activity, malformed streams, login errors and interrupted/time-out requests do not authorize a replay. The same durable Cuan context, workspace and attachments are passed to a fresh CLI turn; Cuan does not resume another account's native conversation or transfer its session history.

A runtime limit sets a five-minute cooldown on the selected model, or on the account when the failing model is unknown. This is a retry delay, not an assertion about the provider's reset time. Reported zero-quota models remain ineligible while the provider continues to report zero. If every allowed route is exhausted, busy or unavailable, the message stays saved, or a task launch returns an actionable error before starting. No background queue or automatic replay of previously failed messages is introduced.

**Running task processes are never automatically restarted on another account.** They may already have changed files or run commands. Their saved output, task state and existing recovery controls remain available; automatic preflight applies when starting a new session. Continuing an active task across accounts would require a separate validated checkpoint/resume mechanism.

## Operations

The usage lookup reads provider OAuth access tokens from the selected private profile and queries the same Cloud Code model-quota RPC. It does not run a model, copy credentials between profiles or expose provider responses. A still-valid access token is reused. Near-expiry tokens (within one minute) are refreshed through Google OAuth; a 401 can trigger one bounded recovery attempt. Backend refresh takes an exclusive profile gate; session leases share that gate and claim separate slots. Refresh therefore waits while any session is active, and new sessions do not overlap a backend credential rewrite. Native CLI credential/state handling still follows the provider CLI’s own concurrency behavior. Refresh attempts are limited to one per account per minute, including rejected grants. Busy accounts defer refresh. Expired/unsupported credentials that cannot be refreshed produce unavailable usage rather than an inferred quota. Refreshed credentials are saved atomically with mode0600, retaining the original token container, account identity and existing refresh token when no replacement is returned. Other credential fields are preserved. Redirects are disabled and network responses are bounded.

Preferences and cooldowns are additive fields in private account metadata under the existing accounts directory; no database migration is required. Existing account files remain compatible. Backup that directory with existing credentials and keep it private. Native atomic deployment, idle gate, health check and rollback apply. Rolling back the application preserves account credentials and primary bindings; older releases ignore the additional fields. In-process usage cache is disposable; persistent cooldowns and account leases survive or release appropriately across process restart.

On the current production owner, both connected private accounts have backup and alternative-model permissions enabled as requested. New installations keep those permissions off until their owner enables them.

## Capacity versus quota

A full session-slot limit is local concurrency, not exhausted provider usage.
Inbox now reports that distinction and records that no provider was started,
allowing the existing bounded safe recovery path. Actual provider quota responses
still use account/model cooldowns and allowed fallback routes. Parallel sessions
do not multiply an account’s allowance or bypass provider rate limits.
