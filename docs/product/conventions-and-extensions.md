## Product direction

Cuan should work as an always-on project manager for nondevelopers. Prefer existing repository/runtime/deployment conventions and let agents handle routine configuration. Keep technical configuration inspectable without making it a prerequisite for ordinary use. Questions, networking preparation and relationship notes need no project/task; use Default for small one-off execution, and include a distinct named project in the approved plan for an ongoing goal.

These are design decisions and proposals, not a claim that the capabilities below are implemented.

## Project access through chat

Accept legitimately supplied access needed for approved work, such as the VPS used to deploy Cuan. Do not require the user to operate a separate secret-manager product. Reuse existing secure environment/configuration facilities where suitable. Bind access to the discussed project; ask a simple project/target clarification when ambiguous. Reuse it for later authorized work and support replacing outdated access.

The implementation must persist protected values separately from ordinary chat, sanitize persisted messages before their use in previews/search/notifications/logs, and inject access into execution only as needed. A confirmation bubble can say “VPS access saved for Cuan” without echoing values. Cosmetic frontend masking or committing credentials into a repository `.env` is insufficient. Automatic capture, protected storage, masking and execution injection remain future work; existing search currently treats saved messages/notes as ordinary text.

## Extensibility

Keep a useful self-contained default. Optional extensions should share the core data model rather than fork Contacts, Inbox or Tasks.

Start with explicit extension boundaries:

- Search providers: built-in search remains available; an optional provider such as Algolia can supply search later.
- Views: Kanban can present the same tasks/statuses used elsewhere.
- Channel connectors: WhatsApp/Telegram can map external identities and messages into existing Contacts/Inbox.

Recommend validating the contracts with one real integration before building a marketplace or loading arbitrary plugin code. Extensions need declared capabilities, compatible API versions, secret references, health/error isolation and clean disable/uninstall behavior. Connector failure must not make the whole Inbox unavailable. For the user, expose outcomes such as “Connect Telegram” or “Enable Kanban”; the agent can assist with setup, while external authorization/consent remains explicit.

No plugin SDK, marketplace, third-party search provider, Kanban view or WhatsApp/Telegram connector was implemented in the universal-search release.

## Terminology and extension points

“Hook” is the umbrella term for an extension point. A filter transforms an input value and returns a value. A validator accepts/rejects proposed data. An event announces a fact and listeners react without changing the original committed operation. User-facing configuration can use “Automations”; these implementation terms belong in developer docs.

Potential events include task.created/completed/blocked, contact.created/updated/deleted, contact.memory.saved/deleted, inbox.message.received/replied, approval.requested/approved and deployment.succeeded/failed. Emit after successful persistence. Production delivery needs bounded retries/idempotency and eventually a durable outbox. Event payloads should contain minimal resource identity/project context rather than credentials. A failing extension must not undo completed work.

Markdown extensions are parser/renderer plugins, not events. Register syntax or fenced-block renderers, preserve the original text, and degrade to readable text/code when unsupported or disabled. Examples: footnotes, math, D2 and domain-specific cards. Message text must never execute arbitrary HTML/JavaScript. Trusted installed frontend code is different from untrusted message content.
