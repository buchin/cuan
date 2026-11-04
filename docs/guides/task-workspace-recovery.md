# Recovering saved task work

Ask Cuan to continue the existing work in its original Inbox. Cuan preserves the
task, project and approved scope. A missing working directory does not mean the
task must be duplicated or its history discarded.

For an authorized continuation with concrete revision feedback and an earlier
attempt, Cuan can build a separate working copy from the retained committed
branch. The original branch and Git metadata remain untouched. When available,
the original index and staged binary patch are saved as private recovery evidence.
The agent inspects this evidence and verifies the original completion criteria.
Unstaged or untracked files that disappeared are not claimed recovered; missing
artifacts must be recreated within the approved scope or reported as blockers.

Opening task details alone does not trigger reconstruction. Damaged retirement
receipts, changed repository/branch identity, interrupted reconstruction evidence
and another deletion of the replacement workspace stop for inspection. They do
not cause an automatic reset. Canceled work requires a fresh instruction to
continue; accepted completed work remains accepted.

A stopped launch is blocked, rather than indefinitely queued, when it is still
the latest launch and there is no queued/running attempt. Capacity backoff remains
queued and retries through the saved launch. Avoid repeatedly sending the same
instruction while its request or attempt remains active.

Implementation evidence: [PM recovery](../evidence/pm-recovery.md).

A terminal answer and a completed task are separate observations. The agent must
save a final task report with evidence; the runner then closes its execution and
Cuan evaluates the original criteria. New reporting sessions retain their scoped
authorization across server deployments using encrypted durable grants. Restart
does not authorize unknown tokens or another task. Older sessions created before
this repair may need a scoped continuation that verifies saved work and reports
it without repeating implementation or external actions.
