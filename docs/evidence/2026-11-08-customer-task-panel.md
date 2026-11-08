# Customer conversation task previews

Production release `eb6d876700d7099f1c240a438de39b086dd9370d`.

The duplicate was in InboxView's customer-only branch: existing tasks were
filtered out of agent conversations, while customer conversations retained both
message-bound and unattached active/completed previews. ComposerTasks already
received every linked task for both conversation types.

Removed these stream previews for both types, preserving unsaved task drafts,
approval proposals, customer reply drafts, ordinary speech and assigned-task
navigation. Existing task details and actual terminal navigation remain in
ComposerTasks. Stored messages/tasks were not changed.

TypeScript and the release frontend gate passed. Two legacy source assertions
requiring stream completion cards were updated to check composer ownership and
its detail navigation instead. The actual customer room was rendered locally
with authenticated read-only production data, then again on production at390px
and1440px. Screenshots were visually inspected: no task card below chat, Tasks1
remains visible, expansion shows live progress and Details/Live terminal.
No chat message or task mutation was sent during this inspection.

This verifies the preview placement fix, not completion of the live login task.
