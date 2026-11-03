# Settings and contacts verification

2026-11-03. UI scope: General, Agent, Memory, Schedules, Appearance, Notifications,
Integrations (and service details), Apps & Skills, Team, Profile, contact list and
detail. Removed duplicated content titles, large back buttons, redundant role
badges and duplicate general links. General delegates to one shared component.

Frontend tests are updated to inspect the actual shared layout/control components
and canonical routes rather than requiring duplicated markup. The 424 frontend
checks and TypeScript/build passed before final release. Targeted account and
provider tests cover nested identity, exact UserInfo origin/auth, rejected
redirect/unverified identity, no ambient-profile fallback, owner-protected and
in-use disconnect, retained keys during ordinary edits, explicit key removal,
and clearing a preferred model to Auto.

Real Chromium fixtures exercised both 390px and 1440px: editing account name and
model; requiring confirmation before DELETE; an in-use 409 leaves the account;
retry removes it; contact detail opens; and contextual Live Chat settings return
to Inbox. The fixture provider list included a host compatibility account and
verified that it never appears as a user-connected account. No production login,
account edit, disconnect, email sending, or chat message was performed.

Rendered all ten Settings pages with real authenticated production reads and
local built frontend; API writes were stubbed. Selected fixture screenshots and
logs are in `/home/dev/reports/cuan-settings-flows`; full page screenshots are in
`/home/dev/reports/cuan-settings-unified`. These are UI/fixture evidence, not a
claim that a fresh provider authorization completed end to end.

Follow-up: project Goal was saved through the official guidance CLI and read
back. The existing create-task service snapshots Goal and Instructions together;
its preservation test passed. The older database-backed MCP draft creation path
now also snapshots the project's guidance rather than bypassing it. A real,
isolated PostgreSQL fixture verified initial guidance, immutable historical task
context, and updated guidance in a subsequent MCP draft. Evidence:
`/home/dev/reports/cuan-project-guidance-sql.log`; fixture cluster stopped afterward.

AgentRunner v0.2.0 was published after owner approval. Cuan imports its process
cleanup primitives through adapters. Targeted consumer tests (including actual
process descendants and lifecycle/account regressions) passed. No UI or private
account execution adapter was replaced during this dependency adoption.
