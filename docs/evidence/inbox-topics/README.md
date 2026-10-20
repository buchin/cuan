## Topics, project work and Default

Independent agent topics use durable room UUIDs and existing room metadata. New message chooses a registered agent and creates a separate topic; first user message names it. Opening a topic never creates work/PTY. Legacy aliases/history and customer chats remain unchanged. New-message recipients deduplicate agents across topics.

Work cards attach to the agent's reply with progress/result links and a return path to the source conversation. Tasks moved from main navigation to Account → Work monitor. Project work uses /projects/PROJECT_ID/tasks with a320px work sidebar and detail/session area; mobile/tablet use list/detail. Existing task URLs remain valid. Scoped selection blocks tasks from another project.

Default is initialized idempotently at startup; new task creation without a project uses Default. Existing configuration/archive state and historical unassigned records are preserved. Legacy creation-request replays remain idempotent. Memory project storage now permits distinct blank projects while rejecting duplicate IDs/repositories.

Targeted Go tests cover topic isolation and naming, owner authentication, idempotent creation, ordinary chat without work, legacy alias preservation, full delegation/report/review/revision in both legacy and new topics, fresh Default initialization/configuration preservation, default assignment and legacy request replay. Existing targeted task/session/workspace/project tests passed after fixing the blank-project memory-store conflict exposed by Default.

TypeScript/Vite and Chromium fixtures passed eight topic viewport/theme cases, recipient retry/dedupe, customer navigation, agent-bubble progress/terminal/back-to-conversation, advanced monitoring, eight project workspace cases and cross-project rejection. Providers are deterministic fixtures, not live paid calls.

## Production release

Deployed code `f44bb7adc4cfb7ba5be21aadb50243ea1305985b` to https://cuan.dojo.cc through the native atomic release. Readiness healthy; schema23 unchanged/no migration. Existing Default configuration was preserved.

Authenticated production checks passed: UI new-topic creation, second independent topic, same-ID replay, registered-agent/topic metadata, Default assignment, project work/session paths, responsive project sidebar, Back navigation, advanced Work monitor and exact assets. Eight viewport/theme cases passed. Two empty synthetic topics and one task/finished attempt were cleaned without creating user messages, rooms for customers, provider executions or terminal runs in the live application.

The compiled isolated server tests also passed on the production host: new topic context isolation, Default task routing and full legacy/topic delegation, real PTY reporting, coordinator result review, concrete corrections and retry bounds. Those tests used memory stores and fake providers, not the production database or a paid provider. Semantic model adherence in real use remains untested.

Retention:35 existing tables and706 account-profile files unchanged (login sessions excluded). Validated private backup retained; persistent project checkout fast-forwarded to deployed code. Production screenshots show synthetic topic/work content. See `production-browser.json`.
