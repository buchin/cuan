# Typecheck diagnostic outcomes

`cuan repo typecheck --path DIR` and `cuan diag typecheck --path DIR` inspect a
TypeScript project (`tsc --noEmit`) or, if no TypeScript configuration is detected,
a Go project (`go vet ./...`). The JSON result records the actual working directory,
command, status, reason and whether retained output was truncated.

| Status | Meaning | success |
| --- | --- | --- |
| `passed` | The selected checker completed with exit zero | true |
| `failed` | The checker or its output collection failed | false |
| `timed_out` | Execution exceeded the inspection deadline | false |
| `not_run` | No supported configuration or executable could be used | false |
| `canceled` | The caller canceled the check | false |

A zero issue count alone is not a passing result. No supported configuration must
not produce a green success, and historical agent claims are not diagnostic output.
A passed diagnostic covers the selected checker, not every project language,
build, runtime behavior, visual acceptance criterion or task Definition of Done.

Both human-readable and `--json` CLI modes exit zero only for `passed`; other
check outcomes exit one. Usage errors retain exit two. For JSON, the envelope's
`success` agrees with `data.success`. The MCP `inspect_typecheck` tool returns the
same structured result and sets `isError` for non-passing outcomes; the MCP server
process stays available for later calls. This corrects earlier behavior where
unsupported projects and CLI envelopes could incorrectly report success.

Inspection commands retain at most 128 KiB of output while continuing to drain
excess bytes, avoiding a blocked writer at the retention limit. A deadline kills
the command's owned process group; inherited-pipe waits are also bounded. An exited
leader whose child keeps stdout open is a failed inspection, not an accepted pass.
A child that deliberately leaves that group needs its own managed lifecycle.
No dependency installation or task execution retry is triggered by this tool.

## Verification

```sh
go test ./internal/diagnostics -count=1
```

Fixtures exercise engine, human/JSON CLI and MCP outcomes for passing and failed
checks, an unavailable compiler, unsupported projects and inherited-output failure.
Additional real subprocess tests cover timeout with a child holding stdout, caller
cancellation, a healthy 2 MiB output producer under the retention cap, and Go vet
failure. No production tasks or conversations are created by these checks.
