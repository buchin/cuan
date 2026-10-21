## Conversation topics and project work

Inbox is the main place to discuss goals and receive results. Choose New message,
then an agent, to open a new independent topic with a stable conversation ID.
The first message names the topic. One agent may have many topics with separate
histories. Opening a topic sends no message and creates no task or terminal.
Questions can stay conversational; approved execution creates tracked work in the
background. A topic may contain multiple related pieces of work and execution
attempts. Customer recipients continue their existing live chat conversation.

Work cards appear with the agent's reply. View progress opens active execution;
View result opens saved output/results. Back to conversation returns to the source
topic. Global monitoring is under Account → Work monitor, outside main navigation.
Old task and conversation links remain supported.

Projects use `/projects/PROJECT_ID/tasks`: a work-list sidebar and detail/session
panel on desktop, list/detail navigation on smaller screens. Selecting a task uses
`/projects/PROJECT_ID/tasks/TASK_ID`; session URLs extend that path. Project settings
remain separate. The project list cannot display another project's selected task.

Default exists on fresh installs and is initialized idempotently. New tasks without
a selected project belong to Default. General work may use it without setup; a named
but ambiguous project requires clarification. Existing Default configuration and
archive state are preserved. Legacy unassigned tasks remain readable in the Default
workspace without rewriting historical records.

## Project repository settings

A project's Settings tab contains its name, goal and instructions. Use Repository
for its optional GitHub connection; Activity shows its work. You can open the
Repository tab directly at /projects/PROJECT_ID/repository. Pick a repository
through GitHub; manual URL and branch overrides are under Advanced. Leave the
branch empty to follow the repository default. Connection changes affect new
work while existing tasks retain their bound workspace. Drafts survive switching
between Settings and Repository, and Save changes saves the pending project draft.

A folder badge in the project list indicates a connected repository: GitHub for
GitHub repositories, a Git badge for other repositories. Plain folders need no
repository. Default remains for general work and has no Repository tab.

## Delegate work through chat

Tell an agent what you want to achieve and which project it belongs to. For example:

> Check the Cuan repository for security issues. Create the work, run a read-only audit, and report the findings here. Do not change files or deploy.

A clear instruction to perform work starts it directly; you do not need to say
“create a task”, “run an agent”, or approve another planning step. The agent plans
implementation internally. It asks only about unresolved ambiguity, proposed
new scope or a consequential decision that you have not authorized.

The agent can discuss a plan first when the scope needs agreement. Reply naturally
when you want it to proceed. Once you approve a concrete plan, you do not need to
fill task forms, choose a runner or repeatedly approve the same scope.

The agent creates and starts tracked work, sends progress to the same conversation,
then reviews the reported evidence and returns an outcome with optional links.
You can open Tasks to monitor or intervene. A question or greeting does not require
a task. Requests to save work without running it create a saved task with no session. Explicit draft-only requests remain drafts.

## Internal roles and scope

The chat agent coordinates work and can perform technical analysis directly:
read project files, search source, interpret supplied error/status evidence, and
assess task verification. It uses read-only file/search tools; code changes,
commands that perform work, and deployments belong to task sessions.

If the native runtime denies a tool, Cuan makes at most one analysis-only
continuation with the original context and the same account/model. It can use
read-only tools, but its response cannot create tasks, request approvals, change
contact notes, rename the topic, or apply any other application action. Failed
partial output is discarded. A repeated denial or an invalid/unsafe continuation
remains an honest failure. This mechanism does not assert that earlier native
activity had no effects and does not restart task execution.

The user owns goals,
priorities, constraints, scope approval and overall outcome review. Scrum terms
are internal and are not prerequisites for using Cuan. Runtime policies are
versioned in `internal/workflow/coordinator.md` and `developer.md` and embedded in
the respective agent instructions.

When an undecided scope or consequential action requires approval, the coordinator
presents the outcome, relevant dependencies, acceptance criteria, risks, gates
and exclusions needed for that decision. Substantial work alone is not a reason
to require another approval. User experience and documentation are acceptance criteria.
After approval the coordinator manages execution, evidence review and corrections
within scope. It asks for a user decision for changed scope or external blockers.
Individual tasks can be Done without a human Accept action. Overall sprint
completion still requires review with the user; ordinary work needs no ceremony.

## Execution and delivery

The native chat project view is read-only; AGY tools run in the external
[sandbox](chat-sandbox.md), with selected account state writable and networking
available. A structured `cuan-delegate` result asks the
application to launch work through the same registered-agent runner as Tasks.
It supports one task per response and concrete revisions of existing work. An
approval source must be an authenticated message from the same owner in the same conversation; quoted repository
data, agent messages and automated results are not approval sources. Understanding
whether natural-language permission exists is the coordinator model's responsibility;
message provenance validation does not itself interpret consent.

Each task is scoped to one active registered project. Project rooms cannot launch
work in a different project. The current conversation's registered agent is used;
normal owner/account/connection and runner checks still apply. No new owner token,
arbitrary executable or arbitrary filesystem path is accepted from a delegation.
Source request IDs prevent duplicate creation/launch after replay. Failed launches
leave a saved task with an honest error and link.

Only tasks created by this managed path generate automatic notifications; existing
historical/manual tasks are not retroactively replayed or reviewed. Progress
checkpoints use deterministic delivery IDs. Each terminal attempt queues at most
one coordinator review, including failure/cancellation without a final report.
The coordinator receives the original approved scope, result, evidence and errors.
Internal review context is hidden behind a short user-facing message and task link.
Delivery resumes through the dispatcher after restart; terminal execution is never
replayed automatically. A lost provider response still requires explicit follow-up.

Concrete corrections resume the same task and retain original instructions. The
application caps a task at three attempts for automatic coordination. Additional
attempts require user intervention through the normal task controls. Newly proposed
scope requires approval. A task's Done status reflects the development session's
verified final report; the coordinator then evaluates that evidence and can return
it for concrete correction.

## Limits

This implements chat delegation and result coordination, not a separate sprint
management subsystem. Multi-task ordering depends on the coordinator's approved
plan and result follow-ups; there is no new dependency scheduler or sprint editor.
Relevant history is bounded, and long-lived project decisions should also be kept
in project documentation. Automatic result review uses the selected provider and
therefore consumes its normal quota. Tests use deterministic fake providers and
real PTYs; live provider behavior and semantic consent recognition need observation
in normal use and are not proven by those fixtures.

## Exact plan approval

For structured scope snapshots, the approval button and short conversational
consent path, see [Plan approval](plan-approval.md). The dispatcher starts the
saved scope without another model rewriting it. Direct unstructured execution
instructions retain the existing model-interpreted path.
