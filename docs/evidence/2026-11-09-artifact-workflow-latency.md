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

## Fresh native production completion

Release `c0eab376f84044035585a582b7c97b438d210eb2` passed exact-SHA health
with38 migrations at2026-11-09T05:02:23Z. Fresh room
`79a57af2-6c80-4e7a-91df-e5a281e6e53a` received the same original request
`716feb7b-fd86-49a5-91db-06f745f5d612`. Task
`msg_8832ed4a-9219-4068-bf24-662ea8250f04` reached native Done at
13:09:31.985382+08 with exactly two attempts, no manual retry, task status
acceptance, steering or workspace mutation.

| Segment | Seconds | Boundary (+08) |
| --- | ---: | --- |
| Initial PM | 101.1 | 13:02:42.275–13:04:23.394 |
| Dev admission | 4.0 | 13:04:23.346–13:04:27.309 |
| Dev execution | 114.0 | 13:04:27.309–13:06:21.353 |
| Mandatory verifier handoff | 1.3 | 13:06:21.447–13:06:22.779 |
| Verifier execution | 100.8 | 13:06:27.304–13:08:08.075 |
| Final semantic PM review | 83.9 | 13:08:08.175–13:09:32.029 |

Developer `att_d2e954db9ee36cc5e458af16cc3a1572` and verifier
`att_4af812671e784cdd201a3d28ad9e4ca3` both use ops artifact workspace,
without Git checkout. The verifier's native execution scope is read-only,
deny push/deploy. Its runner unchanged-directory receipt and current final
report remain present. Handoff request `5fa4d38a-2d03-45c3-aaa2-f37ba04f2c8b`
records `verification_handoff` and no provider/runtime/model stage. Final request
`da1c9169-8047-4f1a-a312-9369d93ea46b` does perform semantic provider review
before accepting all criteria. No third execution was needed.

Officially downloaded file attachment `a3b65d38-5d4c-48cf-a81b-12071fb35ac8`
is170 bytes and matches expected SHA256
`b903747b2d3c581e6a87b03e0bc310dfcd337daf503375bfc15102ac3b8f3d50`.
Root read the file and1264-byte verifier report attachment
`0d6752b3-86f3-41c0-a2bf-3c41e3c576a9`; actual contents match the requested
five lines and final newline. Downloaded copies are under
`/home/dev/artifacts/cuan-direct-verifier-v2/`.

Compared with the earlier run, the handoff review fell from88.3 to1.3 seconds.
Dev completion to Done fell from246.4 to190.6 seconds (about23 percent).
However, incoming request to Done is409.7 seconds versus405.6 previously:
slower initial/final model responses and execution offset the saved handoff.
Do not claim improved end-to-end latency or general reliability from one run.
The next bottleneck remains initial/final model latency, not runner admission.

Without an explicit maintenance trigger in this benchmark, the registered job
reports success at13:08:19.929733213+08 after verifier completion. This proves
event-driven maintenance ran, not that any particular storage bytes were freed.

## Review policy context selection

The next optimization preserves policy text verbatim while omitting definitions
of optional native observations that the actual snapshot does not contain.
The complete review instruction is27060 bytes; a snapshot with no optional native
observations selects18108 bytes (8952 fewer). Actual reduction depends on the
saved context; these are policy bytes, not model tokens or a measured latency win.

Outcome, original criteria, report/runner provenance, correction ownership,
execution authority, file delivery and acceptance paragraphs remain unchanged.
Present observations, including unavailable/failed observations, retain their
definitions. Malformed/nonobject snapshots retain the full contract. No evidence
field, owner follow-up, project guidance, schema or native acceptance gate is
removed. Selection happens before member Taste adds prose around the JSON so
preferences remain injected without breaking native context parsing.

Targeted tests compare retained paragraphs verbatim, prove full equivalence with
all native observation types present, conservative fallback, independent-artifact
routing and read-only/publication acceptance regressions. Source compaction alone
does not prove faster or equally good model judgments; fresh native production
completion remains required after rollout.
