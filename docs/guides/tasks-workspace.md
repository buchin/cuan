# Tasks workspace

Open a task to enter its latest session. A task without a session shows the
original request and an explicit **Start** action with an agent selector.
Opening or refreshing a task never starts execution.

While running, the native interactive terminal fills the workspace. Once a
session ends, its original request and recorded result become the main view.
Continue with the reply field; implementation results offer **Accept** and
**Request changes**. Accept requires verification evidence and the existing
checklist gates. Request changes focuses the reply field and launches one new
attempt with concrete feedback.

**Detail** contains session history, agent-reported checklist, evidence and a
link to edit task metadata. Open the saved terminal from Detail when needed;
completed-result pages do not load a terminal automatically. Back returns to the
originating task/project list and filters. Closing metadata returns to the
selected session.

Agents may classify a final response using **--result-kind answer** for
conversation without file changes, or **--result-kind implementation** for work
(the default for old or unspecified reports). Answer presentation has a reply
field and no required Accept action. Classification is scoped to the exact
attempt/report and controls presentation; it never automatically approves Done
or changes the implementation acceptance gates. Internally, the final report
still reconciles through Review. Unknown/legacy results retain implementation
review behavior.

Earlier sessions show their own final report snapshot and cannot accept or
continue a newer attempt. Missing sessions show an error rather than attaching
another process.

## Search and create

Open **Tasks** to search, sort, or filter by status, project, priority, and agent.
Filters are part of the URL and remain when a task is closed. **New task** opens a simple prompt composer. Select a Project when repository
context is needed. The server derives the initial title and priority from the prompt; the agent can refine them. The first line usually supplies the initial title and the complete
prompt is saved as the human instructions. Choose **No agent** to save to Backlog without execution,
or select an agent and press **Create** to open its session immediately.
The composer remembers your Project and Agent; a verified available agent is
selected initially. Project-specific pages use their current project.
While the server starts execution, the session page shows **Starting session…**,
then opens the confirmed attempt. Task details is not an intermediate step.
Viewing a task never starts an agent.

## Prepare and run

1. Describe the work in the prompt; title, priority, and acceptance criteria do
   not need separate input during creation. You can refine them in task detail.
   Select a project when
   repository context is required; use **No project** for standalone work.
2. Open the task for its session/request/result. **Detail → Open task details**
   exposes instructions, metadata, checklist and notes; choose **Edit** to change them. Failed saves retain the draft. A stale
   draft is rejected rather than overwriting newer work; copy the draft, discard
   it, and reopen Edit to apply the current version.
3. Choose **Start**, select a configured agent, and confirm **Start session**
   for a saved task. New tasks with an agent go directly from **Create** to the session.
   Configure and verify agents in **Settings → Agents** first. The saved title,
   instructions, criteria, existing notes, and additional feedback form the run
   context. Human instructions remain a separate field.
4. If the launch response is lost, choose **Check launch request**. It recovers
   the same request, including after a reload; it does not start another run.
   On the initial session page this action is **Check session**. Reloading a
   pending start never replays it automatically. You can leave the page while
   starting; completion will not pull you back.

## Review and revisions

Execution and workflow are separate. New agent sessions enter **Review** only
after a successful exit or an intentional close after a validated final report,
with a structured report containing summary and verification evidence. A successful exit without a complete report leaves the task **Blocked**.
Neither an agent report nor an exit code approves Done. Failed, interrupted, and canceled attempts retain their output and leave
work blocked for an explicit retry. Editing a status does not launch an agent.

Inspect the result, check every acceptance criterion, and enter verification
evidence before **Approve result**. **Request changes** requires concrete feedback
and starts a new session while retaining previous attempts and the human task.
Criteria are reset for each new run and must be verified again. **Reopen task**
returns approved work to Todo. Summary and completion notes are written by the
bound agent through its task tool, or by the user; an exit code alone does not fabricate a result summary.

## Sessions and terminal

**Detail** lists durable attempts, status, timestamps, exit code, and errors.
Opening a session navigates to `/tasks/{taskId}/sessions/{attemptId}`. Each session
has its own shareable/reloadable page with the terminal and that attempt's recorded
agent reports. The session selector inside Detail changes the URL; missing sessions show an
error rather than attaching another run. The terminal fills the session workspace. **Detail** opens a side panel for
agent reports, timestamps and session history. Sessions hide the duplicate global header; the desktop sidebar defaults to
icons and can be expanded without changing the normal navigation preference.
Mobile sessions hide bottom navigation; the back arrow returns to the originating list. Task metadata is a secondary drawer and returns to the session when closed. Select earlier sessions to read saved output. **Activity** shows workflow events
and comments. The terminal starts in read-only mode. **Enable input** explicitly
enables keyboard input; reconnect returns to read-only. Reconnect attaches the
same live process. Closing a drawer, switching tabs, or disconnecting does not
cancel execution. **Cancel session** requires confirmation and retains evidence;
it does not undo files the agent already changed.

Expand **Terminal tools** for Follow, search with Next/Previous, Copy output,
Download output and reconnect. The **⋯** menu contains these secondary tools and closes on Escape or outside
click. The keyboard button enables/disables input. After enabling input, type
directly into the terminal’s own prompt. There is no second message composer
while the agent is live. Input and keyboard control
attach to the same live interactive CLI, never a replacement shell. Once a
session has ended, follow-up instructions start a new attempt with the saved
task context. Older attempts stay read-only.

Task agents run in interactive mode: Antigravity uses
`--prompt-interactive` rather than `-p`, Claude uses its interactive positional
prompt, and Codex uses its interactive CLI with `--full-auto`. Antigravity task
runs auto-approve tool permissions, matching the existing task execution
adapter; Inbox retains its separate restricted launcher. These processes run
as the configured runtime user with access to the selected workspace.
Task credentials remain scoped to the active task and attempt. Output is
bounded by server retention and the terminal buffer; exports contain the currently
loaded output. If a process is no longer live, the UI shows retained output rather
than creating a replacement shell. Interactive task sessions have an eight-hour execution limit. Final
`review` or `blocked` reports close the matching interactive session after a
two-second output drain. New terminal input or a later progress checkpoint cancels an older pending
close. Actual signal exit codes are retained for audit; an intentional report
close is distinguished from provider failure or user cancellation. Human
approval is still required for Done. A process interrupted by server restart is never replayed
automatically. Task deletion requires confirmation and refuses active attempts.

## API and storage

Task endpoints remain under `/api/v1/tasks`. Create accepts a UUID `request_id`,
`title`, `instructions`, `project_ids`, `assigned_agent`, `priority`, initial
`status` (`backlog` or `todo`), and `dod_checklist`. Update adds `summary`,
`complete_note`, and `dod_checklist` (`[{"text":"Verify output","completed":false}]`).
Send `expected_updated_at` from the loaded task to reject stale changes.

`POST /api/v1/tasks/{id}/run-agent` accepts a UUID `request_id`, `agent_id`, and
optional `revision` (required from Review). Reusing a key recovers its recorded
attempt; reusing it with a different payload returns409. Failed requests are not
automatically retried with a fresh key. Terminal WebSocket authentication uses
subprotocols, not query tokens. Connections attach to the matching `session_id`;
foreign origins and mismatched sessions are rejected.

Schema23 adds task summary, completion notes, checklist, create-request hash,
and attempt-request hash with defaults. It does not rewrite existing rows.

## Native production release and rollback

1. Verify the targeted task/server/terminal checks, browser fixture flows, and
   PostgreSQL migration22→23 in a dedicated database.
2. Ensure no active inbox requests or task attempts. Create and validate a private
   database dump and account-profile backup. Preserve retained-table snapshots.
3. Build `go build -o cuan-migrate ./cmd/migrate`. With the service database URL
   supplied privately in the environment, run `cuan-migrate --direction up`.
   Production keeps `CUAN_AUTO_MIGRATE=false`. Confirm schema count23 and the
   existing application remains healthy before activation.
4. Commit and push the approved branch. Run
   `scripts/fural-deploy.sh --branch sprint26-agents` for checksummed versioned
   deployment, lock, atomic switch, health/readiness checks, and health rollback.
5. Verify exact assets, task create/edit/reload/approval/delete, retained data and
   profiles. Run deterministic PTY checks in isolation without calling a provider.
   Remove only exact synthetic fixture records.
6. For application rollback, keep additive schema23 and run the native activator
   on the previous release with `--schema-count 23`. Do not run the down migrator:
   it can remove application data and is not the application rollback procedure.

## List controls and quick creation

Desktop uses compact table rows with inline status and assignee controls. Status
shortcuts cannot approve Done or start execution. Active sessions lock these
changes; stale edits are rejected. Tablet and mobile use task rows with visible
updated times. Phones have a status selector and a Sort menu so search retains
space. Project, priority, and assignee filter chips can be removed individually.

The new-task composer has one instructions field, a Project selector, and an
optional Agent selector. It has no mandatory metadata form. Drafts and the create
request ID survive reload in this browser tab. Closing a nonempty composer asks
before discarding it. A failed create keeps the same request ID. If creation
succeeds but launch fails, the persisted task still opens; uncertain launches
expose **Check launch request** instead of inviting a duplicate creation/run.
The action remains **Create**, with the selected agent controlling whether a session starts. An existing
CLI agent must be connected/configured before execution; creating a task is still
possible with **No agent** when agent listing is unavailable.

## Bound agent reporting

Create with an agent saves the task and opens its Starting session immediately;
the UI opens the confirmed attempt once the launch returns. Navigating
away does not cancel work or pull the user back. Failed/uncertain launches leave
the task available, with explicit same-request recovery. The browser does not
choose title/priority defaults; prompt-only API creation derives them on the server
without a remote inference call.

The server executable includes its agent tool, so no extra SDK or CLI install is
required. Inside a bound session:

```sh
"$CUAN_TASK_EXECUTABLE" task --help
"$CUAN_TASK_EXECUTABLE" task get
"$CUAN_TASK_EXECUTABLE" task report --summary "Plan and current progress" --progress "Inspecting the project"
"$CUAN_TASK_EXECUTABLE" task report --state review --summary "Actual changes" --complete-note "Actual verification, outcomes and limitations"
```

Use `--state blocked` with a summary and concrete blocker instead of claiming a
review-ready result. `--report-file path.json` accepts a structured report with
optional title, priority, summary, complete_note, progress, state, dod_checklist,
commit_sha and test_summary. The agent defines/refines criteria and records actual
verification; the human still approves Done. Final result snapshots remain in the
attempt's activity history even when a later session replaces the task's current
summary. Previous notes are included in the next session's context, then cleared
for the new attempt so earlier evidence cannot qualify new work.

The private capability file supplies a short-lived credential for just the current
task and active attempt. It is never an owner token and is not placed in URLs,
prompts or command arguments. It cannot change human instructions, assignment,
project, process state or Done. Access is revoked on process exit, expires after
eight hours and five minutes, and is invalid after server restart; temporary files are removed
on normal termination. The CLI checks private file permissions and uses only the
loopback API. Existing historical sessions remain readable and are not rewritten.
No database migration is required for this reporting contract; it uses existing
task activities on schema23.

## Session guidance and workspace

The initial conversation contains the user request and a short pointer to a
private per-attempt guidance file. Reporting rules, nonempty criteria, original
instructions, previous result/revision context and the explicit workspace live
in that file. Existing repository instructions are preserved. Guidance and
capability files are removed on normal session termination.

Antigravity receives the workspace through both its process working directory
and --add-dir; the guidance explicitly prohibits using provider profile/scratch
directories for deliverables. Conversational requests do not imply creating an
app, repository or tests. The reporting CLI --help includes the optional JSON
schema example; agents must report actual tool errors rather than inspect the
server executable to infer a contract.

## Reading and reviewing the task list

The list shows workflow status; finished-process labels stay in session details.
Search, filters and sort retain their URL state. Desktop uses a compact table;
tablet and phone use task rows without horizontal scrolling. Empty project
labels are omitted. If an agent-refined title differs from the original request,
a Request preview keeps the human intent visible beneath the title.

Before approving, compare the delivered result with the original request shown
in the approval dialog: check scope, the correct output workspace, and actual
verification. Passing tests or agent-created criteria alone do not establish
that the user’s request was satisfied. Session guidance asks the agent to make
the same comparison before reporting and to avoid expanding a title or checklist
into unrequested deliverables.
