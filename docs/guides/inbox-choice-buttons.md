# Conversational choices

When Cuan asks users to choose a name or preference, it returns `prompt_choices`
with concise named options, one primary recommendation, and `allow_other:true`.
Selecting an option sends that answer through the ordinary durable chat path;
it is not executable plan approval. **Others** opens a visible inline answer
composer below the options, using the shared Inbox/terminal writing components.
It focuses within the tap event and preserves the main composer draft. Opening
or typing sends nothing; Send or Enter submits the alternative, Shift+Enter adds
a line, and Escape closes it. Users can also type normally in the main composer.

An alternative can be composed while another delivery is pending; sending still
honors the existing pending-request guard. Named and custom options do not take
attachments from an unsent main draft, and acknowledging a choice preserves
those draft attachments.

Internal agent choices in customer conversations remain internal and address
the agent rather than sending the preference to a customer. Choice buttons share
the task-card action surfaces and 44px targets.

An explicit closing request to choose with no structured action is rejected
before application actions. The existing single, format-only repair can supply
the already stated choices without rereading files or investigating again.
Invalid options are not promoted to executable approval. Read-only analysis
recovery still cannot emit actions. Native transport recovery preserves valid
choice payloads, including JSON-string payloads.

Historical messages are not rewritten or replayed. Replies already saved as
plain text retain their original content; these gates apply to new responses.

Verification:

```sh
go test ./internal/server -run 'TestCoordinatorExplicitChoice|TestInboxChoiceFormatRepair|TestInboxFormatRepairRetainsCandidate'
cd web && npm run build
```

From the repository root,
`CUAN_CHECK_PROMPT_CHOICES=1 node docs/evidence/task-attention-check.cjs` checks
390px/1440px light/dark screenshots, named-option dispatch, internal delivery,
draft preservation and no approval metadata or send from Others.
