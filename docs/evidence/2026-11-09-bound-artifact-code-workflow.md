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

## Interim source review, not final handoff

Developer attempt one remains live and has modified backend, CLI, MCP and test
files, but has not saved a source commit or final report. Its latest checkpoint
describes baseline compilation and implementation preparation; subsequent output
shows test-fixture attempt/runner identity failures under investigation.

A read-only snapshot of the uncommitted worker CLI was independently exercised
outside the live workspace with a Go overlay and the fixture in
`fixtures/bound_artifact_independent_test.go.txt`. Both checks fail:

- A real HTTP `206 Partial Content` response with matching partial-body length
  is accepted as a complete download.
- An output parent symlink inside the attempt artifact directory permits a real
  write outside that directory. The test supplies flags before the positional
  ID so it exercises destination handling rather than argument parsing.

Snapshot SHA-256 values:

- `internal/taskcli/artifact.go`:
  `9204d5f739e3b9ff0e83f6b5de3bee8157d2f9b33ce5c89fe67c9da8a4c99899`
- `internal/taskcli/cli.go`:
  `cd1022073ae6f62f5ad15bd7410e733986b90af3348e092445079436798c7e19`

These findings apply to that interim snapshot, not a future final commit.
The fixture creates only disposable local files and an HTTP test server; it
does not operate on production user data. No live task source, task status,
approval, retry or acceptance was changed. Deployment/merge remain forbidden.
The final native verifier and source review must reject these behaviors if they
remain in the actual handed-off commit.

## Proven orchestration routing defect

Developer attempt one timed out at `2026-11-09T03:12:37.807094+08:00` without
a final report or source commit. Working changes remained in its task workspace.
The dispatcher automatically admitted attempt two,
`att_c313791f4e296725f0f4206c1333f30d`, as current-outcome inspection of the
old running Cuan revision `f4b9c0287c76f19821912882b82bdb55270aa80f`.
That is not verification of the unfinished local feature. The request permits
task-branch publication for review while expressly forbidding merge/deployment.
Attempt two failed native visual/project preparation at
`2026-11-09T03:14:43.519444+08:00`; it establishes no feature verification.

The exact native planner guard required both deny-push and deny-deploy before
rejecting substitution of an unrelated running release. The correction uses
deny-deploy alone, retaining the existing exception for native-proven publication
containment. Push permission is not deployment scope. A new lifecycle scenario
permits task-branch push, forbids merge/deploy and uses a timed-out worker without
a source handoff. It fails against the old planner and passes with the fix.
Existing live-inspection, unpublished-outcome and supplemental-verifier lifecycle
tests also pass. This planner repair is not yet deployed.

The PM's subsequent saved assessment requests implementation rather than more
passive inspection. Its claim of no saved code must not be read as absence of
working files: the actual uncommitted workspace was independently observed.
No historical failure was changed and no task acceptance/retry was forced.

## Saved PM correction stranded after typed inspection

Release `455444a767feea81f94672f28a3cab533db363aa` passed exact-SHA health
with 37 migrations, but the existing task remained blocked. The saved PM decision
was revise/implementation/replan. Native guards correctly prevented mutating the
read-only inspection, but also rejected a separate developer continuation because
there was no tested source commit to hand off.

A local correction now admits only a new developer attempt when the exact saved
PM decision belongs to this task/owner, the finished current-outcome inspection
is natively bound to its original timed-out code worker, no source handoff exists,
and original owner scope allows source work but forbids deployment. It reuses the
ordinary approval, cancellation, repetition, durable launch and workspace rules;
the inspector's snapshot is never granted write scope and failed history is not
rewritten. The queued successor's native claim is tested as implementation with
push allowed, deploy denied and no inspection/verifier intent.

The lifecycle regression fails against the old manager admission and passes with
the correction. It rejects changed scope, revision, owner, cancelled inspection
and an already established source handoff, and checks launch dedupe/history.
Targeted manager/source-inspection tests pass. One older repeated-review fixture
also failed on unchanged HEAD: it requested operational verification without a
source commit while labelled code. The fixture now explicitly uses the ops lane
and holds that same lane; the existing no-owner-deflection assertions stay intact.
This correction is not yet deployed and production resumption is unproven.

## Production admission after recovery repair

Release `04a1ddf1036a5676b820a6b79f57153b7f32a76c` completed successfully
with exact-SHA production health and 37 migrations. Native recovery admitted
attempt `att_24272c99da0e24d3c15763685c3355b8`, claimed at
`2026-11-09T03:35:30.231776+08:00`, without a manual task retry or acceptance.
The official execution diagnostic reports implementation intent, repository writes
and branch push allowed, deployment denied. The task remains doing, not Done.
This proves separate developer admission only; retained-work reuse, final source
quality and independent verification remain unproven. Early terminal activity
still inspects prior native visual-check machinery, so routing admission alone
must not be reported as successful end-to-end recovery or efficient execution.

The independent fixture now also exercises the documented positional-ID-before-
flags download invocation, verifies the requested filename and complete bytes,
and requires repeated download to preserve existing content. On the preserved
interim snapshot, the new case fails because `--output` is ignored after the
attachment ID; the earlier partial-response and symlink-write cases still fail.
These are interim-source findings, not findings against a final developer commit.
Run all `^TestIndependentArtifact` tests against the exact handoff before drawing
any acceptance conclusion. No task data, worker source or task acceptance changed.

## Backend scope falsely classified as visual work

The original task includes “Backend/CLI/MCP saja tanpa modifikasi UI”. Native
inspection classification stripped “tanpa perubahan UI” but did not recognize
“tanpa modifikasi UI”, leaving the UI keyword as a positive visual requirement.
A regression using that actual scope fails before correction (`vision=true`).
The existing bounded mutation-prohibition matcher now recognizes `modifikasi`.
A paired case still requires visual evidence for “Tanpa modifikasi UI; periksa
tampilan halaman inbox”; actual read-only visual duties must not disappear.
The full inspection-scope table, capture-restriction and original screenshot
requirement tests pass. This changes evidence classification, not task acceptance,
source checks, original authorization or the running developer's current claim.

Release `9b7721a7a72e7b54602e124da30c714c5d1e3d68` passed exact-SHA
production health with 37 migrations. The same developer attempt remained running
across release; no manual retry was issued. At 03:45–03:46 +08 the official logs
and an actual Go compiler process showed active targeted test execution.

A fresh read-only copy of the unfinished worker CLI changed artifact.go SHA256 to
`8fdb25d6f4fdec3b54713febb32ab4d0e83fb543b81e4712a025372a5b8f20d0`;
cli.go retained its earlier hash. Running the independent fixture through a local
Go overlay now passes the documented `--output` and existing-file-preservation
case. Partial HTTP response and parent-symlink containment cases still fail,
including an actual out-of-root write in the isolated test directory. This is
measured partial implementation progress, not final source acceptance. No final
commit or independent verifier handoff was present at this observation.

## Actual developer checkpoint: security acceptance still fails

The developer pushed clean checkpoint
`553bcaf9477fcef2ffb5d6a16858d25a470213ff` on the bound task branch and
submitted its review report at `2026-11-09T03:47:54.359708+08:00`. The report
claims complete security boundaries and 100% targeted test coverage. That claim
is contradicted by independent executable evidence: fetching this exact remote
commit and overlaying its taskcli artifact.go and cli.go into the independent
local fixture still accepts HTTP 206 and writes outside the artifact directory
through a parent symlink. The documented-output test passes. These findings now
apply to an actual committed handoff, rather than an unfinished working copy.
Native final checks and independent task verification remain pending; no task
acceptance, source patch, manual retry, merge or feature deployment was performed.

## Native check failure and committed-local routing gap

Attempt three finished failed at `2026-11-09T03:48:58.049023+08:00`, with
agent exit zero but native `test.sh --fast` exit one. The sole reported failure
was `TestRunner_AutoDeployCompliance_NonCodeInvestigation_SucceedsWithoutDeploy`:
its read-only fixture omitted the now-required unchanged-repository receipt.
The same test fails on the unchanged root branch, independently of artifact code.
The fixture now supplies that receipt; production completion guards are unchanged.
Targeted auto-deploy and real read-only fingerprint tests pass.

Native recovery nevertheless tried an unrelated running release, with its launch
stopped because source/target was absent from the project cache. The existing
no-deploy guard covered current-outcome inspections only, so a retained commit
bypassed it. A committed-local lifecycle regression fails before repair; the
guard now applies to both inspection purposes, retaining the native-proven
publication exception. Targeted local-only, ordinary live-result, unpublished
and supplemental-verifier lifecycle tests pass. These repairs do not change the
failed attempt or make its artifact security failures acceptable. Production
recovery and a corrected final handoff remain unproven.

## Native developer correction succeeded, independent security remains open

The saved PM decision automatically started developer attempt
`att_53b03973c65443a46594be3f4a3eb145` at 03:52:09 +08 (before the later
routing repair deployed). It corrected only the stale runner fixture, preserving
production completion enforcement. The native project suite passed in 31 seconds
after the model report; the attempt succeeded in 10m59 and saved/pushed commit
`bd6238d437c341ecd8932949edf958b14b8a7e85`. No manual retry or acceptance.
Independent tests fetched from that exact remote source still fail HTTP 206 and
parent-symlink confinement. Native suite success does not establish feature DoD.
Separate source verification and PM acceptance remain pending.

Implementation session guidance now distinguishes focused developer checks from
the native post-report registered suite. It discourages repeatedly running an
unchanged suite and requires actual original failure/security acceptance checks;
an explicitly requested pre-handoff suite still runs on final source, with no
waiver of native checks or independent verification. The scoped reporting/review
integration test passes. This guidance cannot change an already issued session.
