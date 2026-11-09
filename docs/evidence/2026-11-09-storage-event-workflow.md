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

Initial fixture's independent review/final acceptance and a fresh post-repair
native artifact routing check remain unproven at this observation.
