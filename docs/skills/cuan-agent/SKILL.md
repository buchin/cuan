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
