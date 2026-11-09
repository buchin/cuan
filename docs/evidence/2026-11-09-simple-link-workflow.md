# Small native code workflow benchmark

## Requested outcome

Fix duplicate `[View work]` links in one agent reply: retain one link per task
and preserve links to different tasks. Reuse existing reply formation, add focused
regression tests, and independently verify the exact task-branch commit. No new
features, layout changes, merge or deployment. This follows the user's explicit
redirection toward simple workflow completion rather than extended security work.

## Authoritative identities

- Room: `fa14bcf2-fede-4a51-8239-ab19affd205e`
- Initial request: `c28d99de-7f17-425a-bc91-62704246eb27`
- Task: `msg_347deb31-855d-4f3b-9ce2-414c2552589c`
- Final source: `395b64911c182bc4570a96d8ff7748b86e953a97`
- Developer: `att_31a733e384655763afdcc9783e98507d`
- Verifier: `att_2f3ad43c2fee55feefb2f221507241c3`

The initial send timed out only while waiting for its reply. Its returned request
identity and official trace confirmed acceptance; the message was not resent.
The room was bound to the registered Cuan project before the sole request.

## Observed native execution

Times below are UTC+08 as returned by the official Cuan CLI.

| Event | Time | Result |
|---|---|---|
| Owner request accepted | 07:14:46.965 | One explicit useful code request |
| PM model process | About 93 seconds | Delegated exactly one task |
| Developer started | 07:16:36.769 | Original write/push scope, deploy forbidden |
| Developer finished | 07:33:10.302 | Succeeded first pass; native suite passed in 41.705s |
| PM source assessment | 07:34:18.758 | Requested independent verification of exact source |
| Verifier started | 07:34:22.786 | Exact source above; read-only, push/deploy forbidden |
| Verifier finished | 07:39:30.433 | Succeeded first pass; unchanged source |
| PM accepted Done | 07:40:42.504 | Pass/completed/none on current verifier evidence |

Exactly two task attempts, both succeeded. No retries, manual task acceptance,
worker steering, follow-up owner message, extra task or source edits in the worker
workspace were supplied by the root observer. Root observed official state/logs,
fetched the actual source and inspected its implementation and tests.

The change adds deduplication to existing reply formation. Tests cover the actual
reported duplicated paragraph, repeated identical links, and preservation of
multiple task links, including delegation integration. The verifier reported
running `TestDeduplicateWorkLinks_DuplicateTask`,
`TestDeduplicateWorkLinks_MultiTask`,
`TestInboxDelegation_DeduplicatesIdenticalTaskLinks`, and
`TestInboxDelegation_PreservesMultiTaskLinks` (0.316s), plus independent cases.
Native source identity/unchanged-state receipts bind that verification to the
same commit; these receipts establish provenance, not arbitrary feature quality.

## Outcome and remaining limits

The requested branch handoff reached native Done through PM → developer →
independent verifier → PM acceptance without corrective intervention. Source
remains on its task branch, unmerged and undeployed as requested. This is one
successful bounded benchmark, not proof that every Cuan task is autonomous.

End-to-end latency was about 25m56s: developer about 16m34s, verifier about 5m08s,
plus initial reasoning and orchestration/review. This remains too slow for the
small change. Official account metadata confirms preferred Sonnet routes remain
in quota cooldown and configured Gemini fallback is active; no model policy was
changed or quota restriction bypassed. Attribute the latency only after measuring
its parts; successful attempt count alone is not performance success.

Separately, root release `523c741a618825089e2762dbdbb5f3864338bfba` passed
exact-SHA production health with 37 migrations. It refreshes in-flight reviews
when newer authenticated owner context arrives, reusing the same review request
without repeating developer execution. This benchmark did not exercise that
new-message branch; its dispatcher regression is separate evidence.

## Reporting overhead follow-up

The session guidance both prepared a report at launch and instructed the worker
to fetch another report template. The prepared-file instruction was at the end
of the long guide. It also required a separate printed completion answer before
the canonical final report, although the coordinator delivers the owner reply.
The benchmark's developer terminal logged 40.6 seconds of provider reasoning
around final report submission; this observation does not isolate its cause.

Guidance now puts the prepared report entry point first, makes template fetching
an explicit missing-file fallback, and directs the actual answer into the report
rather than another completion speech. Original criteria, evidence requirements,
native checks and independent review are unchanged. Prepared-report round-trip,
scoped reporting/review and executable report-command integration tests pass.
No end-to-end speed improvement is claimed without a subsequent measurement.

The original diagnostic history no longer starts at developer launch (its first
retained record starts at 07:22:25), consistent with bounded log retention.
A sampled 1,000-record window contained 48 short spinner fragments; that sample
alone does not establish logging as the primary latency cause. No logging
behavior or retention limits were changed as a speculative speed fix.

## Parent integration after independent source review

The delegate's no-merge/no-deploy boundary was retained throughout the native
benchmark. After reviewing its exact successful handoff, the parent integrated
only the link fix into dev as `b1c7615e`, under the existing direct repository
shipping authorization. The artifact-download feature was not integrated.
Focused deduplication/delegation tests and the newer owner-context review refresh
regression pass on the combined source. This is a parent release step, not a
claim that either delegate published the change.

Real Inbox React components were rendered at 390px and 1440px with isolated
fixture API data, with no live user/customer writes. Both have no horizontal
overflow or page errors. The screenshots were inspected: current conversation
bubbles, typography, navigation and composer remain calm and usable; task links
are projected into the existing task UI as designed. Files remain at
`/home/dev/artifacts/cuan-link-integration-b1c7615e/` on the development host.
These captures verify rendering, not a production end-to-end execution.

The maintained full work-panel fixture failed its obsolete expectation that a
task card initially appears in the chat; current UI intentionally uses the
composer Tasks disclosure. That failed check was not counted as passing. The
narrow capture adapter used the same actual application components and isolated
API setup and was removed after capture. Updating the maintained full fixture to
the current task disclosure remains a separate correctness issue for future
visual verification. Local Chromium required the existing Alpine runtime's
library path; no browser dependencies were installed or production config changed.

Release `4ecd373dddd4395f350854121fdd65462ee4070b` subsequently passed
exact-SHA production health with 37 migrations. Authenticated Inbox history
remained readable after release. Historical replies were preserved; new reply
formation uses the integrated deduplication.

## Maintain the real visual workflow fixture

The stale full work-panel fixture is now updated to open the current composer
Tasks disclosure, assert it initially closed, assert no duplicate task card in
chat, and verify the active task expands with its current progress. Details and
terminal actions use their actual accessible controls. Existing panel, timeline,
back/draft/scroll, full-height terminal and session-loading/state-transition
assertions are retained. Captured component metadata now includes ComposerTasks,
InboxWorkPanel and AccordionTimeline, which are actually rendered by the flow.

The maintained script passes at 390, 1440 and 1920px. Real screenshots were
inspected at mobile, desktop and the wide four-column detail view. A deliberate
conditional-hook regression still fails with the actual React error “Rendered
more hooks than during the previous render”, demonstrating the lifecycle guard
was not removed. The full native `scripts/verify-ui.sh` entry point also passes,
including its diagram build/capture and workflow captures, in 45.2 seconds.
Evidence is retained at `/home/dev/artifacts/cuan-current-task-disclosure/`.
This repairs a false verification prerequisite; no application UI was redesigned.
