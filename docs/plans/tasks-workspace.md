# Tasks workspace implementation

This work is authorized directly in the Cuan AgentSession, outside Scrum. Scope:
improve Tasks UI, UX, functionality, workflow, and terminal using relevant Fural
patterns. All new product copy is English. Existing user content is preserved.

## Outcomes and acceptance

1. Searchable task list with consistent Backlog/Todo/Doing/Review/Done/Blocked,
   project/priority/assignee/execution context, sorting, URL-backed filters, and
   mobile rows. Opening/closing a task restores the originating list and filters.
2. Read-first detail: wrapped title, Markdown human instructions, agent summary,
   completion/verification notes, editable acceptance checklist, metadata, and
   explicit editing with retained errors and unsaved-change protection. Desktop
   drawer can expand; phones use the full viewport. Delete requires confirmation
   and cannot remove active work.
3. Persist instructions, summary, completion notes, and acceptance criteria as
   distinct fields. Additive schema23 preserves all old data and schema22
   application rollback compatibility. No automatic migrations in production.
4. Start/Retry creates one durable attempt, uses saved task data and the chosen
   registered agent/project, and supports same-request retry without launching
   twice. Open session only attaches. A revision requires concrete feedback,
   preserves human instructions, and starts real work. No ghost revision queue.
5. Execution state remains separate from task workflow. Exit zero goes to Review,
   not Done. Failed/cancelled/interrupted runs retain evidence. Approval requires
   review, completion notes, and checked acceptance criteria. No auto-replay after
   disconnect or restart; cancel/retry are explicit.
6. Sessions show durable past attempts, timestamps, exit/errors, saved output,
   activity and discussion. Terminal has a proper expanding layout, read-only
   default, explicit input mode, reconnect, history selection, follow output,
   copy/download, search, and truthful disconnected/finished states. Browsing a
   task or terminal must not create a shell or agent. Switching tabs/disconnecting
   must not cancel work. Terminal connections must not expose tokens in URLs or
   attach to the wrong run.
7. Targeted service/HTTP/PTY lifecycle checks and fixture browser workflows across
   mobile/tablet/desktop/themes verify behavior, not just screenshots. Verify
   migration on isolated PostgreSQL. Back up and validate production DB/profiles,
   ensure idle work, explicitly migrate, deploy atomically, smoke exact assets
   and real isolated task execution, clean synthetic data, and verify retention.
   Document usage, deployment, smoke, and rollback with release evidence.

## Implementation order

Data and workflow invariants → launch/terminal lifecycle → frontend list/detail
and session view → integrated verification → documentation and production release.
Fural supplies reference interaction patterns; its multi-organization model,
multi-agent infrastructure, roadmap, and bulk operations are outside this change.
