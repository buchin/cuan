## Optional decisions in Inbox

Cuan defaults to English. Settings can explicitly select another supported
language or Auto; saved preferences are preserved.

A separate plan is optional. A clear instruction that already authorizes work
can start it directly. Questions stay conversational; ambiguity calls for a
focused question. The actions below appear only when a decision is needed.

Describe your goal in Studio or an agent conversation. When the agent proposes
work, read the scope and limits displayed below its reply, then choose **Approve
and start**. The button changes to **Approved** when the approval is accepted.
You can keep an unsent draft in the composer while approving a plan.

**Change plan** focuses the composer and appends a reference to the proposal,
preserving your draft. Add the changes you want and send when ready; clicking
that button alone sends nothing and launches no work.

**Not now** saves a deferral in the conversation without calling the provider or
starting work. Its state survives reload; you can still approve that proposal
later. Repeated clicks reuse the same saved deferral. Deferral is not approval
and cannot authorize delegation. Once approved, use work controls to stop active
execution; Not now does not cancel or pause an approved task.

An unambiguous short reply such as “go ahead”, “approve”, “Gas” or “setuju” can
also approve the immediately preceding structured plan. An intervening message,
a question or new attachments prevent that automatic association. For changes,
ask the agent to propose an updated plan before approving it.

The application saves a snapshot of the displayed scope, then starts that exact
scope through normal task delegation. It does not ask the model to rewrite the
plan at approval time. Repeated clicks and delivery retries reuse one approval,
one task and one launch identity. Progress and the verified result return to the
same conversation. Scoped corrections continue the original task.

## API and agent contract

An agent proposal ends with one `cuan-approval` JSON fence containing `title`,
`description` (scope, limits and acceptance criteria, at most 6000 characters),
and exactly one of `project_id` or `project_name`. The UI renders the exact
`description` as ordinary text next to the action. Title-only legacy markers
remain notifications and have no execution button.

The authenticated conversation message endpoint accepts `approve_message_id`,
selecting a completed agent proposal in the same owner/agent/conversation scope.
The optional `plan_action` is `approve` (default) or `defer`; unknown actions are rejected. The client cannot submit a replacement plan. The server assigns a stable request
ID to that proposal and saves its scope in the private user-message envelope.
Raw envelope injection is rejected. Ordinary history shows the readable approval
text, never the envelope. No database migration is required.

The dispatcher converts a saved approval snapshot directly into the canonical
delegation command, preserving existing connection checks, quota routing,
workspace preparation, process ownership, task reporting and bounded corrections.
A model-supplied delegation using this approval but different initial scope is
rejected. Invalid/foreign/user-authored proposal references are rejected before
any approval is saved or execution begins.

## Current limits

Direct execution instructions without a preceding structured proposal retain the
existing natural-language delegation path. The model still interprets that intent;
this feature does not prove that every arbitrary user message authorizes work.
Legacy proposals without a structured scope require a new concrete proposal for
the button flow. The application records approval and execution evidence; it
cannot guarantee that a model implements every scope constraint correctly.

Production checks must not click approval or replay user messages. Use local
fake-provider tests for execution, and read-only production UI/API smoke checks.
