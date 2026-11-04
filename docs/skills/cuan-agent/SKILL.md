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

Use `cuan account list` to read owned AI account status, routing permissions and cooldown metadata; `cuan account usage ACCOUNT_ID` reads provider capacity evidence. These commands return JSON without credential contents or native profile paths. Capacity evidence does not prove successful runtime execution.

`cuan account verify ACCOUNT_ID` probes an existing sign-in and updates readiness, without accepting provider terms or changing the selected model/fallback permissions. Prefer this over asking users to sign in again after a transient provider outage. Verification does not override provider quota limits.

Read-only MCP equivalents: `list_provider_accounts` and `get_provider_account_usage {account_id}`.

`cuan chat messages ROOM_ID --json` preserves request state/recovery metadata and task count, not only message text. Inspect the same request after a wait timeout; do not blindly resend. See [routing recovery](../../guides/agent-routing-recovery.md).

### Cancel active task execution

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
Use the same task read after an uncertain response before retrying. These tools
are not advertised to ordinary Owner MCP sessions and never fall back to Owner
credentials. The existing CLI remains available for the same reporting contract.

## Resend administration

`cuan integration resend show`, `set --key-from-env NAME --from-email EMAIL --from-name NAME`,
and `validate` use the authenticated Settings API. Secrets are imported only by
explicit environment variable name and omitted from output. Existing automatic
email preferences are preserved; validation sends no email. See
[Resend guide](../../guides/resend-integration.md).
