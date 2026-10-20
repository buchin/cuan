## Default and repository projects

Cuan creates a general-purpose project named **Default**, with the stable ID and
URL segment `default`. It has no repository or GitHub requirement. Studio and
Inbox do not ask users to select it. Its goal and instructions can be edited;
project-specific names and repositories belong in separate projects.

For repository work, Cuan prepares a missing local base ref automatically before
reserving execution. Fetching does not checkout, reset, merge or discard files.
Existing task worktrees keep their branch and pending edits. A temporary download
failure stays queued for another preparation attempt, without starting an agent.
Unavailable access or a removed selected branch remains a reported prerequisite.

## Separate a legacy configured Default

Older installations can have a real repository project with ID `default`. Do not
delete it or clear its settings manually: that can lose history and task bindings.
The explicit maintenance command preserves it as a separate project and creates
a neutral Default in one database transaction. All relational project references
move together, including optional modules with a `project_id` column. Task IDs,
attempts, conversations, approvals, task instructions and workspace files remain.
A scoped previous-ID binding preserves encrypted historical workspace versions.

Use the application release containing this command and historical-ID support.
No schema migration is needed (schema 25). Build with:

```sh
go build -o cuan-normalize-default ./cmd/normalize-default
```

Set `DATABASE_URL` privately in the process environment. Preview first:

```sh
./cuan-normalize-default
```

Validate a private database/profile/configuration backup, confirm no active Inbox,
queued launch or task execution, and stop the Cuan application service. Apply:

```sh
./cuan-normalize-default --apply --service-stopped
```

Restart the same supported application release; verify Default is named Default
with no repository/path/credentials and the preserved project still has its
repository, tasks, conversations and original workspace. Repeating the command
is a no-op. The command refuses active work and rolls back on any conflicting
module reference. Error detail is kept in a private maintenance log under HOME.

Project URLs change for the preserved project; task/conversation/session URLs
retain their IDs. Old approval records are historical facts and are not rewritten
or re-executed. Use the current task's project ID for a scoped continuation.

For rollback before admitting traffic, restore the private pre-operation database
backup while the service is stopped, then reactivate the previous application.
After new traffic, do not restore an old database over new messages: reconcile
new records first. No repository or workspace directories are deleted by this
operation.
