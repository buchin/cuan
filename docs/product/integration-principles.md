## Integration design

Cuan serves people who want work completed without managing developer tools.
For future integrations, prefer a provider's maintained official CLI when it
provides a reliable supported authentication or execution flow. Present the
integration through Cuan's UI. Users should not need a terminal, command flags,
manual tokens, or knowledge of how the integration is implemented.

This is a preference, not a requirement to use a CLI for every capability. Use
an official API or SDK when it better supports webhooks, incoming messages,
streaming, permissions, scale, or a more reliable browser authorization flow.
An integration may use a CLI for sign-in and an API for its application features.
Record the choice and its practical limitations in the integration guide.

## User experience

- Start with one clear contextual action, such as Sign in with GitHub or Connect
  Google. Choose technical defaults automatically; do not require an agent name,
  token, repository branch, or CLI configuration in ordinary onboarding.
- Use the provider's authorization page for account selection and consent. If a
  device code or authorization code is required, explain it plainly and show only
  that necessary step. Do not claim the connection is complete before validation.
- Show useful progress and the connected account. Provide retry and cancellation.
  Repeated clicks and reloads must reuse a live login when possible. If a login
  expired or was lost after restart, explain how to resume without losing work.
- Keep task approvals, project choices and saved messages intact through sign-in
  failures. Continue already approved work through its existing task after access
  is verified; do not duplicate tasks or request the same approval again.
- Keep manual tokens and installation settings under Advanced when needed for
  compatibility. An unavailable method must offer an actionable explanation,
  rather than an inert button or a misleading claim of successful connection.

## Implementation boundaries

Run each provider with a private account/owner profile and an allowlisted process
environment. Never inherit ambient credentials, host provider tokens, another
account's profile, SSH agent access, or a global CLI login. Store retained access
encrypted; remove temporary credential files after a successful handoff or
cancellation. Preserve explicit provider consent and existing preferences.

Cuan owns process lifetimes, bounded output, polling, cancellation, retry policy,
and dependency preparation. Prevent cancelled or superseded logins from saving
late credentials. Verify official downloads before installing a missing CLI in
Cuan's private runtime directory. Do not restart a service during a live browser
sign-in without a supported recovery path.

## Verification and documentation

Verify the actual sign-in path, reload/repeated clicks, failure, expiry and cancel,
plus account isolation and credential storage. Distinguish a real authorization
challenge from a completed account connection, and fixtures from native evidence.
Test the UI at mobile and desktop widths. Document permissions, prerequisites,
reconnect/disconnect, limitations, and any provider-specific required steps.
Never borrow a developer's existing credentials to make onboarding appear ready.

GitHub's current implementation follows this approach: Cuan starts an isolated
GitHub CLI browser login, presents its device code in the UI, and uses the saved
encrypted connection for repository operations. GitHub presents GitHub CLI's
consent and OAuth scopes; this is not a repository-selective GitHub App flow.
See [GitHub connection](../guides/github-integration.md).
