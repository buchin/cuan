## Topics, project work and Default

Independent agent topics use durable room UUIDs and existing room metadata. New message chooses a registered agent and creates a separate topic; first user message names it. Opening a topic never creates work/PTY. Legacy aliases/history and customer chats remain unchanged. New-message recipients deduplicate agents across topics.

Work cards attach to the agent's reply with progress/result links and a return path to the source conversation. Tasks moved from main navigation to Account → Work monitor. Project work uses /projects/PROJECT_ID/tasks with a320px work sidebar and detail/session area; mobile/tablet use list/detail. Existing task URLs remain valid. Scoped selection blocks tasks from another project.

Default is initialized idempotently at startup; new task creation without a project uses Default. Existing configuration/archive state and historical unassigned records are preserved. Legacy creation-request replays remain idempotent. Memory project storage now permits distinct blank projects while rejecting duplicate IDs/repositories.

Targeted Go tests cover topic isolation and naming, owner authentication, idempotent creation, ordinary chat without work, legacy alias preservation, full delegation/report/review/revision in both legacy and new topics, fresh Default initialization/configuration preservation, default assignment and legacy request replay. Existing targeted task/session/workspace/project tests passed after fixing the blank-project memory-store conflict exposed by Default.

TypeScript/Vite and Chromium fixtures passed eight topic viewport/theme cases, recipient retry/dedupe, customer navigation, agent-bubble progress/terminal/back-to-conversation, advanced monitoring, eight project workspace cases and cross-project rejection. Providers are deterministic fixtures, not live paid calls.

Production verification follows release.
