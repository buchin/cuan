# Retaining task progress during recovery

Preparing an existing workspace never runs `reset --hard`, `clean -fd`, removes
the workspace, or forces an existing task branch to a different commit.

If the requested branch is already checked out, preparation leaves the index,
uncommitted files and in-progress merge intact for the task agent to inspect.
Switching to another existing branch uses ordinary checkout and fails with saved
work preserved if the switch would overwrite it. A missing local task branch is
restored from its available `origin/<task-branch>` ref before creating a new one.
This does not fetch missing remote refs or restore files deleted by older code.

Age/count cleanup is conservative. Unknown repositories, tracked/untracked or
ignored files, unresolved conflicts, stashes and commits not covered by existing
remote-tracking refs are retained. Inspection and deletion hold the same OS lock
used by execution, and the lock inode is retained. Each sweep has a ten-second
budget. Therefore retention can exceed the configured age/count targets; an
explicit archive/acceptance-aware cleanup policy is still needed for those files.
Remote-tracking refs are local Git evidence, not a new live remote durability
check, and unreachable objects/reflog-only historical work are not proven by this
policy.

Validation uses actual Git repositories and checks conflict/index contents,
branch commit IDs, dirty/untracked files, remote task branch restoration, ignored
checkpoints, unpublished commits, stashes, active locks and clean cleanup:

```sh
go test ./internal/git -count=1
go test ./internal/runner -run 'TestReadOnly|TestRunProjectVerification|TestExecutionCompletionFences' -count=1
```

No production task is resumed, merged or marked complete by these tests.
