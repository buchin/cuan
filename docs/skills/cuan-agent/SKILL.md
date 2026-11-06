---
name: cuan-agent
description: >
  Official Cuan CLI & MCP toolkit for interacting directly with Cuan platform:
  agent chat, inbox rooms (@sm, @dev, @support), task boards (list, update, create, report),
  system diagnostics, and MCP stdio tools without writing ad-hoc scripts.
  Triggers: cuan, cuan-agent, cuan chat, cuan task, cuan mcp, agent_sm, @sm, cuan inbox.
---

# cuan-agent & Cuan MCP Reference

`cuan` is the official CLI and MCP server for the Cuan platform.
Binary is installed on `PATH` at `/home/dev/.local/bin/cuan`.

## Configuration
Configuration is automatically resolved from:
- `~/.cuan/config.json`
- Environment variables: `CUAN_API_URL` and `CUAN_AUTH_TOKEN`

Check configuration anytime:
```bash
cuan config show
```

Set or update configuration:
```bash
cuan config set api_url https://cuan.dojo.cc
cuan config set auth_token <TOKEN>
```

## Agent Chat & Inbox Rooms
Interact directly with agent personas (@sm, @dev, @support) without writing custom scripts:

### 1. Create a New Conversation Room
```bash
cuan chat new --agent @sm --title "Audit CI/CD pipeline"
```
Returns room ID and web inbox URL (e.g. `https://cuan.dojo.cc/inbox/<room-id>`).

### 2. Send Message & Await Agent Reply
```bash
cuan chat send <room-id> "Verify go.mod dependencies" --wait
```
Sends message, triggers the agent worker asynchronously, and waits for the streamed reply.

### 3. List Active Conversations
```bash
cuan chat list --limit 10
```

### 4. Read Message History
```bash
cuan chat messages <room-id> --limit 20
```

## Task Board Operations

### 1. List Tasks
```bash
cuan task list --status doing --limit 10
```

### 2. Get Task Details
```bash
cuan task get <task-id>
```

### 3. Update Task Status & Evidence
```bash
cuan task update <task-id> --status review --summary "Reset styling completed" --complete-note "6/6 DoD items verified"
```

### 4. Create Task Draft
```bash
cuan task create --title "OAuth Callback" --instructions "Implement GitHub OAuth handler" --priority normal
```

## MCP Server (`cuan-inspect` / `cuan mcp`)
Configured in `~/.gemini/antigravity-cli/mcp_config.json`:
- `create_inbox_conversation`: Create a room with an agent persona
- `send_inbox_message`: Send a message and await assistant response
- `update_task_status`: Update status, summary, and verification evidence
- `list_tasks`, `get_task`, `create_task_draft`: Task board management
- `list_inbox_conversations`, `get_inbox_messages`: Conversation inspection
- `list_projects`, `get_project`: Project repositories
- `inspect_git_status`, `inspect_git_log`, `inspect_git_diff`, `inspect_git_branches`: Read-only git tools
- `inspect_system_health`, `inspect_database_schema`, `inspect_database_query`, `inspect_app_logs`: Platform diagnostics

## Project environments

Use the official CLI to manage **Project settings → Environment**:

```bash
cuan project env list PROJECT_ID
cuan project env set PROJECT_ID --from-env NAME --from-env OTHER_NAME
cuan project env set PROJECT_ID --file - < /private/path/project.env
cuan project env unset PROJECT_ID --key NAME
```

Values are redacted by default. Import secrets from named process variables or
private stdin/file input, never literal secret arguments. List with `--reveal`
only when explicitly necessary. Set merges selected keys and preserves other
project settings, with concurrency protection. Use exact project IDs.

MCP equivalents: `get_project_environment {project_id, reveal?}` and
`update_project_environment {project_id, from_env?, values?, unset?}`. Prefer
`from_env` for secrets; writes always return redacted values. Saved configuration
applies to new tasks; existing tasks retain their environment snapshot.
See [project environment guide](../../guides/project-environment-cli.md).

## Account readiness diagnostics

`cuan integration github show` and MCP `get_github_connection` read the owner's
saved Cuan GitHub connection without credentials. Check this before asking the
owner to sign in: a worker's standalone `gh` session is a different identity.
Connected status does not prove access or push permission for a specific repo;
use the registered project connection for actual repository operations.
In a bound worker session these reads use the task capability; `task get` and
`get_current_task` also include `github_connection`. Never search Cuan host
configuration or databases to recover Owner tokens or provider credentials.

`cuan task steer TASK_ID "guidance"` is an Owner-only intervention on an existing
attempt. It does not resume/restart work, replace scope, or accept the result.
Use only for requested or necessary runtime intervention, preserve the approved
scope, and inspect the same task after an uncertain response before retrying.

Use `cuan account list` to read owned AI account status, routing permissions and cooldown metadata; `cuan account usage ACCOUNT_ID` reads provider capacity evidence. These commands return JSON without credential contents or native profile paths. Capacity evidence does not prove successful runtime execution.

`cuan account verify ACCOUNT_ID` probes an existing sign-in and updates readiness, without accepting provider terms or changing the selected model/fallback permissions. Prefer this over asking users to sign in again after a transient provider outage. Verification does not override provider quota limits.

Read-only MCP equivalents: `list_provider_accounts` and `get_provider_account_usage {account_id}`.

`cuan chat messages ROOM_ID --json` preserves request state/recovery metadata and task count, not only message text. Inspect the same request after a wait timeout; do not blindly resend. See [routing recovery](../../guides/agent-routing-recovery.md).

`cuan chat retry ROOM_ID REQUEST_ID` explicitly retries the saved original
message through the Owner API. It returns the deduplicated child request ID and
state; inspect that request via `chat messages --json` rather than sending the
message again. Uncertain prior activity starts fresh reconciliation, not a replay
of previous effects. Active/canceled turns remain protected. See
[retry reconciliation](../../guides/inbox-retry-reconciliation.md).

### Cancel active task execution

For explicitly approved cleanup of obsolete test data, use
`cuan task delete TASK_ID`. This permanently deletes an inactive task through
the authenticated Owner API. Inspect its scope, linked conversation and attempt
first; active executions are rejected. This does not delete its conversation.
Never infer that a conversation is disposable from its title alone.

To retire approved work that is obsolete but worth retaining, use
`cuan task update TASK_ID --status cancelled --summary "Reason and replacement"`.
Both `canceled` and `cancelled` are accepted; storage uses `cancelled`. Active
queued/running attempts must be canceled first with `cuan task cancel`, then
read back before retiring the task. Historical attempt results are preserved.
Cancellation is not acceptance and must never be reported as completed work.

`cuan task cancel TASK_ID` requests cancellation through the authenticated Cuan
API. This does not prove the worker has stopped: inspect the same task with
`cuan task get TASK_ID --json` until its attempt reaches a terminal state.
Failures do not retry automatically or echo response bodies. Saved task scope
and conversation remain available for a scoped resume.

### Download delivered files

`cuan chat download ATTACHMENT_ID --output PATH` downloads an owner-authorized
Inbox attachment to a new private local file. Obtain IDs from conversation
messages; invalid IDs, failed responses, partial/oversized downloads and existing
output paths are rejected. Read the resulting file to verify the actual artifact.

### Inspect one execution attempt

`cuan task execution TASK_ID` reads the latest attempt's saved execution intent
and publication scope through the Owner API. It identifies ordinary execution,
verification, or exact current-outcome inspection and its source/target revision
without returning launch prose, environment or credentials. It selects the launch
bound to that attempt, even if another continuation is pending. `intent_parsed`
describes saved intent parsing only; it is not source validation, successful
execution, deployment or acceptance. The command never starts or retries work.
Legacy attempts without a saved launch are reported as legacy, not reconstructed
from report prose. Bound workers continue to use their scoped task reads.

`cuan task logs TASK_ID ATTEMPT_ID --after-id 0 --limit 100 --json` reads bounded
saved output from an explicit attempt (limit 1–1000). Use the highest returned
record ID as the next cursor. Logs are diagnostic evidence, not proof of successful
completion, and may contain private task content; save privately and inspect only
necessary excerpts. This is saved output, not an interactive terminal.

## Task worker identity

When `CUAN_TASK_CAPABILITY_FILE` or `CUAN_TASK_CAPABILITY_JSON` is present,
this is a bound worker session, not an Owner CLI session. Deliver progress/results
with `"$CUAN_TASK_EXECUTABLE" task report`; use its scoped `task get` and `task push`
when appropriate. The Inbox coordinator publishes the verified assistant reply.
Do not use `chat send`/`send_inbox_message` as a reporting shortcut: they represent
owner instructions. Bound sessions reject Owner mutations, including database
fallbacks and cached owner API clients, while read-only diagnostic tools remain
available. Their MCP tool list omits Owner mutations. Owner CLI operations outside
a bound session continue through their authenticated API identity. Sending an
Inbox message never falls back to inserting an artificial Owner into the database.
This is a CLI/MCP misuse guard, not isolation from arbitrary code on the host.

Bound worker MCP sessions expose `get_current_task`, `report_current_task`, and
`push_current_task`. These reuse the private task capability and CLI transport;
no task/attempt ID, endpoint or credential is accepted as a tool argument.
`report_current_task` accepts progress, state, summary, complete_note,
result_kind, metadata, dod_checklist with original criteria/evidence, and
workspace-relative artifacts. It sends a worker report for normal verification
and coordinator review; a successful tool response is not task acceptance.
Final results need an explicit `state`: `review`, `blocked` or `done`. A metadata
or progress report leaves the session running. Completion evidence without a
state is rejected; use explicit `progress` when saving intermediate evidence.
Successful reports return a compact identity/state receipt. They do not repeat
the original task or evidence; use `get_current_task` or `task get` to read those.
Use the same task read after an uncertain response before retrying. These tools
are not advertised to ordinary Owner MCP sessions and never fall back to Owner
credentials. The existing CLI remains available for the same reporting contract.

## Resend administration

`cuan integration resend show`, `set --key-from-env NAME --from-email EMAIL --from-name NAME`,
and `validate` use the authenticated Settings API. Secrets are imported only by
explicit environment variable name and omitted from output. Existing automatic
email preferences are preserved; validation sends no email. See
[Resend guide](../../guides/resend-integration.md).

Bound workers may create verification output under `CUAN_TASK_ARTIFACTS_DIR`
without changing the repository and attach it with `task report --artifact
session:mobile.png`. The same `session:PATH` form works in `report_current_task`
artifacts. The root belongs to the exact authenticated attempt and cannot be
chosen by the caller. Ordinary paths stay workspace-relative; all sources share
the existing file/count/size limits. This is file delivery, not task acceptance.

## Personal writing preferences

`cuan taste show` reads the authenticated member's profile, and `cuan taste config`
reads its automatic approval setting and confidence threshold. Save an explicitly
authorized preference with `cuan taste add --category brand --topic "Writing voice"
--content "Use concise conversational language"`. These preferences are personal;
never copy one member's profile into another member's account. Bound task sessions
cannot mutate owner preferences.

Chat workers select Taste using the durable request's member identity. Automatic
learning processes explicit durable instructions from that member's own operator
message, excludes quoted/code content, and follows the existing auto-approval
threshold and quota. Confidence is a phrase-based heuristic, not a measurement of
how well the agent knows someone. Ordinary chat, visitor speech, and AI drafts do
not automatically become preferences. Manual reflection reviews attributed member
messages from up to 20 recent rooms. Inferred style from repeated behavior and draft
edits is not implemented yet.

Customer reply drafts offer Regenerate beside Review & Use. Regenerate requests a
new private draft using personal Taste and conversation evidence. It does not send
the draft or authorize task execution. Review & Use still loads text into the
public composer for editing and a separate send action.

Regeneration uses a persisted `regenerate_reply_id` action referencing an existing
agent draft in the same conversation. The owner UI suppresses its audit-message
bubble and shows a spinning, disabled Regenerate control while its durable request
is queued/running/cancelling. Visitor history and receipts exclude the action.
Draft generation uses a tool-free response path and rejects responses that do not
contain a customer reply draft; it cannot authorize a task. Lost-response retries
reuse the saved action and request ID. Loading survives page reloads and stops on
completion, failure, interruption or cancellation. The exact generated command from
the first UI version remains readable as the same private action for compatibility.

`cuan task verify-publication TASK_ID` asks the native server to independently
observe the saved project's existing production release for the current running
attempt. It derives runner/attempt/revision from the saved task, accepts no
caller-supplied target or receipt, and requests no deployment or task acceptance.
This Owner diagnostic supports already-running older workers; new native runners
request the same check automatically when publication proof is absent. A failed
or uncertain response must be inspected on that same task before retrying.

### Complete response stage history

`cuan chat trace ROOM_ID REQUEST_ID --after-seq 0 --limit 100` reads the owner-only
stage journal without rerunning the request. Continue using `next_seq` while
`has_more` is true (maximum page size 200). `chat messages --json` remains a bounded
live snapshot: `step_count` is the latest sequence and `steps_truncated` signals
that complete history belongs in the journal. Late tool failures and recovery
safety evidence remain in the live snapshot even after the first100 stages.

The journal records newly observed sanitized stage events, not arbitrary raw
terminal content. Input/output excerpts retain their existing privacy and length
limits. Previously discarded stages cannot be recovered: old histories that hit
the storage cap carry `trace_history_incomplete`, which prevents unsafe automatic
replay. It does not claim a historical execution failed or rerun its actions.

### Bounded attempt log tail

To investigate the end of a particular execution without scanning its entire
terminal history:

```bash
cuan task logs <task-id> <attempt-id> --tail --limit 100 --json
```

Tail output is chronological. Use either `--tail` or a nonzero `--after-id`,
with a limit from 1 to 1000. Keep explicit attempt IDs when inspecting failures;
a new launch must not silently replace historical evidence.

### Protected UI verification without Owner access

For Cuan UI work, the registered `scripts/verify-ui.sh PRIVATE_OUTPUT_DIR`
renders actual application components using isolated API and terminal transport
fixtures. It covers Inbox detail/back navigation, preserved drafts and scroll,
and TaskSessionPage loading → running → completed transitions at390px and1440px,
as well as Markdown/Mermaid. It fails on browser runtime errors and missing
capture files. Inspect the resulting screenshots with native image tools.
The fixture terminal output is test data, not evidence of a real worker's PTY.
These checks establish component behavior only; production health, publication
and the original task outcome require separate independent evidence. Do not
capture the public landing page as a substitute for the protected screen or
borrow Owner credentials. Report uncovered workflows explicitly.

Native code-task artifact paths refer to the worker's isolated task checkout,
not the shared project clone used before claim. The server derives this checkout
from its configuration and the authenticated task; callers cannot select another
root. Explicit local sessions and verification snapshots keep their own roots.
Session artifacts retain the `session:PATH` form. Missing worker files never fall
back to similarly named files in the project clone. A reporting error names the
requested missing file; preserve the active report capability when correcting it.

Native task push uses that same server-derived execution checkout and its saved
task branch. The repository origin must match the task's saved project binding;
branch/repository mismatches preserve local work and reject publication. Cuan
uses the owner's saved GitHub connection for push and optional PR metadata, never
the runner host's `GH_TOKEN`, `GITHUB_TOKEN` or standalone `gh` login. The runner
requires an exact branch/commit push receipt; a successful push still does not
establish deployment or task acceptance. Missing/uncertain PR lookup does not
authorize creating another PR. No production branch is force-pushed or merged by
this task publication channel.
