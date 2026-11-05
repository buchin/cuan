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

Native releases run Inbox in a [separate chat worker](chat-worker.md), with its
own queue/concurrency and reserved account capacity. The API and task runner
have separate lifetimes; API deployment does not terminate chat execution.

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
boundary, and native recovery of the original task has now succeeded with the original
artifact unchanged. Final coordinator review still fails on native tool denial,
and browser-accessible deliverable delivery remains unfinished. This document
does not certify the whole lifecycle as complete.

## Delivering requested files

Execution agents include `--artifact "relative/path"` on their final `task report`
for each requested file. Up to eight files can be attached (10 MB each, 25 MB total).
Paths resolve inside the execution workspace opened when the session starts;
files outside it, hidden credentials and private keys cannot be attached.
Cuan snapshots outputs into the owner's private attachment storage and surfaces
download cards directly in the original Inbox conversation. HTML downloads as a
file; it is not executed on the application origin. Users need no server access.
Report assessment receives the actual delivered attachment metadata. A server
path or file:// link alone does not satisfy a request for downloadable files.

Example for an execution agent:

```sh
"$CUAN_TASK_EXECUTABLE" task report --state done \
  --summary "Offline checklist ready" --complete-note "Actual verification" \
  --artifact "packing-checklist.html" --artifact "README.md"
```

File delivery currently requires a task originating from an Inbox conversation.
Downloads require sign-in and remain available as immutable snapshots after the
execution exits or the source file changes. Repeating the same delivery does not
create duplicate message cards. Work reports and attachment delivery are distinct
from the coordinator's acceptance decision.

## Tool turn budget, adaptive batching, and infinite-loop guards

Chat tools resolve missing context for a reply or delegation. Deep investigation
belongs in an authorized task, whose execution budget is independent of chat.

- Chat defaults to 12 tool calls. Each inspection counts once. Operators can
  override `CUAN_CHAT_TOOL_BUDGET` (legacy `INBOX_TOOL_BUDGET`) and explicitly
  configure batching with `CUAN_CHAT_INSPECTION_BATCH_FACTOR`.
- The configured budget and five consecutive tool failures stop runaway loops.
- A safe read-only turn that reaches its budget can make one tool-free
  coordinator response, using the supplied conversation and task evidence.
  It has a 120-second native budget and 125-second outer deadline, capped by
  the original request deadline. This fresh evaluation is distinct from the
  candidate-only 40/45-second format-repair stage.
  Scope, project and original authorization still undergo normal validation.
- Failed recovery stays failed and does not publish fabricated findings or ask
  for renewed authorization. Unsafe child activity cannot enable a safe replay.
  Tool findings not persisted in the supplied context are not recovered evidence.

### Published task progress

Task cards use explicit progress reports from the latest attempt. Native terminal
stdout, keyboard shortcuts, tool summaries and reasoning fragments stay in the
Terminal; they are never guessed to be user-facing progress. Without an explicit
report, the card retains its live work status rather than inventing an update.
