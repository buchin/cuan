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

The runner receives the imported baseline in its instructions and uses it for
documentation verification and task diff summaries. An empty diff against a
bound baseline stays empty instead of falling back to the preceding commit.
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
go test ./internal/runner -run 'TestDocumentation|TestUnknownDocumentationBase' -count=1 -timeout 30s
```
