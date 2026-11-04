# Task list performance

Tasks and the default Project activity list request one page of25 tasks. Search,
project/status/priority filters and ordering run on the server. Tab counts cover
the full project scope, including tasks outside the current page. Search input
is debounced; obsolete requests are canceled and cannot replace a newer result.

The page owns refresh for its rows: no per-card detail polling or hidden duplicate
mobile/desktop task tree. Visible active lists refresh every5seconds; idle lists
back off to20seconds. Hidden tabs stop fetching and resume on focus. Failed or
stalled reads expose retry, without retrying writes automatically. Queue numbers
are not inferred from the partial page. Open a task to inspect its full details.

Studio, Inbox, Projects overview, Contacts and Settings no longer download the
entire task history at startup. Shared simultaneous GETs coalesce within the same
authentication identity; there is no persistent response cache. Agent directory
refresh runs separately from unread/contact polling. Chat/terminal streams retain
their own lifecycle. Project extension views retain their existing full-project
input contract and request it only after that extension is selected.

## API

`GET /api/v1/tasks?view=board&page=1&page_size=25` returns:

- `tasks`: compact task previews, with latest attempt and progress;
- `total`: number matching all current filters;
- `scope_total` and `counts`: full project-scope tab counts;
- `page` and `page_size`: actual server page, clamped after deletion/filter changes.

Optional filters: `project_id` (`none` means unassigned), `search`, `status`,
`priority`, `sort`, `exclude_terminals=true`. Page size defaults to25, maximum100.
Sort values are whitelisted: updated, updated-asc, created, title/title-desc,
task/task-desc, id/id-desc, status/status-desc, priority/priority-desc. Stable ID
ordering breaks ties. `active`/`dibatalkan_review` preserve board groupings;
`work_active`/`attention`/`completed` use Project activity groupings, including
answered reviews and canceled attempts. Accepted Done is preserved.

Preview payloads omit project guidance/environment snapshots, full checklists and
large diff/test reports. The task detail endpoint retains the full record. Legacy
`GET /api/v1/tasks` keeps its existing full-list contract for explicit CLI/export
and extension consumers. Standard memory/PostgreSQL stores fetch latest attempts
in one batch; project associations are restricted to returned task IDs.

## Verification

- `go test ./internal/task -run 'TestTaskPageMemoryContract|TestTaskListBatchesLatestAttempts'`
- `go test ./internal/server -run TestTaskBoardAPIIsBoundedAndKeepsLegacyList`
- `TestLaunchPostgresContract` on a disposable `cuan_launch_queue_*` database
  covers paging, counts, status groups and a7000-task dataset.
- `npm test --prefix web` and `npm run build --prefix web`.
- `node docs/evidence/task-page-performance-check.cjs` runs the actual frontend
  build with isolated fixture APIs at390/1440px in light/dark themes, including
  next page, remote search, stale-response rejection and request budgets.
- `node docs/evidence/task-row-menu-check.cjs` verifies row action behavior.

Private production measurements compare the same six routes with fresh browser
contexts. Report completed JSON bytes and initiated requests separately; live SSE
streams are excluded. These measurements are not a Lighthouse/Core Web Vitals
score. The local7000-task run returned25 rows in about80ms and21KB JSON; production
latency must be measured separately.

The current-task MCP reporting tools and task list performance work are separate
modules. No new dependency or public package is required for paging. Shared
components (`TaskPagePager`, `useTaskPage`) serve Tasks and Project activity;
extracting a public library should follow a stable, proven contract rather than
copying application-specific authorization or orchestration into a generic SDK.

The UI requests `GET /api/v1/chat/contacts?view=list` for a single canonical
`contacts` array. Legacy responses retain the identical `conversations` alias.
No preview text is truncated; names, last-message search, unread/task counts and
message history are unchanged. This removes the duplicated array from every
shared contact refresh without breaking existing CLI consumers.
