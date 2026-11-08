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
No result, source commit, independent verification or efficiency claim yet.

## Developer admission

PM selected registered project `cuan`, category `code`, and created task
`msg_10d11aa8-27f4-40a2-8653-4603c6ed15e6`. First developer attempt
`att_a869213f6e426104c71d7b5aa9d1d254` started at
`2026-11-09T02:47:36.849087+08:00`, with a 25-minute execution deadline.
Official execution diagnostics confirm source writes/push are allowed and
deployment is denied. The single original chat request completed after launch;
no duplicate send, manual retry or acceptance occurred. Developer output shows
it reading the bound task and inspecting the actual source baseline.

## Review checks

Review the generated source and actual test results when handed off. In
particular, check both server and runner executable task dispatch, MCP worker
tool exposure, and real transferred file bytes through the scoped HTTP route.
Preserve existing report/push behavior. A server-side attachment path or inline
excerpt alone is not the requested complete-file transfer.

Check that redirected/error/partial/oversized downloads leave no successful or
partial output, and that exclusive destination creation cannot overwrite existing
work or follow a path outside the attempt artifact root. Server authorization
must come from the active issued capability and original delivered-file binding,
not an attachment UUID, worker-supplied owner/task IDs or an Owner login.

No code result or verification result is established at this checkpoint.
