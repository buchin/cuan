# Project settings

Open a project and choose Settings from its menu. Edit the project name, goal,
and instructions for Cuan. Save changes becomes available after an edit. Failures
retain the draft; a stale version offers an explicit reload rather than
overwriting another tab's changes. Discard changes restores the saved form.

Goal and instructions are context for new work, not authorization to expand a
task. They reach coordinator project context and are frozen with each new task.
Task instructions shown to users omit the internal project binding snapshot.
Existing tasks retain their original guidance during revisions and recovery.

## Repository connection

Advanced allows an optional GitHub repository URL and default branch. Cuan uses
the existing saved GitHub access; credentials must not be embedded in the URL.
Saving a changed connection asks for confirmation, verifies access and branch,
and prepares a separate workspace. A failed clone or branch check retains the
old configuration. Clearing the URL creates a separate blank workspace.

Previously created tasks retain their original workspace, including recovery
after restart. Running sessions never have their working directory replaced.
The old directories are intentionally retained. Do not manually delete them
while tasks may be resumed. Up to 100 connection versions are retained; further
changes require an administrator to review references and retention.

The application stores authenticated historical bindings in existing project
metadata; no database migration is required. Unknown or tampered bindings stop
execution rather than silently using a different workspace.

## API

Owner-authenticated PATCH /api/v1/projects/{id}/settings accepts name (1–120
characters), goal (up to 2000), instructions (up to 4000), repo_url (GitHub URL or
empty), default_branch, and expected_updated_at from the saved project. Success
returns the canonical saved project/version. Stale writes return 409, invalid
settings or inaccessible connections 400, missing projects 404. Connection
verification is bounded to 90 seconds. Unrelated metadata, modules, credentials
and archive state are preserved.

## Deployment and rollback

The native release manifest advertises project_workspace_revisions=1. Once a
connection history exists, activation and automatic fallback reject releases
without that feature. Roll forward to a compatible release to retain task
workspace guarantees. This gate does not run a down migration or rewrite project
data. The standalone native installer does not provide a rollback command;
administrators must apply the same compatibility constraint to manual switches.
