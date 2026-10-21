# Inbox coordination

Cuan handles work through the conversation. Users describe an outcome, clarify
important ambiguities and approve proposed scope when needed. They do not need
to select an agent or manage task statuses. Explicit implementation instructions
already authorize their stated scope; Cuan does not request duplicate approval.

## Responsibilities

The Inbox agent coordinates the goal, maintains context, delegates execution,
checks results and reports progress. Task agents implement approved work in its
bound project workspace. These internal roles follow Agentic Scrum without
requiring users to learn its terminology or participate in technical ceremonies.

The runtime embeds `internal/workflow/coordinator.md` and `developer.md` as the
respective role contracts. Inbox native tools are for analysis: AGY uses plan
mode with terminal sandbox restrictions; Claude uses read-only file tools and
Codex uses a read-only sandbox. Execution permissions belong to task sessions.
Provider profile storage is runtime state, never a deliverable workspace.

## Result review

A task report triggers review against the original request and acceptance
criteria. The coordinator distinguishes reported claims from observed checks.
Verification must include usability and documentation for product changes, plus
any approved deployment or migration gates. Process exit alone is insufficient.

If a result falls short, the coordinator sends concrete correction instructions
to the same task: the gap, affected area, verification and finish criteria.
Interruption recovery must preserve approval, context and existing work, avoid
overlapping executions and respect user cancellation. The coordinator must not
implement replacement files itself or announce success for a failed task.

Users receive useful progress and accessible results in Inbox. Task/session links
are optional inspection paths. External blockers are explained with their impact
and options; new scope requires approval.

## Interrupted execution

A failed managed task with exit 137 can continue automatically in its existing
workspace using its original approval. This applies only while the task is blocked,
was not cancelled and has fewer than three attempts. The durable launch path
rechecks ownership, approved scope and active execution before starting. A
concrete revision requires inspection of existing files, reconciliation of
uncertain external effects and fresh acceptance evidence. Other failures go to
coordinator review. Exhausted retries do not loop indefinitely.

## Verification status

The live interruption scenario exposed coordinator implementation outside the
task workspace. Plan mode and the strengthened role contract address that
boundary, but automatic recovery and final delivery still require native end-to-end
verification. This document does not certify those flows as complete.
