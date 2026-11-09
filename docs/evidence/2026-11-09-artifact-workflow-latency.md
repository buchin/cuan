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
