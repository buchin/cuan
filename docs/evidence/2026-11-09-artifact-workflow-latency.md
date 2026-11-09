# Standalone artifact workflow latency

Observed through official `cuan task get ... --json` and
`cuan chat messages ... --json` on 9 November 2026. No retry, task mutation,
manual acceptance or customer message was performed.

Room: `694d605f-ebb4-45d5-860e-fc3fd7b8f644`.
Task: `msg_e90d5e5f-3850-47c1-8db3-db824902bf54`.
The current task remains Done with two successful native attempts.

| Segment | Approximate seconds | Authoritative boundary (+08) |
| --- | ---: | --- |
| Initial PM turn | 53.8 | 10:48:21.729–10:49:15.558 |
| Developer admission | 2.3 | 10:49:15.536–10:49:17.808 |
| Developer execution | 103.1 | 10:49:17.808–10:51:00.890 |
| First PM review | 88.3 | 10:51:00.959–10:52:29.304 |
| Verifier admission | 3.5 | 10:52:29.282–10:52:32.822 |
| Verifier execution | 91.8 | 10:52:32.822–10:54:04.657 |
| Final PM review | 62.6 | 10:54:04.717–10:55:07.340 |

Total incoming request to native Done: approximately 405.6 seconds. Rows overlap
slightly because admission can begin before the preceding reply finishes saving.
Queue delay is not the dominant bottleneck in this observed run. Both PM reviews
start within 11 milliseconds of request creation. Their actual provider response
stages take 82.4 and 48.8 seconds; process durations are 85.5 and 52.3 seconds.
Both use `gemini-3.8-flash-high`, with no investigation tools before formatting.
This does not establish whether provider compute, provider-internal queuing or
prompt reasoning dominates those stages; native trace cannot separate them.

The first PM review's saved decision is the mandatory independent-file handoff.
Code currently applies `requireIndependentArtifactReview` after model evaluation
in `applyInboxReviewEvaluation`. A possible next optimization is routing a
successful delivered standalone artifact directly to required verification,
using the existing saved assessment and launch machinery. This is not yet
implemented. It must not auto-accept work or skip final semantic review. Scope,
owner follow-ups, cancellation, current final-report provenance, unavailable
artifacts, independent-verifier lineage, persistence failures and launch
deduplication must remain enforced. Missing or failed work must retain the
ordinary PM evaluation/correction path. A timing win requires a fresh native
production benchmark, not subtraction of the old review duration.

Owner account inspection also confirms Sonnet is selected with Gemini fallback,
but current Sonnet runtime cooldowns remain. Fresh usage for one account reports
0.85184 percent remaining for Sonnet; that does not invalidate its saved runtime
cooldown or prove a new request can run. No routing settings were changed.

Current production inspection found zero doing tasks. The registered storage
maintenance job is enabled; its post-restart status is idle with a next scheduled
run, not evidence of a fresh successful cleanup or reclaimed bytes.

## Direct mandatory-verification routing

Implemented `inboxArtifactVerificationHandoff` in the existing Inbox lifecycle.
For a bound current review, a successful standalone-file developer attempt with
a current final report and available owner-scoped deliverables supplies the
existing verification-only assessment directly, before provider inference.
The normal `applyInboxReviewEvaluation` path still saves and applies the
assessment, checks owner context/cancellation, and admits the existing deduplicated
read-only launch. No new queue, acceptance path or workflow service was added.

Native verifier results remain on ordinary semantic PM review, including when
their unchanged-directory proof is absent. Progress/blocked/missing reports,
failed/stopped executions, unavailable files, ordinary operations and stale
requests do not use the shortcut. Changed owner context defers to normal review.
The shortcut cannot emit pass or establish that any original criterion is met.

Targeted tests cover ready routing through the real saved-assessment/native
launch path, repeated application without duplicate execution, native verifier
completion returning to semantic review, and incomplete/stopped/unavailable/stale
negative cases. Existing owner-context and launch-persistence regressions pass.
Compilation initially exceeded local memory; the same targeted tests passed
using `GOGC=30 GOMAXPROCS=2 go test -p 1` without relaxing assertions.
Production timing improvement remains unproven pending a fresh benchmark.

## First post-change production attempt: routing regression found

Release `617efaf52fc6b53b9a4c29b462e2178c9e34d3f2` passed exact-SHA
health with38 migrations at2026-11-09T04:52:46Z. Fresh room
`221dde65-c85c-4cb7-97d8-aec7c4743633`, request
`1b0af4d5-15b5-4218-879a-5c2463d2f869`, repeated the original five-line
file request. Initial PM completed in128 seconds and created task
`msg_bab71131-021e-48d1-99a1-b57d626ea879`.

The generated scope explicitly said “Dilarang membuka atau mengubah repositori
aplikasi”. Artifact classification recognized `jangan` but omitted `dilarang`
from its no-repository-change expression. The task incorrectly entered the code
lane and prepared an application checkout. Root requested cancellation to stop
that scope violation; official task readback confirms attempt
`att_8166a3507e0bd3aad0de5741d49c1373` canceled at12:57:48.430476+08,
exit130. This is an interrupted diagnostic benchmark, not autonomous success,
not evidence of faster handoff and not accepted work. History is preserved.

The shared artifact classifier now recognizes the explicit `dilarang` form.
Targeted execution-policy tests preserve the negative case where only opening
production is forbidden but local repository edits remain requested. A task-lane
regression uses the actual generated scope and asserts ops. Both packages pass.
This fixes the deterministic routing cause rather than asking the PM to use a
specific synonym or manually changing the existing task's lane/status.
