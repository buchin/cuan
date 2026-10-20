## Goal

User cukup menyampaikan tujuan dan memberi persetujuan; Cuan mengurus pekerjaan
sampai tuntas, termasuk memulihkan gangguan tanpa kehilangan konteks atau
menggandakan pekerjaan.

This audit preserves that end state. A successful retry or green unit test alone
does not establish the whole goal.

## Current requirement audit

| Requirement | Authoritative implementation/evidence | Current finding |
| --- | --- | --- |
| Accept a natural-language goal without requiring agent/project/task forms | Studio routes through durable Inbox; studio.go and studio_test.go | Implemented; existing targeted fixtures cover dispatch and identity |
| Clarify ambiguity and obtain approval for consequential scope | workflow/coordinator.md; inbox_approval.go; inbox_delegation.go | Structured proposals now render an exact scope and persist a same-owner/agent/conversation approval snapshot; canonical dispatch needs no model rewrite. Direct unstructured intent still relies on model interpretation; deployed in ca9028f; clean fixture and read-only production verification pass |
| Preserve original goal, project and authorization across execution | SourceRoomID/SourceMessageID, stored task instructions, task-bound reporting capability | Direct and structured-approved work retain the original request; structured scope snapshots, digests and task-bound capability are verified by plan-approval and recovery fixtures |
| Start approved work once and keep the user informed | Atomic attempt reservation; Inbox progress delivery and one review per terminal attempt; inbox_delegation_test.go | Existing fake-provider flow covers launch, progress/result, revisions and idempotency |
| Retry known no-effect failures without user operation | New opt-in per-turn recovery policy, one child attempt per message, dispatcher retry | Deployed in 3b230b7; docs/evidence/automatic-recovery records isolated SQL/manual-auto races and production delivery |
| Preserve context during recovery | Original message/attachments reused; prior task summary/verification in session guidance | Latest-attempt progress-only checkpoints now reach recovery guidance and coordinator review; integrated process-loss fixture passes (task-recovery evidence) |
| Never overlap an orphaned execution with a replacement | Live manager guard and durable active-attempt lease | OS supervisor lock and parent-loss cleanup implemented; real process/reconstructed-manager tests pass; deployed in 0071835; production service-user binary conflict/exit probe passes |
| Continue interrupted approved work from actual progress | Coordinator reviews interrupted attempts and may issue a scoped revision | Real fake-provider SIGKILL flow preserves one artifact and progress, continues the same task and verifies the result; no native provider-session checkpoint is claimed |
| Handle unavailable quota/auth without blind repeats | Allowed account/model routing, cooldowns, explicit backup participation, safe recovery actions | Implemented with targeted routing tests; unavailable external access remains a reported blocker |
| Return verified results and continue within approved scope | Task reporting, outcome reconciliation, coordinator result review | Structured goal → approval → execution → SIGKILL → checkpoint → scoped revision → verified final outcome passes as one fake-provider path; one artifact, one task, two attempts, one final result |
| Keep setup and usage self-contained | Native sandbox setup/check guide | Native sandbox and actual binary first-run/restart verified; current public guides reconciled. Docker AGY turnkey runtime remains explicitly unsupported/unverified |

## Remaining completion work

The latest verified behavior keeps clear instructions direct. Optional decisions
use exact scope snapshots and choice actions; planning is not mandatory. The
application release is e828d1f/schema25, with detailed evidence in launch-queue.

1. Reconcile public onboarding and legacy message/task guides with current
   coordinator-managed execution, sandbox prerequisites and persistent profiles.
   The old Docker quickstart still does not establish a turnkey AGY runtime.
2. Launch recovery is now deployed in e828d1f/schema25: memory and real PostgreSQL
   contracts plus SQL-backed reconstructed-server cases prove one launch after
   route release, durable cancellation and scope-change rejection. Production
   backup/migration/retention and actual app rollback/reactivation pass. Preserve
   its24-hour waiting bound and explicit external-blocker outcomes.
3. Audit native fresh-install prerequisites and the actual supported deployment
   path; application/database health alone is not provider readiness.
4. Preserve documented boundaries: model intent interpretation for direct natural
   instructions, unknown external side effects, cancellation, and external access
   blockers must not be advertised as guarantees beyond actual evidence.

Network egress and a native provider-session resume are separate implementation
choices; neither should be advertised as already delivered. The acceptance
invariants remain context preservation, verified process ownership, approved
scope, no duplicated work, and a useful final result.

## Fresh-install documentation checkpoint

Public onboarding, usage, message lifecycle and troubleshooting now describe the
current Studio/Inbox workflow, optional choices, durable launch waiting, persistent
HOME/data and native sandbox prerequisites in English. Seven isolated installer
contracts pass (see `docs/evidence/install-readiness/README.md`). Docker runtime
validation remains open: this environment lacks Compose and a reachable daemon;
no image build or container recreation has been established by these checks.
The complete goal remains open until fresh-install execution prerequisites are
verified on a supported host.

## Guided onboarding and real native first-run

New Studio users are guided to connect AI, then return to their saved goal.
Connecting a project is optional. Browser checks cover390/820/1440, account-fetch
failure, draft retention, optional project/cancel and exactly one submitted goal.
Migration19 no longer seeds a shared owner for new databases; the command startup
no longer overwrites Default. Existing applied migrations/owners/projects remain
preserved. A real PostgreSQL full-server lifecycle and an actual native binary
process-restart check pass with disposable databases; evidence is in
`docs/evidence/onboarding/`. Provider inference and Docker execution were not part
of those tests. Production delivery of this change is pending.
