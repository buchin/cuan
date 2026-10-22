# Task sessions as the primary workspace

Production: https://cuan.dojo.cc  
Release: `35984323a5d94abb39a7285c9565689f32afd2f7`

## Result

Opening a task enters its latest durable session. Without an attempt, the task
shows the original request and explicit Start/agent selection. Viewing a task
never launches execution.

Active sessions retain the native interactive terminal. Ended sessions show the
original request, that attempt's result, and next actions. Metadata, agent
checklist, history, evidence and saved terminal are behind Detail. Completed
result pages do not load xterm until requested. Closing metadata, including
after expanding/collapsing it, returns to the same selected session and filters.

Implementation review offers Accept and Request changes near the result.
Accept still requires verification evidence, current version, successful
attempt and completed criteria. Request changes focuses the reply field and
submits explicit feedback as a new idempotent attempt. Earlier sessions display
their own saved report and cannot accept/continue a later result.

Agents can use --result-kind answer for a conversational response without file
changes. This hides required Accept presentation and shows an answer/reply
experience. Unknown/legacy reports default to implementation. Classification is
stored in an existing activity scoped to the exact final report timestamp and
attempt. It does not auto-approve Done: backend final reconciliation still uses
Review and all implementation acceptance gates remain. This is agent-declared
presentation metadata, not a semantic correctness/security assertion.

## Verification

- TypeScript/Vite production build passed.
- Targeted task, CLI, scoped reporting, workspace and interactive PTY tests passed.
- New tests verify classification validation, exact report binding, conservative
  default and no automatic Done. Actual CLI transport accepts result-kind.
- Isolated native PTY fixture passed on production host using temporary HOME,
  fake provider and in-memory stores; multi-turn reporting/cancellation preserved.
- Browser fixture passed eight 360/390/820/1440px light/dark combinations:
  latest session routing, request/result, conversational answer without Accept,
  no terminal loaded by default, acceptance evidence/checklist gates,
  historical report isolation, explicit ready/start, metadata expand/close return,
  live input/reload without rerun, follow-up, missing session, and both create
  with agent and explicit save-only behavior.
- Authenticated production checks passed the same eight combinations for
  result/latest routing, request/result, collapsed evidence, no initial terminal,
  actual list click, history isolation, lazy saved terminal, ready tasks,
  metadata return, acceptance evidence requirement and exact deployed assets.
- Production smoke created/cleaned two synthetic tasks and two finished
  synthetic attempts. No provider/run-agent invocation or actual result approval.
- Production answer presentation was tested with deterministic fixture reports;
  no automated live-provider behavior/adherence call.
- Validated private backup before native atomic release; no active inbox/task
  work at the gate. Health/readiness passed.
- Post-smoke retention:35 retained tables and641 account-profile files unchanged
  (authentication sessions excluded). Schema23, auto-migrate false; no migration.
- Production project checkout fast-forwarded fromb92470e to3598432.

## Compatibility and rollback

Historical human instructions, titles and results were not rewritten. Old
reports remain implementation/unknown rather than inferred to be conversation.
Explicit metadata URLs use view=detail/full; canonical task URLs open the
workspace. Existing actual session URLs remain valid.

Use the documented native previous-release rollback and retain schema23.
