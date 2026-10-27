# Conversations, execution, and recovery

## From message to result

Studio creates a durable Inbox conversation. The coordinator uses relevant
conversation/project context to answer questions or delegate clear work to a
project task. A task runs in its own session, reports progress back to Inbox,
and supplies verification and a final result. The coordinator can propose a
follow-up when useful. Completion does not inherently require a human status click.

Execution authorization comes from the user's instruction or an accepted exact
scope proposal. There is no mandatory planning ceremony. A conversation can have
multiple exchanges without any task, and execution records stay linked to their
source conversation. See [optional proposals](plan-approval.md).

## Activity and diagnostics

Messages expose current activity and **Detail** diagnostics. Provider process
exit alone does not establish a successful response: the result must contain a
valid reply. Task completion should carry actual outcome/verification evidence.
Provider errors, external access, quota, and uncertain outcomes remain visible.

## Capacity and quota

Connected accounts support bounded concurrent sessions (default four slots per
account); token refresh stays exclusive. Capacity and quota are different states.
Allowed account/model routes may be selected when available, subject to routing
policy. A second account does not automatically grant access to a project.
See [account session evidence](../evidence/account-sessions/README.md).

An authorized task whose route is temporarily unavailable enters a durable launch
queue. It retries the same intent after 30 seconds, revalidates authorization and
scope, and waits up to 24 hours. Restarting the server preserves waiting work.
**Cancel waiting work** stops the queued intent. See [launch queue](launch-queue.md).

## Safe recovery

Known no-effect reply failures can receive one bounded automatic recovery using
the original message and attachments. Unknown side effects must be reconciled
before replay; **Check status** is appropriate when a retry cannot be established
as safe. Manual supported recovery reuses the saved request rather than inventing
a different instruction. See [reply recovery evidence](../evidence/automatic-recovery/README.md).

Execution ownership is fenced at the operating-system process level. Recovery
context includes bounded prior progress and checklist evidence. This supports
continuation from actual progress; it does not promise native provider-session
resume or exactly-once effects in an external service. Missing credentials or
unavailable external systems can still require intervention.
