## Native scheduler event observation

Production release08f63c831133d7bb8e4a90588e8685368730d861 passed exact-SHA health.
Before the event test, official CLI reported storage maintenance enabled/idle,
with next periodic run2026-11-09T10:25:35.710687623+08:00.

Root created isolated room441e1c31-1b21-4f5c-a004-639b184d7fdd and sent a small
standalone Markdown artifact request with no repository/build/browser/server/
customer/push/deployment access, requesting independent verification.
Request09b12a59-1e5f-4db0-8266-dd1f46d99f0a validated first-pass after134.2s.
Taskmsg_4bccd1b5-dcbc-44cd-b917-93f7b5004787 developer
att_e9d1d88cbf658841449801459e0d99b1 finished2026-11-09T10:19:59.691619+08:00.
Without a manual maintenance trigger, the scheduler ran storage maintenance at
10:20:15.712963474+08:00 and reported success, with its next run10:35:15.711558494.
This run precedes the prior10:25 periodic deadline and follows real attempt
completion. It proves production completion-to-scheduler-to-worker integration;
it does not prove bytes were reclaimed or all retention problems are solved.

## Artifact routing defect exposed by the same fixture

The first artifact attempt was incorrectly classified as code and Cuan created
a repository checkout. Its normalized scope said`Jangan membuka atau mengubah
repositori aplikasi`, which the standalone-artifact parser did not recognize.
Its no-repository matcher accepted only a single modification verb after negation.
The worker created its170-byte output in the separate session artifact directory;
that does not erase the unnecessary checkout/native repository checks.
Retain the room/task/history; do not count it as a clean standalone routing run.

The matcher now recognizes a bounded read/open verb plusand/or/dan/atau before
an explicit modification prohibition. Standalone file intent and no-deployment
conditions remain required; unrelated read-only/implementation tasks are excluded.

Tests reproduced both parser false-negative and real claim choosing code before
fix. After fix, standalone-policy, actual project workspace and actual runner
claim regressions pass, with no repository creation or project environment in
the artifact claim. Existing negative scope cases remain passing.

An additionally selected older context test,
TestRunnerClaimHandlerDoesNotOverwriteSavedProjectContext, fails its prepared
report expectation on both unchanged baseline (Go overlay) and this worktree.
It is not credited as passing or attributed to this parser correction.
The exact relevant suite is:
`go test ./internal/executionpolicy ./internal/project ./internal/server -run TestStandaloneArtifact -count=1`.

## Final first-fixture outcome and repair deployment

The first fixture reached native Done at2026-11-09T10:24:27.35084+08:00 after
one developer and one independent verifier attempt. Root downloaded attachment
f5d456d4-a317-45a9-a8cc-ed1a6f61a793 through `cuan chat download`; its170 bytes
have SHA256 `b903747b2d3c581e6a87b03e0bc310dfcd337daf503375bfc15102ac3b8f3d50`
and contain the requested heading and three supplied facts.

This confirms delivery, independent file review and the earlier event trigger.
It does **not** confirm compliance with all isolation restrictions: native runner
evidence records a repository checkout and `test.sh --fast`, contradicting the
task's broad completion claim that no application tests ran. Preserve that
history; a worker/verifier statement cannot override native execution evidence.

Production29aa922a4f71a334745ea26aee00ae9e1b935bcc passed exact-SHA health with
37 migrations and includes the routing repair. An initial release gate exposed
a detached Go telemetry writer racing test temporary-directory cleanup; the
fixture now disables telemetry and the repeated relevant gate passed.

Fresh post-repair room82bcef2e-bde6-4cc4-9602-7abea68c8718 received request
a08baf05-bc09-4ee9-88c4-434fa1c29994 at10:31:34+08. It requests the same exact
file, private artifact execution and independent verification, explicitly forbids
repository checkout/application tests and preserves the coordinated prohibition
that exposed the parser defect.

## Post-repair production result

Taskmsg_28f9a870-dbd8-4428-a4e6-5e4abbfc4301 used the ops lane. Native runner
logs identify private artifact workspace
`/opt/cuan/data/projects/artifact-workspaces/b565f92f70d9560a8be181cedbc8a4a5a7320175c67f051dae1aa244ad163994`
and explicitly record no Git checkout. Read-only filesystem inspection measured
28KB and confirmed no `.git`. This avoids the earlier approximately200MB clone.

Attemptatt_6fe7ddfbbf0cb4cfd9c0ce9d9eb428a6 ran10:33:31.440213 through
10:35:19.67357+08 and succeeded. Its native test summary records no supported
project verifier rather than a registered application test run. The task reached
Done10:36:11.329941+08, approximately4m37s after the original message, with one
attempt and no manual retry/status acceptance. Root downloaded attachment
f300a41e-1185-4391-af80-3bbaae0e26d3 officially and compared it byte-for-byte with
the first fixture:170 bytes and the same SHA256 above, exact requested contents.

Verification independence is limited: the worker invoked an AGY subagent
(`research: Independent Verifier`, conversation5a3bc3f8-776e-4975-927c-270203e22cf1).
There was no separate native verifier attempt. This proves artifact routing and
delivery, not the broader PM/developer/native-verifier separation goal.

Without a manual maintenance trigger, actual scheduler status recorded success
at10:35:34.083927518+08, about14 seconds after worker completion, before the prior
periodic deadline10:40:24.082090046+08. Its next periodic run moved to10:50:34.
This repeats completion-to-maintenance production proof after the routing fix.
The filesystem still had only3.1GB available (96% used) before completion: success
does not establish reclaimed bytes or solve retained snapshot lifecycle.
