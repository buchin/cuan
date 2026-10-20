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

Clear instructions remain direct. Optional scope decisions use exact approval
snapshots and choice actions. The current production application is5961b4a/schema25.

1. Supported native first-run and public documentation are now verified/reconciled.
   Actual binary startup, owner setup, neutral Default, process restart retention,
   browser onboarding and SQL-backed first goal all pass. Docker application build and persistent production/staging lifecycle now pass
   (docker-install evidence); AGY runtime remains unverified. Do not advertise
   turnkey AGY containers.
2. Combined SQL-backed proof now passes: Studio goal → exact proposal → owner
   approval → real agent SIGKILL with progress-only evidence → entirely new server
   instances → automatic scoped continuation → automatically delivered verified
   result. One task, two attempts, one artifact creation and one final result.
   See `docs/evidence/durable-goal-recovery/README.md`. This is reconstruction over
   durable SQL state, not a literal host reboot or arbitrary external effects.
3. Preserve scope, context, authorization and no-overlap invariants during that
   reconstruction. A native provider-session resume and arbitrary external effects
   remain distinct from application-level progress recovery.
4. Keep model interpretation, unknown external side effects, cancellation and
   missing external access explicit. Do not substitute passing local fixtures for
   proof that every provider/environment can always finish a goal.

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
of those tests. Production delivery is verified in49f89ec/schema25, including read-only smoke, actual rollback/reactivation and unchanged retained data.

The first-owner frontend gap found during the follow-up audit is now fixed and
verified: initial root/login has a two-field owner form, configured installations
show normal sign-in, failures retain form state, and successful setup leads to
AI onboarding. Real PostgreSQL concurrency admits only one owner across six
simultaneous setup requests. Existing credentials are preserved.

The approved-goal SQL recovery gap is now closed by the integrated fixture; the
companion direct-goal SQL lifecycle also passes after refactoring. The goal stays
open while Docker installation/runtime verification remains unresolved. The
native production application remains49f89ec/schema25; this final checkpoint
adds only tests and evidence and needs no application redeployment.

## Notification follow-up

User-requested notification improvements are deployed in5961b4a/schema25. Context,
relative times and explicit destination labels replace generic repeated titles.
Known handled proposals/retries/task states stop contributing to attention badges;
read flags persist and read-write failures do not block opening the destination.
Memory, real PostgreSQL (including a retained alert beyond100 newer recovered
attempts), responsive theme fixtures, read-only production checks and actual
rollback/reactivation pass. Existing tables/profiles, all11 read flags and non-feed
preferences are preserved. Evidence: `docs/evidence/notifications-refresh/`.
Docker build/runtime verification remains open; its uncommitted validation work
is separate from this native application release.

## Docker application lifecycle follow-up

Actual isolated Docker build and production/staging container recreation now pass
with the exact input hashes in `docs/evidence/docker-install/result.json`. Owner,
session, neutral Default, UID1000, HOME/data persistence and environment isolation
are verified; Compose missing-secret checks and seven installer tests pass.
Earlier statements that no Docker build/container recreation was established are
historical and superseded by this evidence. AGY execution in the stock image
remains unsupported/unverified, so this does not close the whole goal. Native
production remains5961b4a/schema25. Test daemons and image caches were stopped/removed;
production health passed. See `docs/evidence/docker-install/README.md`.

## Native runtime setup follow-up

`scripts/setup-agent-runtime.py` combines verified CLI installation, sandbox
installation and isolated CLI startup verification for the non-root service user.
Read-only native production checking passed without network/account access; three
negative/success contracts pass. README/onboarding now use this single command.
See `docs/evidence/native-runtime-setup/`. This closes the split-prerequisite
setup gap, not the unsupported stock Docker AI runtime or a fresh-provider
inference verification. The full goal remains open.

## Actual container namespace constraint

The default Docker sandbox gap is now reproduced, not merely inferred from
missing binaries. Actual Alpine Bubblewrap under a non-root container user and
standard security options exits1 at namespace creation; the same host supports
the native service-user sandbox. See
`docs/evidence/docker-install/sandbox-policy.json`. All probe resources were
stopped/cleaned and production health passed. Installing the CLI/Bubblewrap in
the image alone will not close this gap. The user was asked whether first-release
AI execution must be turnkey through Docker or native Linux is the official
execution path while container AI follows later. This deployment support decision
is pending; it has not been inferred from elapsed time. No scope was removed and
the full goal remains active.

## Docker feasibility diagnosis checkpoint

The next diagnostic established the exact clone denial and proved that a scoped
seccomp clone exception permits namespace creation. Mount propagation then fails
with EPERM, changing to EACCES when that syscall is permitted in the temporary
profile. A seccomp-only adjustment is therefore insufficient on this host; the
additional confinement rule is not conclusively attributed without audit events.
See `docs/evidence/docker-install/sandbox-seccomp-probe.json`. No experimental
policy is shipped; all resources were stopped/cleaned and production health passed.
A separate runtime or a fully verified specialized isolation policy are both
possible design directions. The first-release deployment support question remains
unanswered; neither path has been silently selected or removed from scope.

## Scoped AppArmor / private procfs checkpoint

A temporary parent/child AppArmor candidate passed syntax checking and allowed
Bubblewrap namespace/system bind mounts, while keeping the application parent's
mount denial and upstream proc/sys denies. Private procfs mounting still fails
EPERM with standard Docker system-path protections. Exact candidate, hashes and
trace are retained in `docs/evidence/docker-install/`; no supported policy or
container AI readiness is claimed. The test profile was unloaded and checked
absent, caches/mounts/daemon cleaned, production health passed. A different
execution boundary or explicitly designed replacement for container path
protections would need review and full isolation verification. The deployment
support choice remains unanswered and the goal remains active.

## Project/task UI and notification follow-up

Separate user-approved UI work is deployed in9f37463/schema25: compact contextual
project/task headers, result-first single-scroll sessions, shareable session and
Detail URLs, contextual creation, desktop/tablet notification quick view and direct
mobile notification navigation with return-to-origin. Clean fixture tests, exact
asset hash comparison, production read-only responsive/theme checks, actual
rollback/reactivation and retained data/profile/preference checks pass. See
`docs/evidence/project-task-ui/` and `docs/evidence/notification-popover/`.
This does not resolve or silently change the outstanding container AI support
decision; no real task execution/provider inference was performed in this release.
