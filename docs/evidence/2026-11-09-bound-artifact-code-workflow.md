# Bound artifact access: native code workflow

## Requested result

Enable active task workers/verifiers to read complete files already delivered by
attempts of that same task. Existing `delivered_artifacts` excerpts are useful
context but cannot establish inspection of omitted text or binary image content.

Implement through Cuan's authenticated task capability, worker CLI and MCP.
Reuse existing attachment/access helpers. Keep owner/task/active-attempt/original
room/source-message binding. Download into the current attempt's private artifact
directory; reject unrelated attachments, expired grants, path escape, overwrite,
HTTP errors, partial transfer and excessive size. Transfer is not acceptance or
evidence that file content was independently checked. Do not use Owner credentials
or assume the worker shares the server filesystem.

Include actual file-transfer and access-boundary tests plus usage documentation.
No UI changes, user-data mutations/deletion, merge or deployment. Save/push the
task source and verify that exact commit independently before final handoff.

## Production submission

Room: `d9fa85bd-0edb-4e89-b95a-f25f733e1ffa`.
Request: `9e521da5-9866-4d20-bf4d-64df2a8e4d33`.

The task was requested through the official CLI as a real Cuan implementation,
not a prescribed internal execution lane or forced acceptance. The CLI send
observation timed out after ten seconds; reading the same room confirmed one
running request, so it was not resent. No task was created at that observation.

This evaluates the actual user → PM → developer → independent verifier workflow
for useful code work, rather than repeating the resolved account investigation.

## Implementation details

1. **Backend Capability Endpoint (`internal/server/task_agent_artifact.go`)**:
   - Endpoints: `GET /api/v1/task-agent/{id}/artifacts/{attachment_id}` and alias `GET /api/v1/task-agent/{id}/attachments/{attachment_id}`.
   - Authenticated strictly via active task capability token (Bearer token hashed against `s.taskAgentGrants`).
   - Boundary checks: verifies grant expiration, bound task ID, running status of latest attempt, source room and message binding.
   - Rejects unauthenticated requests (401), expired grants (401), foreign task IDs (403), inactive/ended attempts (409), invalid attachment IDs / path escapes (400/404), and attachments from other tasks (404).
   - Supports metadata inspection (`?metadata=true`) and full binary/text file downloads with strict Content-Type, Content-Disposition, nosniff, and sandbox CSP headers.

2. **CLI Worker (`internal/taskcli/artifact.go`)**:
   - Subcommands: `cuan-server task artifact download <attachment-id> [--output filename]` and `cuan-server task artifact read <attachment-id> [--raw]`.
   - Resolves target within the attempt's private artifact directory (`CUAN_TASK_ARTIFACTS_DIR` / `c.ArtifactsDir`).
   - Guardrails: prevents path escape (`..` or absolute paths outside artifact dir), prevents overwriting existing files, validates Content-Length against received bytes, rejects partial/error responses and responses exceeding 25 MB.

3. **MCP Tool Extensions (`internal/diagnostics/mcp_task_session.go`)**:
   - Registered tools: `download_current_task_artifact` and `read_current_task_artifact`.
   - Uses the private task capability session without accepting task IDs or Owner credentials.
   - Delegates directly to `taskcli` artifact download/read implementations.

4. **Targeted Automated Tests**:
   - `internal/server/task_agent_artifact_test.go`: `TestTaskAgentArtifactDownloadAndGuardrails` (11 subtests covering binary PNG transfer, text Markdown transfer, alias endpoints, metadata queries, unauthenticated rejection, expired grant rejection, wrong task ID rejection, path traversal rejection, ended attempt 409 rejection, multi-attempt verifier access, and other-task attachment rejection).
   - `internal/taskcli/artifact_test.go`: `TestArtifactDownloadSuccess`, `TestArtifactDownloadCustomOutput`, `TestArtifactDownloadPathEscapeRejected`, `TestArtifactDownloadAntiOverwrite`, `TestArtifactDownloadPartialAndErrorResponses`, `TestArtifactReadText`, `TestArtifactDownloadDirectCommand`.
   - `internal/diagnostics/mcp_task_session_test.go`: `TestTaskSessionMCPArtifactTools`, `TestTaskSessionMCPUsesOnlyBoundCapability`, `TestTaskSessionMCPWithoutCapabilityNeverUsesOwner`.
   - All tests pass 100%.
