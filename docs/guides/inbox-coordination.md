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

## Verification status

The live interruption scenario exposed coordinator implementation outside the
task workspace. Plan mode and the strengthened role contract address that
boundary, but automatic recovery and final delivery still require native end-to-end
verification. This document does not certify those flows as complete.
