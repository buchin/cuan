# GitHub connection

## Current release status

Release `f40909a` includes integration service cards, contextual task connection
requirements, the task-bound push tool, scoped queued continuation and OAuth
implementation. Desktop/mobile browser tests, deterministic process/real-Git
continuation tests and production read-only/rollback/retention checks pass.

This installation has no configured OAuth App, so ordinary GitHub authorization
is not yet available. The normal connection screen has one primary action:
**Sign in with GitHub**. Manual tokens and installation setup are hidden under
**Advanced**. The one-time OAuth setup below belongs to the installation owner;
ordinary users should not copy tokens or configure an OAuth App. If OAuth is
unavailable, clicking sign-in explains the prerequisite and preserves work. No real
provider inference or live GitHub write was used in verification. Historical
implementation checkpoints below are superseded by this release status.


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

Combined worker verification now passes with real worktrees and fixture GitHub refs: an active execution lease prevents even a remote query; remote failure retains files; eligible completion removes its checkout; reopening restores the result; task records remain unchanged. Provider-runtime compatibility is still unverified, so the changes remain unreleased.

The actual PTY/supervisor path now passes a deterministic harness test that runs real Git commands, commits task output, reconstructs the terminal manager and continues with exactly one implementation commit. This replaces inference only and does not establish every provider sandbox behavior. Release activation now blocks worktree-unaware binaries after isolated repository tasks exist, including automatic health fallback. Six release compatibility contracts pass.

## Save a task branch

An active agent session can run `"$CUAN_TASK_EXECUTABLE" task push` after committing
its scoped changes. The capability permits only its current task and active
attempt. Cuan resolves the frozen repository/workspace, validates the managed
branch, and pushes the exact captured commit to that branch using stored access.
The GitHub credential stays out of the prompt, command arguments and remote config.

This command does not stage or commit files, save uncommitted changes, merge,
force-push, create a repository or deploy. Repeat push is safe for the same commit;
remote divergence is rejected. If the response is lost, rerun push to confirm the
same branch state rather than repeating implementation. Missing GitHub access
returns an actionable connection message; contextual connection/resume UI is still
pending. Legacy tasks on shared project branches cannot use this managed-branch
push until explicitly migrated. Tests use real local Git remotes; no live GitHub
write has been made. This command is not yet deployed.

## Missing connection in an agent task

A task push without stored GitHub access now records a system prerequisite event
and returns `github_connection_required` with a connection URL. Task details and
session pages display **Connect GitHub** while that event is unresolved. Human
instructions, current task state and local files stay unchanged. The connection
page accepts a validated local task return path and offers **Back to task** after
connection. Arbitrary/external return URLs are rejected.

A subsequent successful task push records a resolution event and hides the CTA.
This checkpoint preserves context and navigation, but does not yet automatically
restart the agent or retry the push after connection. That continuation remains
pending; connecting GitHub must not be described as completing the task. These
changes are not yet deployed or browser-verified.

A recovery worker now scans up to twenty blocked tasks per thirty-second pass
when a saved GitHub connection exists. It queues a scoped continuation through
the durable launch system only when the latest prerequisite belongs to the latest
finished attempt, original Inbox authorization still matches, and the task's
instruction fingerprint is unchanged. The prerequisite event ID deduplicates
launches across scans/reconstruction. Cancelled attempts are excluded; manual
tasks without original Inbox authorization are retained for explicit continuation.

Tests prove changed scope/wrong owner do not queue and repeated observations keep
one durable launch. This is queue-level verification, not proof of a live provider
completing the subsequent push. Browser and combined continuation verification,
plus deployment, remain pending.

## GitHub authorization without manual user tokens

The main connection action now opens GitHub's authorization page. Manual access
tokens are available under Advanced. A self-hosting administrator must register
one GitHub OAuth App and configure these private service environment variables:

- `CUAN_GITHUB_CLIENT_ID`: the registered OAuth App client ID.
- `CUAN_GITHUB_CLIENT_SECRET`: its private client secret.
- `CUAN_GITHUB_REDIRECT_URL`: the exact callback URL registered on GitHub,
  e.g. `https://cuan.example/api/v1/integrations/github/oauth/callback`.

Register the OAuth App under GitHub Settings → Developer settings → OAuth Apps.
Use the public Cuan origin as Homepage URL and the exact callback above. Restart
the Cuan service after configuring its protected environment. Ordinary users then
click Connect GitHub and review GitHub's permission page, without copying tokens.
The OAuth App requests GitHub's `repo` scope, which includes private repository
access; it is not the repository-selective GitHub App installation flow. Fine-
grained manual tokens remain an Advanced option for narrower repository grants.

Authorization is owner/browser-bound, expires after ten minutes and is consumed
once. Service restart expires an in-flight authorization; retry connection without
recreating the saved task. Cancellation/failure returns to Settings without
changing the task. Tokens and provider error bodies never enter redirects.

OAuth App environment is currently absent in this installation. The implementation
and fixture callback tests pass; real GitHub authorization and browser verification
of the updated primary/Advanced UI remain pending. It is not yet deployed.

## Human authorship for task commits

Task sessions receive Git author/committer identity automatically. Connected
GitHub accounts with a known numeric ID use their public name/login and GitHub
no-reply email. Otherwise the authenticated Cuan owner's username/email is used.
Set `GIT_AUTHOR_NAME` and `GIT_AUTHOR_EMAIL` in the service environment to override
this explicitly (for example with another verified no-reply address). Identity
is passed only to the task process; no global Git config is overwritten.
Legacy standalone runners require explicit identity configuration. This automatic
identity change is deployed in release f40909a.
