# Native release workflow: task-list limit

## Scope and first observation

A new useful code task must fix `cuan task list --limit 1 --json` returning 31
rows, preserve filters/order and shared CLI/MCP behavior, add targeted tests, then
reach a verified native release through PM/developer/independent verification.
No pagination redesign, UI changes or customer task mutations. Registered Cuan
project metadata confirms auto_deploy enabled; the preferred model's quota policy
was not changed. The prior no-publication benchmark does not prove this path.

- Room: `8f59eed3-e84e-4595-851b-f15b9c96eb57`
- Request: `4e8eca39-3403-470e-8f37-869956919ac3`
- Original owner message: `d238d12f-3552-42bc-9f98-019673646493`

The official request trace shows a 68.9-second initial provider stage and a
23.3-second format repair. Both responses failed the action contract. The saved
reply nevertheless promised delegation and appended an invalid-plan clarification
note. No task was created. The original request was not blindly resent.
Native stage metadata records ~105KB initial prompt and a candidate-only repair;
no raw rejected action values were preserved, so the exact offending production
field cannot be identified retrospectively.

## Reproduced protocol gaps and correction

The provider's actual `--json-schema` allowed arbitrary fields inside every
application action payload, but the delegation decoder rejects unknown fields
and unsupported types. Candidate-only repair received that same unconstrained
payload schema, without the native contract. Delegation now has a distinct typed
schema branch with exactly the Go payload's fields/types, supported category and
action enums, and creation/proposal/continuation required-field alternatives.
Other action kinds retain their existing behavior. Native authorization, scope,
parser and execution checks remain authoritative; no unknown fields are ignored.
A schema/parser parity test detects field/type drift and unrestricted branches.
The ordinary and repair turns reuse the same schema rather than another protocol.

A separate executable regression proves malformed delegation payloads were
salvaged as successful marked-up replies. The recovered delegation now passes
through the existing parser before being returned; invalid recovery remains a
failure with no task-start promise or action applied. Valid transport recovery
retains original project/authorization. Response diagnostics distinguish payload
fields/types from constraints using fixed labels, without recording private
candidate contents. Coordinator, formatting/recovery and diagnostics tests pass.
These tests reproduce the class of failure; they do not recover the missing raw
production field or demonstrate successful native publication yet.

## Post-repair native run

Release `f36e91b46da9a588ea759d9fe27b2105fc62b3ff` passed exact-SHA
production health with 37 migrations. The first room retained zero tasks and its
completed invalid-delegation reply. Root started a fresh run with the identical
original request, preserving that failed history rather than resetting it.

- New room: `892ac17e-b3c7-4adf-96a8-c9dc4d6bc164`
- Request: `c6d56a5f-6369-4c73-9382-2d326f29c211`
- Task: `msg_326c5110-4e5f-49a3-a19c-4b0f4491764e`
- First developer: `att_c0cde12ca73960004c80af4d2d5e4669`

The provider's first response validated without a format-repair stage (89.7s).
PM created exactly one code task bound to Cuan; its developer started at
`2026-11-09T08:27:56.649+08:00`. Original scope permits push/deployment.
A native `managed_release_claim` is recorded for the attempt, establishing the
managed source handoff path, not a claim of completed verification/publication.
Developer logs identify the ignored API limit and direct JSON forwarding; work
remains in progress. The root observer has not changed its task state/source,
steered it or manually published its feature. Final release and Done are unproven.

## Native publication observed; final acceptance pending

Developer1 producede149054c431a4c37723725c51be3c1a18ea00347; source verifier
att_be0850a88fb83c4e3898e53f6807f337 succeeded. PM's first publication review
provider timed out after243.7s, then the saved review recovered without replaying
the developer. Native publication attemptatt_515503b78ecff1e39fd6ec434b0e7db5
prepared targetdca6eee1486c0ae5265a1298e6ecd5a1b7ea20da but failed during three
concurrent Go builds with`no space left on device`. The failure remains failed.

The PM automatically admitted developer recoveryatt_0618d61f030ca23f90f6ce091c63e3ea.
It reconciled to the already-integrateddca6eee1 checkpoint, ran source checks and
handed off a clean tree without a new task feature implementation. Independent
verifieratt_fba9a1c9b0227474d6772777935fd79c then succeeded on that exact commit.
This repeated source cycle is real overhead, not hidden as a single attempt.

Native releaseatt_b2efef7c7cea6ed0d51c28edfbf20f21 prepared and published target
70aa072a29e8391b7e5824ad37bccefd981d47cb from the retaineddca6eee1 source.
Its recorded process ran01:36:50.661–01:43:18.705UTC. Native production observation
and delivery receipts bind this task/attempt/source/target, and health passed the
exact target SHA with37 migrations. Root did not merge or deploy this task's
feature manually. The coordinator automatically started final independent live
inspectionatt_dc8cdd627ee7b57f34878dd9a35c34d9, currently pending completion.
The task must remain unfinished until its full outcome is accepted natively.

Root's additional authenticated read-only production probes:

- `cuan task list --limit 1 --json`: one task.
- `cuan task list --limit 2 --json`: two tasks, same leading task/order.
- `cuan task list --status done --limit 2 --json`: two tasks, both done.
- Official MCP`list_tasks` with`status=done,limit=1`: one done task, no RPC error.

These are additional root observations, not checks attributed to the independent
native inspector. They create/change no customer tasks or messages.

## Interventions and limits

This is not an autonomous efficiency success. Root separately repaired/published
admin email suppression/templates atfeaca99e while preserving native task source;
freed an idle host Go cache (~1.3GB); pushed the sequential-build correction,
leased owner cache reuse and generated-config retention fixes; and explicitly
cleaned six idle legacy provider Go module caches (~2GB) using the tested official
Go cleaner under account leases while the native build was active. No task status
was manually accepted, no worker workspace edited, and no retry button used.

Native target70aa072a includes those earlier code corrections, but the later
legacy-module cleanup tool53d321bf was run as a verified standalone infrastructure
operation and is not claimed to be in that production binary. Sources, reports,
credentials and provider histories were retained. Full snapshot/history lifecycle,
a measured storage reserve, repeated-review overhead and a fresh low-intervention
benchmark still need work. One successful release does not complete the broad goal.
