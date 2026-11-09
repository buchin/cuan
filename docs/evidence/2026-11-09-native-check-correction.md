# Native project-check recovery

The telemetry workflow in room `67909438-7e90-49af-bfb8-6c40a003bac4`
retained source `8cd1d1e6c40ea6574cfc93f2cbd6436c59cba316` but its
native `test.sh --fast` gate failed. The recorded failure was
`TestSupplementalVerifierUsesPrivateExactCommitWorkspace`: a different completion
was already retained. This is not evidence of a feature or deployment failure.

## Root causes and correction

The fixture placed its workspace directly under the test temporary root while
the completion outbox deliberately lives beside the workspace directory. Its
outbox consequently escaped test cleanup into shared `/tmp/runner-completions`.
Recycled HTTP server ports and fixed fixture identifiers could collide with an
old, different result. The production idempotency guard correctly preserved the
old result. Nesting both verifier/finalizer fixtures inside their own temporary
workspace tree keeps their outboxes owned by test cleanup. Their mock completion
endpoints now return matching native acknowledgments, and tests check delivery.
No production outbox is deleted and no completion identity check is weakened.

The publication review guard also converted a concrete developer-test correction
into release reconciliation when deployment evidence was missing. A current,
runner-bound failed project check now precedes publication classification. It
requests a bounded developer correction with the actual diagnostic, retaining
original scope, history, independent verification and publication requirements.
Inspection/verifier launches, cancellation, unknown failures, foreign records,
passing checks and missing current reports cannot use this source-correction path.
The existing review evidence digest revisits only affected saved failures.

## Validation and limits

Targeted verifier/finalizer fixtures passed three consecutive runs. Targeted
server tests cover native failure priority, model-pass rejection, stable repeated
classification and rejection of foreign/historical/canceled/incomplete evidence.
Existing publication and prerequisite review tests passed. The project fast gate
also passed on the final source (`GOGC=30 GOMAXPROCS=2 GOFLAGS=-p=1
./scripts/test.sh --fast`).

These checks do not establish production recovery or task completion. The same
retained telemetry task must still correct its source, pass independent checking,
publish and produce live evidence without manually forcing its status.

## Lean follow-through

Production review `8b7dbada-9ed5-4de0-a7dc-d18fe53a0f9d` spent 191.2 seconds
in its provider process merely to request the already-known test correction.
The corrected publication guard classified implementation, but its embedded
6000-byte diagnostic plus instructions exceeded the delegation contract and
admission returned `Invalid delegation`. The task remained blocked; this is
not successful recovery.

Native check correction now keeps a short failure-tail excerpt and refers to the
complete immutable native record through the existing bound task tools. A long-log
regression goes through the real review parser and existing correction queue,
including duplicate application. The lifecycle selects this deterministic
handoff before provider inference. It never accepts the source, changes scope,
resets correction budgets or bypasses native launch admission. Unknown outcomes
and independent verifier findings still need their ordinary assessment.

Production `b7b9f4c69d51434fe5e4e8b4756e84f54ea2b865` passed exact-SHA
health verification. Request `491cdff6-2726-42c7-a325-df9fe18e2d42` ran from
17:51:58.930 to 17:52:09.113 (+08), recorded `native_check_correction`, and had
no provider stage. The same telemetry task automatically entered developer
attempt `att_50ac926908cecd2779da1e162aafb8ef` at 17:52:10.338. This proves
the correction admission and removes the prior 191.2-second provider decision;
it does not yet prove that the corrected feature passes independent/live DoD.

## Separate visual coverage finding

Room `a86b4638-8afc-4fea-a542-d07d52880488` received Mermaid and terminal
screenshots during its post-release check although its approved scope is the
Inbox list indicator. `scripts/verify-ui.sh` invokes `web/scripts/verify-ui.mjs`,
which always captures that broad set. Its component manifest does not include
`InboxSubPanel` or `TaskProgress`. Those generic captures alone do not establish
the requested indicator's visual correctness. This backend correction does not
claim to repair that coverage/attachment selection gap.

The post-publication verifier report also left deployment incomplete solely
because its role forbade deploying, despite identifying the published containing
snapshot. The runner now gives this role a point-by-point DoD checklist mandate
and exposes its actual native post-publication stage, source/target containment,
repository and observation time after snapshot checks pass. It explicitly
distinguishes that identity evidence from feature correctness and remote health.
Current-outcome inspection of an unpublished source never gets this declaration.
Existing real snapshot tests cover both cases, failed checks, source mutation,
native image transport/tampering and unavailable evidence. No acceptance gate is
relaxed. Already delivered native screenshots are no longer offered for redundant
reattachment. This does not yet fix the registered visual recipe's coverage.

Direct inspection of the delivered native `inbox-390.png` confirmed it shows the
chat/composer, not the requested Inbox list indicator. Its existence therefore
cannot prove mobile alignment of the indicator. Preserve this negative finding
when assessing the task rather than treating a generic capture as visual DoD.

## Role routing and repeated preparation

Developer attempt `att_50ac926908cecd2779da1e162aafb8ef` completed with native
checks passing on source `6485a9dd36f2f01a9b6bc1f2234590e6e21490df`.
PM request `796c30aa-8bf9-4cea-ac07-f1f64158dc4d` requested independent checking
but retained only decision/explanation/revision, with no `remaining_work`.
The legacy unclassified revision path started ordinary writable attempt
`att_c444b37073ae3bbd1840651025f86d65` instead of a typed verifier. This is
another routing failure, not evidence that a verifier ran. A steering message
was delivered to preserve source, avoid reimplementation/deployment and report
only actual findings; native independence remains outstanding.

Ready tested developer commits now use the existing deterministic handoff
pattern to request mandatory independent verification before provider inference.
The existing source-verifier admission still checks original authority, native
source evidence and launch identity. An unclassified revision on a tested
implementation also defaults to read-only inspection instead of writable
execution, excluding explicit source rejection and owner/external decisions.
Regression tests exercise the real saved assessment/launch path and prove exact
source identity, one verifier launch and no acceptance without independent work.

Setup inspection found that npm preparation ignored the managed owner cache
already leased by the runner and created/deleted a new download cache every time.
It now accepts only the internally selected managed cache from that lease;
private installer HOME/configuration and separate node_modules remain unchanged.
The native npm offline fixture installs a real integrity-locked tarball into two
separate workspaces from one warm cache, checks the source lock is unchanged and
proves lifecycle scripts did not run. This removes repeat downloads; it does not
claim that all installation/materialization cost or this Go task's latency is
eliminated. Git snapshots already reuse retained directories and bounded local
history; Go caches already persist across these roles. No shared mutable Git
checkout or node_modules symlink was introduced.

Release `27dc12833f6c80ff588786a0c466a451e6fe1ddf` passed exact-SHA health
verification at 2026-11-09T10:14:44Z. It includes the role handoff, verifier DoD
stage context and shared npm-download preparation. The already active legacy
ordinary execution is preserved rather than being retroactively labeled an
independent verifier. No production source-verifier transition is credited until
the actual typed launch and source binding are observed.

At 18:18:26 (+08), task telemetry entered native verifier
`att_cf7169eb686500ecef880fd4d05b5e5b`, bound to developer attempt
`att_c444b37073ae3bbd1840651025f86d65` and exact source
`712d56e004d3767671dd64939836ad57d0972e89`. Official `task execution`
confirmed `intent_kind=verification`, read-only repository and denied push/deploy.
The agent stage began at 18:18:33, about seven seconds after runner start.
This transition used the existing validated native developer handoff and its
synthetic source-review assessment `ef0ce011-36d1-4889-aa3c-66058d8c99e8`;
it is not evidence of a model Inbox request or of the generic handoff path being
exercised. Verification remains active and no independent pass, publication or
Done is credited yet. The short observed preparation also means cloning/setup
cannot explain the earlier multi-minute PM/role-routing delays in this task.
