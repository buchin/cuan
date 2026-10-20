# GitHub connection

Open Settings → Integrations → GitHub. Create a fine-grained access token on
GitHub for the repositories Cuan should access, then connect it once. The token
is validated against GitHub and stored encrypted in a private owner-scoped file.
It is never returned by connection or repository-list endpoints.

Grant Contents read access for cloning or read and write for pushing. Selecting
repositories in a fine-grained token limits what the integration can discover.
An account connection does not prove every repository permits writes. Tokens can
expire or be revoked; reconnect when GitHub rejects access.

Disconnect removes Cuan's saved connection; it does not revoke the token on
GitHub or remove credentials previously stored separately for older projects.
Revoke the token in GitHub settings to remove its upstream authority.

In Project Settings → Advanced, choose **Choose from GitHub** to browse accessible
repositories. Load more for additional pages, or enter a repository URL directly.
A changed repository uses GitHub's default branch unless you specify an override.
Saving verifies and prepares a separate workspace; existing tasks keep their
original workspace. The connection also supplies access for project sync and
branch discovery. Existing explicit project credentials take precedence.

Shared tokens are read on each repository operation, so replacement or disconnect
applies to the next operation without copying credentials into projects. This is
not an OAuth login and does not automatically commit, push, merge, or deploy.
Agent runtime push access and the complete Git delivery workflow need separate
verification. GitHub is optional; projects without repositories remain supported.

## Task workspaces

New repository tasks carry an isolation flag in their frozen project context.
Execution prepares a stable Git worktree and branch per task and repository
workspace revision, from the locally fetched remote default branch. Resuming
reuses that worktree and keeps pending edits. Changing the project connection
continues to preserve the original repository binding.

Tasks created before this change retain their original project directory; the
upgrade does not move their pending work. Existing branch changes, missing task
workspaces and repository mismatches stop preparation with an explicit error
rather than recreating work silently. Worktrees and task branches are retained;
there is no automatic cleanup or merge yet. This has local real-Git verification;
provider-runtime compatibility and automatic Git delivery remain pending.

## Storage retention safeguards

The retention inspector considers only completed tasks after seven days, with no
active execution and an independently confirmed remote commit matching local HEAD.
Tracked changes, untracked files and ignored files all prevent eligibility. It
never treats a folder named node_modules, dist or cache as proof that its contents
are disposable. Active, blocked, review and recently completed tasks are retained.

The inspector is a prerequisite, not an enabled cleanup scheduler. Automatic
removal still requires an execution lock, remote verification, a durable retirement
record and tested restoration for reopened tasks. Until those are implemented,
workspaces are retained and this feature must not be described as automatic cleanup.

The retirement primitive now requires an exclusive execution lease and a fresh
state observation under it. It writes an authenticated retirement receipt, removes
only a clean worktree without force, and keeps the branch. A reopened task can
restore that same branch and committed files using the confirmed receipt.
Interrupted/unconfirmed retirement or a changed retained branch requires review.
Real-Git tests cover removal, restart restoration, active-lease refusal and keeping
new revision files. The automatic scheduler and live remote verification are still
not wired, so automatic cleanup remains disabled in production.

## Automatic cleanup implementation status

The server now starts an hourly retention worker. It examines at most ten done
tasks per pass, rotates through candidates, acquires the same OS execution lease
as task processes, rechecks task status, and reads the task branch ref directly
from GitHub. Missing/expired access, absent remote branches or changed commits
retain the workspace. Each observation has a 20-second deadline. No task records,
chat history, branch refs or shared project clones are deleted.

Confirmed retirement receipts are authenticated, synchronized to disk and consumed
once on restoration. A pending receipt or changed retained branch stops automatic
restoration for review. The worker and worktree isolation have not yet been
released to production; runtime compatibility and combined worker verification
remain prerequisites. Earlier implementation notes above describe the stages
leading to this wiring, not separate active cleanup mechanisms.
