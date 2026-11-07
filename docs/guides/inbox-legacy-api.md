# Legacy Inbox conversation adapters

The application uses Studio and the durable Inbox conversation API. Older callers
of `POST /api/v1/inbox` now enter that same coordinator flow rather than receiving
a generated placeholder plan or a hardcoded worker choice.

Send `message`, an optional `title` and `project_id`, and a UUID `request_id`.
The response retains HTTP 201 with `thread` and `messages`, and adds
`status: "accepted"` and the request ID. Acceptance records the operator message;
it does not mean the assistant has replied or any work has completed. Read the
conversation normally for the asynchronous reply. A coordinator must be available;
errors never manufacture a plan or task.

For follow-ups in these agent conversations, send `content` and a UUID
`request_id` to `POST /api/v1/inbox/{id}/messages`. The HTTP 201 response retains
`message` and adds the durable `request` receipt. Operator messages cannot use this
adapter to impersonate an assistant. Existing historical non-agent rooms retain
their original message-storage behavior; this adapter does not rewrite history.

Keep the same request ID when a response is lost or uncertain. Repeating it with
the same content reconciles the saved request; different content is rejected with
409. Older callers that omit the ID receive a generated one, but cannot safely
replay an uncertain request without recovering that identity. Callers should
supply their own UUID before sending. A supplied conversation title applies only
when the conversation is first created.

The adapter does not approve a proposal, select an AI account, create execution
outside the normal task gates, or accept a worker's result. Project selection,
authorization, cancellation, coordinator recovery and independent code verification
continue through the canonical workflow.
