# Task execution baseline

Task workspaces use the project's selected branch rather than the cache's
currently checked-out HEAD. The selected cache ref is imported locally as
`refs/remotes/cuan-base/<branch>`. A new task branch starts there. Existing task
branches, merge state, and pending files remain intact.

When recovery follows `Project verification failed`, Cuan refreshes the approved
repository branch using the project's registered repository connection before
reserving another attempt. A failed refresh returns `repository_preparing`;
it does not reserve an attempt or discard saved work. No ambient developer
credentials are used. This refresh changes refs, not the source checkout.

Each workspace preparation resolves the imported ref to a full commit SHA in
`TaskWorkspace.BaseCommit`. New branches start from that commit. The runner receives
that exact SHA in its instructions and uses the same snapshot for documentation
verification, code-change detection and task diff summaries. Later updates to the
tracking ref cannot change the baseline of an already prepared attempt. A retry
can import and pin a newer approved commit while preserving the earlier attempt's
snapshot and all retained task files.

An unavailable/unknown comparison is not evidence of an unchanged repository.
Deployment compliance rejects an unverified comparison instead of silently using
it to skip required deployment evidence. Older workspace values without the new
SHA field retain their existing ref/default-branch fallback.

An empty diff against a pinned commit or bound ref stays empty instead of falling
back to the preceding commit.
Publication still targets the actual project default branch; an internal
comparison ref never becomes the GitHub pull request base.

Retained task changes are not automatically rebased. The executing agent must
inspect and reconcile the current baseline within its approved scope, then pass
verification. Updating refs alone does not prove that the task is complete or
deployed. This recovery refresh currently applies specifically to project
verification failures, not every new attempt.

Targeted checks:

```sh
go test ./internal/git -count=1 -timeout 30s
go test ./internal/project -run 'TestExecutionWorkspace|TestExecutionWorkspaces|TestRelativeExecutionWorkspace|TestSharedAccess' -count=1 -timeout 30s
go test ./internal/runner -run 'TestExecutionBaseline|TestPinnedCommit|TestDocumentation|TestUnknownDocumentationBase' -count=1 -timeout 30s
```
