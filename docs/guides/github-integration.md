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
