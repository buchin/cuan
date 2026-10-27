## GitHub browser sign-in through the CLI

Production `758f9f3`/schema25, staging `9911437`/schema25. The production release
contains only the scoped GitHub login changes; the larger coordinator candidate
was excluded. Tests and build pass. Deployment created a private database backup
and readiness/health passed without migrations.

Staging used the real UI and native gh to start a GitHub device challenge. No
OAuth App/client ID/client secret was configured. A valid one-time code appeared;
reload resumed the same job. 390/820/1440px layout checks passed. Cancel stopped
that pending login, removed its code, and no GitHub account was saved.

Production Sign in with GitHub also produced a native device challenge without
manual configuration. Reload resumes the same pending owner job. That job was
left available for the owner to authorize on GitHub. No token or live code is
included in committed evidence. The code is available only in the authenticated
Cuan UI. Full real authorization and private repository access remain unverified.

Targeted subprocess tests verify allowlisted environment, isolation from ambient
GH_TOKEN/GITHUB_TOKEN/Fural/profile credentials, same pending job on repeated
start, encrypted saved token, private profile cleanup, no late save after cancel,
owner-only status and disconnect cancellation. Installer tests verify the private
CLI cache, exact archive entry and checksum rejection. These fixtures prove
those boundaries, not real GitHub account authorization.

See native-device.json and production-device.json for the verification scope.

## Completed owner connection — 2026-10-21

A later real owner sign-in completed successfully. Authenticated production GETs
confirm connected account buchin and 89 readable repositories across two pages,
including 31 private repositories and private buchin/cuan. See
production-connected.json. This used Cuan's saved encrypted connection through
its own repository endpoint, not ambient developer/Fural credentials. No task
was replayed, repository modified, or production inference invoked. The old
pre-release device job returns404; that does not invalidate the saved connection.
Task-level clone/commit/push and continuation remain distinct unverified outcomes.
