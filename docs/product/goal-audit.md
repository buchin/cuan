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
| Clarify ambiguity and obtain approval for consequential scope | workflow/coordinator.md; inbox_approval.go; inbox_delegation.go | Model is instructed correctly, but backend currently verifies an authenticated approval message, not an immutable structured scope approval |
| Preserve original goal, project and authorization across execution | SourceRoomID/SourceMessageID, stored task instructions, task-bound reporting capability | Implemented for normal delegated work; approval scope enforcement needs strengthening |
| Start approved work once and keep the user informed | Atomic attempt reservation; Inbox progress delivery and one review per terminal attempt; inbox_delegation_test.go | Existing fake-provider flow covers launch, progress/result, revisions and idempotency |
| Retry known no-effect failures without user operation | New opt-in per-turn recovery policy, one child attempt per message, dispatcher retry | Implemented and tested locally/isolated SQL; release evidence must prove production delivery |
| Preserve context during recovery | Original message/attachments reused; prior task summary/verification in session guidance | Safe chat retry is covered; progress-only checkpoints are not yet included in task recovery guidance |
| Never overlap an orphaned execution with a replacement | Live manager guard and durable active-attempt lease | Gap: expired lease is not proof an old OS process is dead; durable execution fencing is required |
| Continue interrupted approved work from actual progress | Coordinator reviews interrupted attempts and may issue a scoped revision | Partial: no durable native provider checkpoint; recovery context and holder verification must be strengthened before treating this as reliable continuation |
| Handle unavailable quota/auth without blind repeats | Allowed account/model routing, cooldowns, explicit backup participation, safe recovery actions | Implemented with targeted routing tests; unavailable external access remains a reported blocker |
| Return verified results and continue within approved scope | Task reporting, outcome reconciliation, coordinator result review | Existing evidence covers nominal flow; end-to-end interruption/recovery proof remains necessary |
| Keep setup and usage self-contained | Native sandbox setup/check guide | Native path verified; Docker AGY turnkey runtime and several legacy guides remain gaps |

## Next concrete verification

1. Prove safe automatic recovery across a reconstructed server, with one original
   message, one recovery child, bounded model calls, and a manual/automatic race.
2. Persist/verify execution ownership independently of an expiring lease before
   replacing an interrupted task; exercise actual child-process loss.
3. Include bounded public progress checkpoints and previous execution state in
   the next task session and coordinator review, then prove no completed work is
   repeated in a fake-provider interruption fixture.
4. Make approval scope explicit enough for the backend to reject unrelated user
   messages or altered proposals as authorization.
5. Reconcile documentation and install checks with current behavior, and run an
   integrated goal → approval → execution → interruption → recovery → result
   scenario without real AI inference.

Network egress and a native provider-session resume are separate implementation
choices; neither should be advertised as already delivered. The acceptance
invariants remain context preservation, verified process ownership, approved
scope, no duplicated work, and a useful final result.
