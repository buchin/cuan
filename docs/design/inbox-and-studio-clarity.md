# Inbox and Studio clarity

Inbox filters describe conversation types: **All / Assistant / Customers** in
English, **Semua / Asisten / Pelanggan** in Indonesian. The unread badge stays in
main navigation; the header and filters do not repeat that number. Conversation
previews use one line. Generated task launch acknowledgements use current task
counts to summarize waiting/working state rather than repeating stale failures.
Ordinary message text remains available in the conversation.

Internal system review inputs (`system` + `cuan_work_review`) are not conversation
content. They remain stored for coordination, but do not appear in previews,
search, message totals, unread counts or the timeline. Real system notifications
and agent replies remain visible, including inability to start a task.

Successful launch responses deliver the task preview without “Work queued/started.
I’ll report progress and the result here.” Existing generated acknowledgement
paragraphs are removed during rendering when there is a task attachment. User
quotes and messages without a task attachment remain intact. New recovery logic
reads saved launch state, not English acknowledgement strings. A failed admission
retains its explanation; review text is not allowed to conceal a stopped launch.

Each linked task attaches to the latest message that can actually display its
preview. A system update in an agent conversation can carry the card. User
references and synthetic review inputs cannot consume its only attachment.
Customer-room task previews remain internal to the owner's Inbox. If no message
can carry a task, its preview is rendered separately. Several tasks referenced by
one eligible message retain their own previews. Cards do not require an empty
speech bubble solely for a launch timestamp.

Studio starts near the top on mobile, with a short gap from heading to composer.
The secondary “Tell Cuan your goal…” sentence is removed. Desktop retains the
centered writing area. Textarea space, draft retention, attachments and sending
behavior are preserved.

Verification:

- `npm test --prefix web` and `npm run build --prefix web`.
- `node docs/evidence/inbox-clarity-check.cjs`: real build at390/1440px,
  English/Indonesian, light/dark, actual failure visibility, no launch paragraph,
  card-only response, Studio placement and draft retention at a short viewport.
- `node docs/evidence/chat-attachment-width-check.cjs` and
  `node docs/evidence/customer-task-card-check.cjs` preserve attachment sizing,
  customer privacy and existing action behavior.
- Server contact/read-model and delegation/launch/recovery tests preserve stored
  history and prove that a preview still refers to an actual execution attempt.
