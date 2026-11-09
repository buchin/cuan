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

## Separate visual coverage finding

Room `a86b4638-8afc-4fea-a542-d07d52880488` received Mermaid and terminal
screenshots during its post-release check although its approved scope is the
Inbox list indicator. `scripts/verify-ui.sh` invokes `web/scripts/verify-ui.mjs`,
which always captures that broad set. Its component manifest does not include
`InboxSubPanel` or `TaskProgress`. Those generic captures alone do not establish
the requested indicator's visual correctness. This backend correction does not
claim to repair that coverage/attachment selection gap.
