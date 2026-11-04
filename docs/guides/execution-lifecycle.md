# Task execution lifecycle

A task is the durable user request. An attempt is one execution of that request;
ending an attempt does not prove that the task is complete.

## Deadline and ownership

Runner and reaper share the task execution timeout policy: 25, 35, or 45 minutes
according to scope, and 5 minutes for read-only execution. Existing timeout
environment overrides remain supported. Claiming an attempt persists its
`execution_deadline`; the reaper uses that deadline with a 30-second grace.
Legacy attempts without a saved deadline use the same shared policy.

Completion and task-agent reports lock and update the task, attempt, retry, and
activities atomically. They reject stale/superseded attempts, foreign runners,
expired leases, canceled attempts, and changed task versions. A duplicate final
report is ignored. Releasing an old assignment cannot clear a newer claim.
Expired execution becomes blocked; it cannot turn into a review result because
a delayed process reports success. Terminal reconciliation honors task deadlines.

## Evidence and continuation

A result requires a fresh summary, a completion note, and explicit evidence for
every original acceptance criterion. Agents cannot rewrite the approved checklist
through reports. Reported evidence is an observation, not a replacement for
independent runner verification.

The runner exposes its task-report command through `CUAN_TASK_EXECUTABLE`:

```sh
"$CUAN_TASK_EXECUTABLE" task report --state review \
  --summary "What changed" --complete-note "What was verified" \
  --dod-item "Original criterion, unchanged" \
  --dod-evidence "Observed check and result"
```

Use `--report-file FILE` for structured reports, including criterion IDs.
Automatic continuation is bounded and requires a fixable gap and confirmed
evidence. It keeps the same task and branch. Prior checkpoint reports and review
feedback are included in recovery context; feedback is not appended to the
approved checklist. Hitting the retry limit leaves a blocked task.

## User-facing task detail

Clicking a task opens its overview at `/tasks/:id`. It shows the current next
step, work summary, result and verification, original instructions, and criteria.
Incomplete historical Review tasks cannot be approved in this view until their
evidence is complete. Original instructions and task settings expand on demand.
Edit preserves the original request as a separate field. Technical sessions live
at `/tasks/:id/sessions/:attemptId` and are accessible from **Detail teknis**;
`latest` resolves to the current attempt. Explicit project terminal links retain
`terminal=open`. Pinning a terminal affects only the mounted task session.

This adapts Fural's separation of task detail and AgentSession. Cuan keeps one
public assistant and automatically chooses it when starting/retrying work; users
do not select the private AI accounts behind it.

## Migration, deployment, rollback

Migration 000032 adds nullable `task_attempts.execution_deadline` only. Existing
history is retained; no old tasks are automatically replayed. The native atomic
release runs migrations before switching the live release. The previous binary
can run with the added nullable column. Prefer rolling back the release while
keeping the additive schema; the down migration drops only the new column and
must be run only after newer workers have stopped.

Use `fural-agent repos ship RPcd484fc4229ae192d17a0f01` for a release. Verify health
reports the released revision, the runner is active, and migration 32 is present.
Do not repair historical failures by relabeling them complete.

The assistant name is editable in **Settings → Agent**. `@cuan` is only the
default. Task guidance, session identity, and retry flows read the saved public
name while routing through the stable internal public-assistant ID. Historical
private assignees are not reused as the target of a new retry or follow-up.

## Recovery sweep selection

Local startup recovery, local lease recovery, zombie attempt reconciliation and
host deadline discovery select task IDs from existing status indexes before
loading task records. Candidates are board status `doing`, any running attempt,
or a queued `local_terminal` attempt. This includes stale board states without
loading settled history or enriched list/read models. The latest canonical
attempt is still checked before any reconciliation; queue age is not execution
time, and detached daemon workers remain governed by their leases/deadlines.
An empty candidate set never becomes an unrestricted ID filter. Custom legacy
stores without the selection interface retain their previous fallback behavior.

The isolated PostgreSQL regression seeds 7,000 accepted tasks plus active/stale
cases and selects only three recovery candidates; memory instrumentation verifies
that the sweeps do not issue thousands of latest-attempt reads. Existing reaper
checks still require failed attempts to leave a blocked task, stopped launch and
available project lane. This optimization does not remove the separate Inbox
result-delivery history scan; that remains a distinct performance concern.
