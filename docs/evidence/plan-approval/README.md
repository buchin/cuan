## Scope-bound plan approval

Implementation pending clean release verification; not deployed yet.

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
