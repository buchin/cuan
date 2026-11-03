# Project environments through CLI and MCP

Agents can manage the same environment shown in **Project settings → Environment**
without browser automation. Ordinary users can continue using that screen.

## Connection and access

Use the existing Cuan CLI connection (`CUAN_API_URL`, `CUAN_AUTH_TOKEN`, or
`~/.cuan/config.json`). Mutations use the existing owner-authenticated settings API;
no database access or separate server endpoint is required. Use the exact project
ID from `list_projects`, rather than its display name.

## CLI

```sh
cuan project env list PROJECT_ID
cuan project env set PROJECT_ID --from-env DOJO_TERMAPI_VPS_USERNAME --from-env DOJO_TERMAPI_VPS_PUBLIC_IP --from-env DOJO_TERMAPI_VPS_PASSWORD
cuan project env set PROJECT_ID --file /private/path/project.env
cuan project env set PROJECT_ID --file - < /private/path/project.env
cuan project env unset PROJECT_ID --key OLD_VARIABLE
```

`--from-env` imports only explicitly named variables from the current process.
This avoids putting secret values in shell history or process arguments. File and
stdin input accept one `KEY=VALUE` per line, blank lines, comments beginning with
`#`, and optional surrounding single or double quotes. Empty values are supported.
Variable names must match `[A-Za-z_][A-Za-z0-9_]*`. Multiline/NUL values, duplicate
input keys, and values with surrounding whitespace are rejected. Values containing
`#`, `=`, `$` and shell syntax are sent literally; nothing is executed. The maximum
saved environment is 32,000 characters.

Output is JSON and redacts **every value** by default. Only
`cuan project env list PROJECT_ID --reveal` exposes saved values; avoid using this
in shared terminals or agent transcripts. Writes always return redacted output.
General MCP project inspection also redacts the environment field.

## MCP (`cuan mcp`)

- `get_project_environment`: `{ "project_id": "PROJECT_ID" }`. Optional
  `reveal: true` explicitly includes values.
- `update_project_environment`: `{ "project_id": "PROJECT_ID", "from_env":
  ["DOJO_TERMAPI_VPS_USERNAME", "DOJO_TERMAPI_VPS_PUBLIC_IP", "DOJO_TERMAPI_VPS_PASSWORD"] }`.
  Optional `values` is an object of string values; prefer `from_env` for secrets
  to keep values out of tool arguments. Optional `unset` is an array of key names.

Set operations merge selected keys, replacing all previous assignments of those
keys while preserving unrelated variables and comments. Unset removes only the
selected keys. Other settings, including repository, deployment branch, goal,
instructions and auto-deploy, are preserved. The client passes the current
`expected_updated_at`; a concurrent edit returns a conflict rather than overwriting
someone else's changes. Review current settings before retrying a conflict.

## Verification and recovery

Read back using `cuan project env list PROJECT_ID` to verify key names without
revealing values. Successful writes also compare the server response with the
requested changes in memory. API errors omit response bodies to prevent accidental
secret disclosure. If the response cannot be decoded after a write, read back
before retrying because the server may already have saved it.

To roll back an addition, use `unset --key NAME`; to restore a previous value,
import it from your private source with `set --from-env NAME` or `--file`.
This CLI does not restart Cuan or deploy application changes. The existing
project environment mechanism applies the saved configuration to new tasks;
existing tasks keep their original project guidance snapshot. Values use the
same storage and access model as the UI; masking output does not add encryption
or change which project agents receive them.
