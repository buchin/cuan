# GitHub connection continuation verification

Clean frontend 8292269 passes fixture browser checks at 360/820/1440 pixels:
task prerequisite → connection → same task return, external return rejection,
resolution hiding the CTA, and no overflow. The browser test exposed a missing
CTA in the task ready state; that state now renders the same prerequisite panel.

`TestTaskPushConnectionRecoveryRealGit` exercises the actual task capability
endpoint, encrypted owner connection store, frozen project worktree, real commit
and local bare remote. Missing connection records one prerequisite; connecting
allows repeated push of the same existing commit and records one resolution.
Instructions stay unchanged and implementation commit exists exactly once.

`TestGitHubRecoveryQueuesOnlySameApprovedScope` separately proves original Inbox
owner/scope checks, instruction fingerprint, durable launch reuse and service
reconstruction. Existing launch queue tests cover route availability and process
launch. These checks do not yet combine a real model-provider continuation with
GitHub push, and no live GitHub writes or production AI inference were performed.

`TestGitHubQueuedProcessContinuationRealGit` now passes the combined path: missing connection → retained local commit → connected owner credential → deduplicated durable launch → actual PTY/supervised deterministic agent → task push → final done report, with one continuation and exactly one implementation commit. It exposed a recovery request ID validation bug, fixed by deriving a stable UUID from the prerequisite event. No real model inference or live GitHub write is claimed.
