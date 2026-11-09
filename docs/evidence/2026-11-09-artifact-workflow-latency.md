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

## Context-selection native production result

Release `25da3ccf11778067ff568fedb95d759a8927f3bf` passed exact-SHA health
with38 migrations at2026-11-09T05:40:14Z. Fresh room
`33e1d537-ecbd-4b46-81e9-2d59f62ea015` repeated the unchanged five-line request.
Task `msg_c3a16b0e-36b5-468d-a6fc-cdd658cd7558` reached native Done at
13:49:05.240282+08 with two attempts and no manual acceptance, retry, steering
or workspace intervention:

- Initial PM request `68085755-6c91-4134-adbb-5e87d68cba7a`:
  13:40:39.092–13:44:14.948,215.9 seconds; provider process204.3 seconds.
- Developer `att_33a69ba6cab55721fdf9071fa0abb57a`:
  13:44:16.239–13:45:36.350,80.1 seconds; admission1.3 seconds.
- Mandatory handoff `18f6a1fb-a8c1-44d5-a660-bacd7f1a7ff3`:
  13:45:36.472–13:45:37.753,1.3 seconds with no provider stage.
- Verifier `att_bc69c02416f2bc5c642ffc50dd50f4a3`:
  13:45:41.239–13:47:38.290,117.1 seconds. Native read-only/deny push/deploy
  scope, private artifact workspace without Git checkout, and runner unchanged
  receipt are present.
- Final PM `d03a6b5b-ee0a-4608-a867-dd8076f65b04`:
  13:47:38.374–13:49:05.253,86.9 seconds. Context completes at13:47:45.295;
  model selection at13:47:53.011, before a72.1-second provider process with
  a67.9-second response stage. Model remains `gemini-3.8-flash-high`.

Total request to Done is506.1 seconds, slower than the previous409.7-second
run; final review86.9 seconds versus83.9 previously. This does not establish a
latency improvement, nor statistically establish a regression from selection:
the unchanged initial PM stage alone increased by115 seconds. Native completion
and correct output are proven for this fixture, not general model quality.

Root officially downloaded attachment `9f994111-9fc6-421b-a2c4-3fa988e23a7f`
and2557-byte report `07229ec6-c70f-409f-a02c-c3b8bf9d271f`, reading both.
The actual170-byte file has expected SHA256
`b903747b2d3c581e6a87b03e0bc310dfcd337daf503375bfc15102ac3b8f3d50`.
Copies reside in `/home/dev/artifacts/cuan-review-context/`.

Two official `inspect_app_logs` MCP invocations on the host returned no matching
trace records, including with system-journal privileges. This is unavailable
diagnostic evidence, not proof no process ran. Actual per-request selected prompt
bytes were not observed; only local selector sizes above are established. The
durable request trace does prove the measured stage boundaries. Next investigate
the repeated context/catalog preparation (about15 seconds here), the large
unchanged initial coordinator policy, and expose numeric prompt diagnostics
through durable trace so future investigations need no journal access.

## Remove discarded review preparation and persist input counters

`prepareInboxTurnContext` now selects the authenticated current task-review
snapshot before normal conversation/project analysis. Previously each review
built chat history, inspected project/backlog/Git state, then replaced that
entire prompt with the canonical review snapshot. The new review path retains
the exact bound project's AGENTS root and frozen task guidance, owner follow-ups,
current evidence and phase approval. Ordinary user messages and retry
reconciliation retain their existing context path. User text cannot select the
internal review path. A real Git probe fixture confirms that a review does not
run the discarded repository inspection.

Every provider stage also records `response_input` in the existing durable
request steps: known stage enum plus prompt/context/repair-candidate byte counts,
never prompt content, account paths or model reasoning. These counters appear
only under Response details → Technical details, and through official chat
trace/copy; they do not add a visible progress-timeline row. Bytes are explicitly
not tokens. A persistence failure follows the existing trace-integrity stop path.

Targeted context, authorization/follow-up, launch-persistence, CLI, artifact and
numeric-only trace tests pass. All463 frontend tests/typecheck pass. Real browser
fixture screenshots at390px and1440px were opened and inspected, confirming quiet
technical disclosure, unchanged timeline connectors and no horizontal overflow.
Screenshots: `/tmp/cuan-response-timeline-390.png` and1440.png.

Broader targeted testing found
`TestStalledReviewReconcilesCurrentSnapshotWithoutReplayingOldProvider` fails in
all three historical request states: its expected acceptance instead returns
saved-execution reconciliation. A Git-HEAD overlay of all modified existing
backend files reproduces the identical failures before this change. No test or
acceptance gate was weakened to hide this baseline issue. Baseline output is
retained under `/home/dev/.tmp/cuan-context-baseline-7il2ndzo/result.log`.
No claim of a fully green server suite or production latency improvement yet.

## Preparation rollout: category override found during native benchmark

Release `75ff6f20a671c83732d6e6ea61d3de9510d551fd` passed exact-SHA health
with38 migrations at2026-11-09T06:04:50Z. New room
`0f940c51-afc8-4ddc-a712-232da0ed256f`, request
`62ef2af0-6bb4-4da3-b18b-21b9e8e16ab6`, repeats the unchanged file request.
The durable trace now exposes actual initial input:79646 prompt bytes and15720
context bytes. This is production evidence without journal access, not token
usage or proof that input length alone causes the latency.

Initial PM completed at14:07:51.482+08 (153.4 seconds) and created task
`msg_42ac02ae-7c8e-4c7a-b10b-4aaef8b14f2c`. The task scope is recognized
standalone artifact work, but the explicit category supplied to creation can
override inference. It incorrectly used code lane and prepared a Git checkout.
Root canceled the diagnostic task to preserve the no-repository scope; native
attempt `att_1aa24bf8ff94586f7470440d6950b833` is confirmed canceled at
14:09:03.443021+08. No acceptance or latency improvement is claimed.

A regression reproduces explicit code/readonly hints overriding this recognized
scope, including through metadata update. Task creation and updates now constrain
such hints to ops for explicit standalone-file work. Category is scheduling
metadata, not authority to open a forbidden repository. Ordinary repository work
and schema-migration locking retain their existing classification. Targeted
category/inference/frozen-project-context tests pass. Historical canceled tasks
are not manually recategorized, restarted or removed.

## Native proof after category enforcement

Release `c73bc0dfca38986ad9661b25378cec90e2812da8` passed exact-SHA health
with38 migrations at2026-11-09T06:16:01Z. Fresh room
`9140612b-dd6e-499c-a7c4-e810558d8e1d` repeated the original request without
changing its wording. Task `msg_01a99aa6-9092-4f5d-9247-7e68f5c344e6`
reached native Done at14:22:18.828365+08, exactly two attempts, without manual
retry, acceptance, steering or workspace edits in this run.

| Stage | Time (+08) | Seconds |
| --- | --- | ---: |
| Initial PM | 14:16:46.124–14:18:29.374 | 103.2 |
| Dev | 14:18:33.871–14:19:41.749 | 67.9 |
| Mandatory handoff | 14:19:41.825–14:19:42.378 | 0.55 |
| Verifier | 14:19:43.874–14:21:01.228 | 77.4 |
| Final PM | 14:21:01.310–14:22:18.845 | 77.5 |

Both dev `att_8836ba1c31c5966822df9770bbddbb7c` and verifier
`att_db77272a202b46551f39f04685ccc266` use the private ops artifact workspace
without Git checkout. The verifier has native read-only/deny push/deploy scope
and the runner's unchanged-directory receipt. Handoff request
`5ccf8ad1-a5c9-4eec-ac7b-71ac957c039a` uses no provider stage.

Final review `be42515b-47a1-45b9-a9e2-ac2517d64cd7` records context complete
at14:21:01.456753,0.146 seconds after request creation, versus6.921 seconds
in the preceding completed benchmark. Actual input preparation ends at
14:21:02.706315,1.396 seconds after creation, versus14.676 previously; quota
and catalog-cache variability also affects that latter difference. The provider
process still takes76.0 seconds, including a71.5-second response stage. Do not
attribute provider inference speed to removal of discarded project analysis.

Durable input counters now prove the actual final review prompt is41040 bytes,
context15263 bytes; initial PM is79646/15720 bytes. These are actual request
bytes, not token usage. The unchanged initial coordinator instructions account
for approximately64KB beyond context and remain a concrete next investigation.

Total request to Done is332.704 seconds versus506.149 in the preceding completed
run. This is one observed workflow, not a controlled latency distribution or a
claim that all173 seconds were saved by this code. Initial model latency and
worker/verifier durations also vary. The confirmed deterministic reduction is
the discarded context path, while Done still uses independent semantic review.

Root officially downloaded170-byte file attachment
`cf965365-af7b-4550-a65e-04c8a8cacfb2` and1147-byte verifier report
`0e8f3678-128a-4597-a4dc-fca04cabc1bc`, reading both. Actual file contents match
all five lines/newline and SHA256
`b903747b2d3c581e6a87b03e0bc310dfcd337daf503375bfc15102ac3b8f3d50`.
Copies: `/home/dev/artifacts/cuan-snapshot-preparation/`. Without an explicit
maintenance trigger in this run, the existing job records success at
14:21:17.057839576+08 after verifier completion. No reclaimed-byte claim.
