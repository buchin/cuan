# Studio

Studio is the first screen after sign-in. Describe the outcome you want, then select **Get it done**. You do not need to choose an agent, project, runner, or task status.

Cuan opens a new conversation in Inbox with an available connected coordinator. Each submission starts an independent topic. The coordinator uses relevant conversation and project context, asks for clarification when necessary, and presents proposed work for approval. An approved request can use an existing project or create a named local project. A greeting or exploratory discussion does not automatically create or run a task.

**New message** in Inbox, including its mobile compose button, opens Studio. Existing conversations, including customer live chat, still open directly in Inbox. Customer contact management remains in Contacts.

Your draft stays in this browser until accepted. If delivery fails, retry the same goal: the saved request ID prevents duplicate acceptance. If no agent is connected, Studio explains how to connect one in Settings → Agents. A changed goal uses a new request ID.

Once work starts, use the work card in the conversation to inspect progress or the result. Project work remains available under `/projects/:projectId/tasks`. The coordinator reports verified results back to the conversation. Deployment still requires authorization.

## Operator notes

`POST /api/v1/studio/goals` requires owner authentication, a UUID `request_id`, and a nonempty `goal` of at most 6000 characters. It returns `202` with `conversation_id` and `status`. Reusing an ID with different content returns a conflict. The existing durable Inbox acceptance and coordinator queue handle processing.

Studio chooses an enabled agent with an owned profile, preferring coordinator roles. Eligibility does not guarantee provider connectivity; connection/runtime failures remain visible through Inbox response details. No provider availability probe is billed during selection.

Named projects use application-generated IDs and controlled local workspace paths. Existing projects and Default are preserved. Project creation follows authenticated conversation approval checks. No schema migration is needed for this release; deploy and rollback follow the existing native release procedure.

## Visual identity

Studio uses Cuan’s kepeng mark in navigation. The navigation label remains **Studio**, including on mobile. The mark follows the current theme; the navigation mark inherits the active item color. The greeting has no repeated logo. The **Get it done** button sits12px below the goal input. On mobile it fills the input width, with help text beneath and a compact top layout. On desktop the button is right aligned and help text sits on the left. There is no textarea resize handle. Navigation marks are decorative for assistive technology because the button already has a readable label.
