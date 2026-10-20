## Scope-bound plan approval

Production application release: ca9028fbb2602639fef12200421323f2503827e7.

The agent can provide an executable structured proposal with a title, exact
scope/limits/acceptance description and project destination. The UI renders that
description and an Approve and start action. The authenticated API accepts only
a reference to a completed proposal belonging to the same owner, conversation
and agent, then persists the exact scope snapshot in its user-message envelope.
It assigns a stable proposal-specific request ID. The dispatcher launches the
saved scope without another provider response; substituted scope is rejected.

Targeted tests verify proposal creation does not launch work, forged user-message
references are rejected, natural “Gas” approval binds to the immediately preceding
plan, questions do not bind, an explicit repeated button action reuses that
approval/task/attempt, and only one provider call (the initial proposal) occurs.
The task receives the exact approved scope. Existing delegation, recovery,
attachment and notification targeted checks pass.

Browser fixtures at 390/820/1440 verify the exact description, approval payload,
stable request identity, disabled Approved state, preserved unsent draft and no
horizontal overflow. Fixtures use fake APIs and no inference. See browser.json
and screenshots. TypeScript/Vite build passes with existing CSS/chunk warnings.

Usage and contract: docs/guides/plan-approval.md. Direct unstructured execution
still relies on model intent interpretation; do not claim universal immutable
approval enforcement or full product-goal completion from this feature.

## Release verification

Clean-worktree targeted tests pass (23.592 seconds), including fake-provider
approval/execution, process-loss recovery, legacy notification and delegation
checks. Clean TypeScript/Vite build and browser fixtures pass. New browsers use
English even with Indonesian navigator languages; explicit Indonesian and Auto
preferences remain available. Default-language checks also pass in production.

Atomic native activation/readiness and read-only mobile/desktop recovery smoke
pass. No production approval click, message send, retry or provider inference was
performed. All 36 application tables (auth sessions excluded) and 863 profile
files match /opt/cuan/backups/plan-approval-release. Schema24 and disabled automatic
migration remain unchanged. Previous release: 0071835c4c6f8e7a25f6ef549e23b6570a2d32d9.
See production.json. Existing CSS syntax/chunk-size build warnings remain.

## Optional choice actions follow-up

Added Change plan (composer reference/focus, no send) and Not now (persisted,
idempotent deferral, no provider/task launch). An approved task cannot be deferred
through this action; use actual work controls. Deferrals cannot authorize execution.
Browser fixtures verify all three actions at 390/820/1440, preserved drafts and
focus, 44px minimum touch height, saved deferral and subsequent approval. Backend
fixtures verify repeated deferral, no task/provider call, rejected use as consent,
later approval and refusal to defer already-approved work.

The structured approval → execution → SIGKILL → progress checkpoint → same-task
revision → verified result fixture now passes as one integrated path, with one
artifact creation, two attempts and one final coordinator outcome. Direct explicit
authorization remains supported by its existing fixture.

Read-only comparison of Fural's task create page confirms it calls createTask and
navigates with spawn/autostart=1 for a selected agent, without a mandatory plan
step. Cuan likewise keeps proposals optional; the choice actions are only for
conversations needing a decision.

## Direct execution alignment

Coordinator guidance now explicitly treats a clear work instruction as authority
for that stated scope, without requiring “create task/run agent” words or another
planning turn. Implementation planning stays internal. Proposals remain optional
for ambiguity, additional scope or consequential undecided actions.
A fake-provider fixture sends an ordinary read-only audit instruction and verifies
single-turn delegation, progress/result, identity and idempotency. This validates
application plumbing and delivery of guidance, not universal model intent accuracy.

## Direct-work release verification

Application release 3f98a030f4c39b5497733134e87b5f79c73cf64f delivers optional
choices and coordinator alignment. Clean targeted server checks pass (34.140s),
including direct instructions, approval/deferral, process-loss recovery and
legacy notifications. Clean TypeScript/Vite build and 390/820/1440 browser
fixtures pass. Native atomic activation and readiness pass.

Read-only production language/recovery smoke and served optional-action assets
pass; no new message, approval, retry or provider call was made. The production
check does not claim a new real-model turn was exercised. All 36 application
tables (auth sessions excluded) and 863 profile files match the pre-release
snapshot at /opt/cuan/backups/direct-work-release. Schema24 remains unchanged,
automatic migration disabled. Previous application release: ca9028f.
See direct-work-production.json. Existing build warnings remain unchanged.
