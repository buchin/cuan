## Task result, Default and Inbox improvements

- Answers display as Answered after successful execution; implementation review
  stays distinct. Classification belongs to the latest final report and attempt.
  Listings load classifications in one query rather than per-task activity reads.
- Results appear as left-aligned agent messages with name/time. Verification and
  the original request remain in Detail; empty action rows are omitted.
- Project task rows show title, short summary, status and time. Empty filters and
  internal IDs are hidden; agent_dev is rendered as @dev.
- Inbox customer empty state links to Live Chat setup; the Inbox actions menu
  retains access to the same Settings page.
- Default's settings keep its stable name and empty repository. Explicit legacy
  separation retains task/conversation IDs, approval text and old workspaces.
- Missing Git base refs are fetched before any task attempt, without touching
  pending source or task edits. Existing worktree continuation performs no fetch.
- A stopped launch already delivered by its foreground Inbox reply does not
  produce a second identical failure bubble. A later distinct blocker still does.

## Verification

Targeted project/task/server tests pass, including real local Git fetch and
worktree reuse, invalid/stale result classification, settings, and queued launch
reconstruction. The isolated PostgreSQL separation test verifies preview, active
work refusal, optional-module history, transaction rollback and idempotent replay.
Browser checks cover 390/820/1440 px in both themes, answer versus review labels,
accessible Detail, empty filters and both Live Chat shortcuts. No fixture launches
an agent or writes production data.

Completed saved work repaired outside a provider session is delivered to Inbox as a recorded system result, without inventing an attempt or replaying a stopped launch.

Live deployment and acceptance results are recorded separately after activation.
