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
# Account model discovery

`GET /api/v1/provider-accounts/{accountID}/models` returns the installed
provider CLI's model IDs for the signed-in owner's private account profile.
It requires saved account credentials and excludes the legacy server profile.
At present this endpoint supports Antigravity accounts; unsupported providers
return an actionable error instead of a fallback catalog.

Discovery does not run inference or establish remaining quota. A quota-limited
account can still return its catalog. Catalog reads use the existing bounded
profile lease and cache, so they do not interrupt active work. A failed read
must leave a previously saved model selection intact in the consuming UI.

## Newly connected brains

The canonical Cuan assistant can use a signed-in private Antigravity account
without a legacy persona/connection binding. This matches account onboarding in
Settings: connecting a brain does not create another public agent. Historical
bound accounts still obey binding and enablement checks; partially bound records
fail closed. Routing revalidates owner, provider, readiness, cooldown and exact
profile home under the execution lease. Model/fallback preferences are preserved.

### Pending launch after a capacity failure

A route can reserve an execution before a deferred launch response is saved. If
that execution later fails while the launch still says queued/claimed, the stale
launch used to suppress review and recovery indefinitely. Reconciliation now
closes only the launch bound to that terminal execution. It preserves the task,
all attempt results, approvals and recovery budgets.

A separate newer pending continuation can also be stranded when the earlier
attempt changes the board state to blocked. The existing queued intent is
restored only for the exact known quota failure (exit 75), unchanged project and
scope, a current unexpired launch and no active execution, cancellation or later
explicit block. Ordinary failures, manual blocks, expired/cancelled approvals and
accepted work stay untouched. Execution still rechecks the saved owner approval
and cancellation through the normal launch path. No failed attempt is replayed.

### Started launch with a queued execution

A launch marked `started` means its attempt was created, not that the runner has
begun executing it. A launch bound to its own still-queued attempt therefore does
not hold the project execution lock. Treating it as working made several queued
launches block one another while the runner stayed idle.

Claiming serializes lane checks for each runner. For a project-bound candidate,
the database takes a short project claim lock and rechecks eligibility in a fresh
snapshot before transitioning it to running. Concurrent runners cannot use the
queue correction to claim two conflicting project tasks. Actual running attempts,
live claimed preparation and started launches with unknown attempt ownership
retain their existing protections. Existing attempts advance through normal
claiming; no retry, task rewrite or manual lock deletion is required.

### Inspection stopped before a worker existed

A saved live-result inspection can fail preparation before any execution attempt
is reserved. Cuan permits one separately identified preparation repair for the
known snapshot-publication failure, or a scope-validation rejection whose saved
intent now passes validation. The original launch stays stopped as history.
Malformed intent, changed instructions/owner/source/repository, cancellation,
expired admission, any existing attempt, and repair chains remain ineligible.
The replacement retains the original expiry and authorization, observes the
current release, and passes normal source and lane checks. It only inspects an
exact snapshot; it cannot implement, push, deploy or accept the task. This shares
the existing preparation allowance rather than resetting task retry budgets.
The same preparation repair can finish an already saved, still-valid admission
even if a later model opinion asks the owner to deploy. That opinion is not new
owner scope: original approval, native source validity, inspection prohibitions
and cancellation are rechecked. This permits only the originally admitted
read-only inspection; it neither overrides an owner restriction nor establishes
deployment, successful verification or task acceptance.

### Supplemental verification and the current release

When a supplemental verifier finishes, it remains a read-only attempt. A fresh
review may still find that the requested outcome needs inspection on the running
release. Cuan resolves the original implementation through the verifier's native
source and unchanged-repository receipts, then uses the existing single
current-outcome inspection allowance for that source. It retains the current
review and original owner approval; the verifier never becomes an implementation
or deployment source. Missing receipts, changed source, cancellation and owner
binding mismatches cannot authorize this handoff. The new inspection must still
prove the actual feature and publication requirements independently.

### Completing a specific evidence gap

A current-outcome inspection that passed native source checks can still be
rejected because its screenshots or checks do not cover an original criterion.
A saved coordinator `continue`/`replan` verification decision may use the existing
single revalidation allowance with `revalidation_kind=evidence`. This shares the
same deterministic request ID as changed-source revalidation; deployment does not
renew it and the result cannot start another revalidation chain. It can inspect
the same or the newly observed running revision without pretending source paths
changed. The native previous inspection receipt, passing checks, unchanged-source
baseline and original owner/source/review bindings remain required. No code,
configuration, push, deploy or task acceptance is authorized. The worker receives
the concrete missing evidence, and must report actual targeted captures; unrelated
generic application screenshots cannot establish the requested feature.

Older runners without `supports_evidence_revalidation` cannot reserve this mode.
Typed inspections/verifiers never fall back to ordinary execution of a historical
implementation checkout. Separately, a completed current manager review with no
saved launch can finish its original read-only admission using the saved exact
decision. Cuan does not resend the owner/model message, retry an existing stopped
launch, accept stale evidence or reset attempt history. The mandatory native
verification envelope is compared to its saved raw feedback exactly; adding the
server's read-only limits does not invalidate that original manager decision.

Native inspection admission keeps implementation provenance separate from the
immediate execution predecessor. After a successful supplemental verifier,
`source_verifier_attempt_id` binds that verifier while `source_attempt_id` still
identifies the original implementation. The server revalidates its original
checks, unchanged repository receipt, source receipt, and owner/request binding.
A successful predecessor permits only this validated inspection through its
exact claimed launch. Both storage adapters atomically recheck the latest
uncanceled predecessor, unchanged launch revision, lease, project, and instruction
hash. Ordinary retries after success remain rejected. This does not grant another
implementation, publication, acceptance, or recovery allowance.

Slow or interrupted coordinator delivery scans emit one infrastructure diagnostic
with a fixed stage code, current task ID, candidate/examined counts, elapsed time,
and interruption flag. A completed scan also emits this summary at most once per minute, including
candidate discovery failures, so fast early exits are observable too. This distinguishes
slow acceptance audits, candidate discovery, task reads, recovery, and coordinator
review without logging conversation content or credentials. A ready inspection
plan alone does not prove that the delivery scan reached or admitted that task.
The same periodic diagnostic scan records fixed review checkpoints and inspection
eligibility/plan/admission codes per task. These observations neither retry work
nor change authorization; they distinguish reaching a task from admitting it.
`cuan task execution` also identifies an existing admission for the exact ordinary
inspection plan, separately from the latest task launch. Its state and fixed stop
code reveal an older stopped admission without exposing stored feedback or errors.
A plan can be ready while its existing admission remains terminal; readiness does
not grant a retry or a replacement authorization window.

An unstarted legacy inspection stopped by the missing supplemental predecessor
can use the existing one-time preparation repair. It requires the same original
implementation and authorization, a successful native-bound supplemental
verifier, no reserved attempt, and the original unexpired deadline. The saved
stopped launch stays unchanged. Other stale inspections gain no retry.

After an inspection succeeds, evidence revalidation identifies that completed
attempt as its parent. Preparation and verifier-repair markers belong to the
parent's history and are not copied into the new stage or its request key. The
parent's native provenance is still validated recursively. This keeps one
revalidation allowance across deployment revisions and prior repair paths.

New revalidation stages explicitly select their source and approval fields rather
than copying all flags from an earlier stage. Chat instructions likewise separate
proposed, implemented, tested, and deployed capability: dependencies, approval,
and worker reports alone never establish compatibility or data safety. Missing
prerequisites remain work to verify, not guarantees to present to the owner.

An explicit model fallback preserves the saved preferred model. Configure it with
`cuan account fallback ACCOUNT_ID MODEL_ID` using an ID from that account's native
catalog; `off` clears and disables it. For Sonnet-first routing, keep Sonnet selected
and choose a supported Gemini entry as fallback. Neither quota display names nor
similar names establish runnable IDs or shared reset groups.

Both Inbox and native task admission prefer selected-model routes across all
allowed connected accounts before explicit fallback routes. Native execution
leases from that same pool, excludes failed account/model pairs separately, and
can continue retained work on another permitted model of the same account after
quota failure. It preserves original scope and reconciles uncertain effects;
changing models never grants replay of external writes. The actual attempted model
receives its own cooldown. Account-wide legacy limits remain binding.

The runtime model is transient lease metadata; saved primary selection and
credentials do not change. New work prefers the primary again when it becomes
available. Running sessions are not interrupted to switch back. Disabling fallback
is rechecked under the lease. Explicit fallback configuration also prevents old
public-agent fallback defaults or unrelated higher-quota models from overriding
the account's selected pair. Catalog checks establish support, not successful
inference or available quota.
