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

This first implementation provides connection management and paginated repository
discovery. Project repository picker and shared access in execution are still
pending. It is not an OAuth login and does not automatically commit, push, merge,
or deploy. GitHub is optional; projects without repositories remain supported.
