## Historical checkpoints — not current release status

These dated investigation records preserve failed scenarios, limitations and
superseded checkpoints. Use [the current audit](goal-audit.md) for present findings.

## Goal

User cukup menyampaikan tujuan dan memberi persetujuan; Cuan mengurus pekerjaan
sampai tuntas, termasuk memulihkan gangguan tanpa kehilangan konteks atau
menggandakan pekerjaan.

This audit preserves that end state. A successful retry or green unit test alone
does not establish the whole goal.

## Current checkpoint — 2026-10-21, production workflow release

Production runs `79c04c0371d55a91d8813828302c89ce44d5bc38`/schema25, following
the full workflow release tagged `goal-workflow-20261021`. The full typed coordinator, canonical report
evaluator, same-task interruption recovery and private task artifact delivery are
now deployed. Health and private pre-release database backup checks passed;
no database migration was required. Release activation checks for live GitHub
sign-in processes before switching, to avoid interrupting authorization.

The earlier scoped GitHub CLI release `758f9f3` demonstrated a real device
challenge from the production UI and reload resumption of the same login. That
challenge expired without completed owner consent. The normal UI needs neither
a Cuan OAuth App nor a manually created token. A subsequent owner login is now
saved successfully: authenticated production checks confirm account buchin and
89 readable repositories, including 31 private repositories and private
buchin/cuan. This proves saved connection and private repository listing access;
it does not prove task-level clone/commit/push or authorized continuation.

Read-only verification of the current production release passed at 390, 820 and
1440 pixels: exact assets, existing message rendering and contextual CTA,
composer focus, typing-safe shortcuts and navigation. Post-login writes were
blocked and provider calls were zero. See
[production verification](../evidence/goal-workflow-production/README.md).

Dedicated staging runs `9911437`/schema25 with the typed coordinator protocol,
canonical report evaluator and private task artifacts. Its native distance
converter scenario now passed: structured proposal, zero tasks before approval,
real Approve and start click, exactly one Default task, one successful native
attempt, HTML/README delivered as private Inbox attachments, authenticated browser
downloads, denied anonymous downloads, and automatic canonical report review.
The downloaded HTML was opened in Chromium and five numeric cases, empty state,
mobile overflow, no network requests and no page errors passed. Earlier failed
request records are preserved: native input/finish events initially triggered
false positives; `e914bbe`/`1dd7b42` corrected that classification, retaining
rejection of actual write/terminal tools. See
[controlled native evidence](../evidence/coordinator-native/README.md).

The original interrupted packing task already resumed on its second attempt,
preserving its original HTML hash/inode and original approval. Its structured
report review passed without implementation tools. Same-task interruption,
cancellation, approval, report and artifact boundary tests passed. This remains
separate evidence from the new first-attempt native download scenario.

The full objective remains open: verify task-level GitHub delivery/authorized continuation and fresh-live AI OAuth
onboarding from an empty account, and verify post-login setup through the actual fresh-account authorization flow. A controlled positive scenario alone does
not prove every requirement or erase earlier failed scenarios.

## Completion evidence by requirement

| Requirement | Evidence and limit |
| --- | --- |
| Natural-language goal, approval, one execution and delivered result | Native dedicated staging scenario passed with one task and one attempt; HTML and README downloaded through Inbox, anonymous access denied, downloaded HTML checked in Chromium. |
| Interrupted work retains context and approval | Separate native recovery scenario passed with the original task, approval and artifact retained; this is distinct from the first-attempt delivery scenario. |
| User-visible progress | Start, attachments and final reviewed result captured in the native scenario. Mid-execution progress forwarding has fixture coverage; a captured native intermediate progress report is not claimed. |
| Production readiness | Full backend deployed at fb624e5, schema25 and health passed; read-only responsive UI and exact assets passed. No production inference/replay used. |
| Fresh AI connection | Actual CLI reached Google OAuth from an empty isolated profile without manual theme selection. Full authorization and post-login completion remain unverified; preseeded settings are not claimed to survive the CLI rewrite. |
| GitHub connection | Actual production device challenge, saved owner-authorized connection and private repository listing verified. Task-level repository delivery/continuation remains unverified. |
| Future integration design | Repository AGENTS.md links the approved integration principles: maintained official CLI where appropriate, simple UI, isolated credentials, explicit consent, documented API/SDK alternatives. |

The overall goal remains open. The user confirmed that Google was already
connected on the dedicated installation. Reverification through Cuan's actual
provider-account API on 2026-10-21 succeeded using claude-sonnet-4-6 (see
../evidence/fresh-ai-oauth/existing-account-reverification.json). Existing-account
readiness is therefore verified and is not a user-login blocker. A complete
fresh-profile OAuth recording remains an evidence limitation, not a reason to
ask the user to repeat a working connection. GitHub consent and private repository listing are now verified. Task-level GitHub delivery/continuation remains unverified.

## Earlier checkpoint — native recovery

Production runs `09a3fc4`/schema25; dedicated Dracin staging runs `8b2ed65`/schema25.
Production Default is neutral (name Default, no repository); legacy Cuan data was
preserved in its own project. Private Cuan repository access still needs GitHub
setup; no ambient token was borrowed.

The real Studio proposal was approved through the UI and created one task. A
controlled interruption stopped its first execution. Earlier coordinator review
incorrectly implemented duplicate files in its provider profile; this was failed
acceptance. The corrected orchestration now automatically resumed the same task
using its original approval. The second native attempt succeeded, preserved the
original HTML byte-for-byte and inode-for-inode, and added its missing README.
There were no surviving original processes or duplicate tasks.

Full acceptance remains open: final Inbox coordinator review still attempts a
native terminal tool and stops on permission denial, even with snapshot-only
instructions. User-accessible delivery and independent browser checks of the
artifact remain unproven. Fresh-live OAuth setup from an empty account is also
not established by the existing-account success. Staging health and targeted
tests do not prove these missing outcomes. See
[recovery evidence](../evidence/live-goal-recovery/coordinator-boundary.md).

The checkpoint below and its pending-Default/recovery statements are historical
and superseded by this current checkpoint.

## Earlier checkpoint — 2026-10-21

Production and the dedicated Dracin test installation run `94ff5f3`/schema25.
The owner explicitly accepted provider terms with optional interaction sharing
OFF. The dedicated account completed native onboarding, and actual connection
verification plus a Studio message both succeed using Sonnet. Auto model routing
uses evidenced quota instead of the unavailable implicit Gemini default. Direct
Studio connection, System appearance and the dedicated favicon are deployed.
Production health, exact assets, responsive browser checks and retained data pass.
The earlier consent/location blocker sections below are historical and superseded.

The full objective remains open: fresh live OAuth automation from an empty account
and actual approved implementation with controlled interruption/recovery are not
proven by the successful reply or deterministic fixtures.

Current work also addresses the owner's observed legacy Default project and
project-task UX. Production's `default` project is still named cuan, bound to a
repository, and its Git cache has no commit/ref despite a synced flag. A saved
administrative task could not launch from that cache. Source fixes add explicit
transactional Default separation, historical workspace compatibility, preparation
of missing Git bases before execution, nonduplicated launch failure delivery,
answer/result UX and Inbox Live Chat shortcuts. Real Git, PostgreSQL, targeted
service and responsive browser evidence pass; live activation is pending.
No production task or AI request has been replayed for verification.

## Current requirement audit

| Requirement | Authoritative implementation/evidence | Current finding |
| --- | --- | --- |
| Connect AI from Studio without an agent-name/configuration form | StudioView + TeamView connectionOnly; direct-ai-connect browser evidence | Deployed in 94ff5f3; fixtures retain the draft and require explicit submission. Dedicated-account verification and a real Studio reply succeed; native empty-profile OAuth start is verified, while completed fresh-account authorization and post-login setup remain unproven. |
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
snapshots and choice actions. The current production application is 5882000/schema25. Older checkpoints below are historical; the current checkpoint above takes precedence.

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

## Resumed Docker private-proc investigation

On resumed goal work, an experimental explicit replacement of OCI masked/readonly
path mounts with confined AppArmor path denies permits private procfs on this host.
The boundary fixture exits0: read-only project/attachment, writable selected
profile, hidden unrelated file and outer process environment, self-executable
terminal helper, denied application namespace creation and11 kernel-path denials.
A real parent SIGKILL leaves no sentinel descendants; `--die-with-parent` remains.
Exact policies, worker hashes, fixture issues and limits are in
`docs/evidence/docker-private-proc/`. All temporary profiles, daemons, mounts and
caches were stopped/cleaned; native production0da59d4 stayed healthy.

This supersedes an assumption that private procfs is necessarily unavailable in
all Docker configurations. It does not establish a supported or fully reviewed
replacement policy, actual AGY startup, Cuan container execution/recovery or
cross-host compatibility. There is now a concrete technical path to investigate
without reducing release scope; the earlier deployment-support question is not
being treated as an immediate impediment to this investigation. The original
full goal remains active and unproven.

## Official musl CLI startup in the experimental container boundary

Official AGY1.4.3 installs with SHA-512 verification and runs version/help inside
the private-proc Bubblewrap boundary as non-root, without accounts or inference.
All six Cuan-required flags are present (help writes to stderr). Reinstallation
validates the existing executable. Installer executable hashing now streams1MiB
chunks; a100,662,912-byte fixture matches SHA-512 under64MiB address-space limit.
Container installation and validation pass at128MiB memory limit.

The boundary and real parent-loss checks still pass. All temporary profiles,
daemons, caches and private mounts were cleaned; native production0da59d4 remains
healthy. Exact worker, result and limits: docs/evidence/docker-cli-startup/.
This closes the CLI startup uncertainty, not actual Cuan container execution,
provider tool execution, policy alias/bypass review or supported host setup.
The full goal remains active.

## Experimental Docker path-policy review and payload filter

A real alternate bind-mount probe disproves sufficient isolation from the
experimental AppArmor path denies alone: after a new user/mount namespace,
the payload can open the alias of /proc/keys. No contents were read. Denials of
fresh procfs and nested Bubblewrap had not covered this distinct case.

A post-setup Bubblewrap seccomp filter blocks the exact reproduction at unshare
with EPERM while retaining official CLI startup/help, the boundary fixture and
real parent-loss cleanup. Raw amd64 filter, before/after workers, hashes and
limits: docs/evidence/docker-policy-review/. No profile/no_new_privs guard was
disabled, and all diagnostic resources were cleaned. Native production491b2dd
is unchanged and healthy. Actual Cuan command/filter integration, broad syscall
regression coverage, full container execution/recovery and supported host setup
are still open. The goal remains active; the earlier experimental policy must
not be promoted as a supported configuration on the strength of path tests.

## Native release installer actual-binary follow-up

Actual installer/migrator/launcher with the current production server binary now
passes a fresh disposable PostgreSQL installation, initial owner setup, static
asset readiness, reinstall and process restart. Private configuration, auth
session, renamed project, previous release and workspace are retained.
See docs/evidence/native-release-installer/actual-install.json and the reproducible
scripts/test-native-installer-live.py harness. No production DB writes or AI calls.
This closes the application-install verification gap, not service-manager
provisioning or authenticated AI runtime/container execution. The full goal stays
active and unproven.

## Editable project guidance and durable workspace revisions

Release7b5362d adds editable project name, goal/instructions and optional
repository/branch. Goal/instructions and repository revision are frozen with new
tasks; revisions keep that binding. Connection changes prepare a separate
workspace and retain authenticated old bindings for recovery, never resetting
the prior directory. Atomic versioned saves reject stale changes. Coordinator
context reads project guidance without treating it as expanded authorization.
User-facing task instructions omit the internal binding snapshot.

Targeted real-Git, memory, actual PostgreSQL API/install/restart, browser and
direct/approved process-loss recovery tests pass. Release compatibility gates
prevent activating workspace-unaware binaries after a connection history exists.
Production read-only UI, health, rollback and retention pass without changing
project settings or performing inference. See docs/evidence/project-settings.
The full goal remains active; native service supervision and stock Docker AI
runtime still require separate work and verification.

## 2026-10-21 verification checkpoint

Current production is 0eb1aea/schema25. Native systemd supervision is now verified
with actual binaries and SIGKILL/restart retention (native-service evidence).
Task worktree isolation, safe retention/reopening, scoped Git push and contextual
GitHub continuation are deployed with deterministic process and real-Git evidence.
Integration/OAuth UI and actual rollback/retention checks pass. Real OAuth App
configuration is absent, so production correctly reports Setup needed.

A fresh-install audit found task Git author variables were dropped by the AUTH
substring filter. Empty-HOME actual commits now pass with human owner identity;
the fix is not yet deployed. See task-git-identity evidence. This demonstrates why
passing developer-environment tests alone cannot prove fresh-install delivery.
The full objective is still unproven; no actual-provider/GitHub end-to-end success
is claimed. Experimental Docker isolation work remains stopped.

Production f40909a now includes the human Git identity fix. Actual rollback/reactivation and read-only asset/retention checks pass; all 37 application tables, 930 profile files, seven item states and eleven notification read flags are unchanged. No production inference or commit was used to verify this release. OAuth App registration and actual-provider end-to-end proof remain open.


## 2026-10-21 actual-provider historical completion investigation

Read-only production observation found 22 completed Inbox requests, five
succeeded task attempts, one failed attempt and five agent reports. No done task
with a recorded commit was present. These aggregates establish actual provider
traffic, not end-to-end implementation delivery or interruption recovery.

The blocked “Hello world” attempt finished on 2026-10-19 at 23:39:14 +08:00.
Its activity history contains a required report contract but no agent report.
The saved output (304 characters) diagnoses a headless command permission denial;
the provider nevertheless exited zero. Consequently Cuan correctly withheld
completion instead of claiming that the requested work shipped.

Current task launcher uses the explicitly authorized interactive task mode and
workspace argument. TestS25AgentOutlivesRequestAndPromptIsNotShellCode passed
again on 2026-10-21 (internal/terminal, 0.233s), checking those exact arguments,
request-independent process lifetime and literal prompt handling. This fixture
does not prove authenticated provider behavior. Historical missing-report failure
must not be presented as a reproduced regression in the active release.

No production task was resumed, submitted or modified; no account profile or
credential was borrowed. Actual authenticated model implementation, final report,
scoped Git delivery and interruption recovery remain unproven. Live GitHub OAuth
App configuration remains absent. Keep the full objective open.


## Cancellation is not an interruption to recover automatically

The coordinator review path previously accepted an old approval to revise a
user-cancelled attempt. A new guard rejects that automatic continuation both
when delegation is received and when a persisted launch is drained. A later
accepted user message can request continuation of the original scope; system
reviews and pre-cancellation approvals cannot undo cancellation. Recovery context
keeps saved checkpoints and explicitly carries the cancellation constraint.

Targeted tests cover cancelled-task delegation, queued-launch rejection, unchanged
attempt count, fresh authenticated user turns and rejected unaccepted triggers.
Existing direct/approved process-loss recovery and queued reconstruction tests
remain passing. This changes application behavior; it does not establish actual
provider end-to-end recovery. The guard is deployed as 98f8811; clean-checkout tests, actual rollback/reactivation, read-only production UI/asset checks and retained-data verification pass. See docs/evidence/cancel-recovery.


## Approved repository interruption with pending Git state

`TestApprovedRepositoryRecoveryPreservesPendingGitWork` extends the approved
Inbox process-loss flow with a real repository and task-bound isolated worktree.
Before SIGKILL, the child stages a tracked edit and writes both untracked and
ignored files. The scoped continuation verifies the same workspace path, index
and file contents before finishing. Assertions prove one task, two attempts,
one artifact creation, no duplicate appended edit, unchanged main workspace,
and no extra commit. Existing direct/approved recovery and cancellation tests
pass together (6.992s).

This closes a combined process/worktree pending-state fixture gap, not actual
model reasoning, database reconstruction in this particular test, or real GitHub
delivery. The authenticated provider prerequisite was requested explicitly;
profiles from ambient environments or production must not be silently reused for testing.


## Actual-provider completion gate blocked — 2026-10-21

Revalidated after `ad1f5dc`: this sandbox has no Cuan accounts directory and zero
connected Cuan test accounts. GitHub OAuth App configuration is also absent from
the sandbox; production already reports Setup needed. Read-only production
aggregates remain 22 completed chats, five succeeded attempts, five reports and
zero done tasks with a recorded commit. No new actual-provider delivery or
recovery proof has appeared.

The dedicated test-account prerequisite has remained unresolved across the last
three goal checkpoints. Deterministic coverage and production delivery have been
advanced independently; further fixtures cannot close this external evidence gap.
Completion is not established. Do not run production tasks or borrow another
application's account/profile to manufacture proof.

Unblock by connecting a dedicated AI account to an isolated Cuan installation.
Then verify an actual goal and approval, scoped implementation and verification,
controlled process interruption, continuation of the same task/workspace, exactly
one implementation effect, and a final user-visible result. GitHub authorization
must additionally be configured if that scenario includes remote delivery. The
pending user question requests this account prerequisite; no secrets should be
printed or committed.


## Provider setup consent gate — current blocker

`ddb04ed` pre-login settings preparation passed again in the separate checkout
(0.116s), including an actual child fixture which reads settings before emitting
its Google authorization URL. This verifies settings preparation, not completed
real CLI onboarding.

The latest read-only Dracin check confirms task
`task_b186cfdf7cded14ee1292fff5f1fe546`, attempt
`att_0b0284644f06210d595e06dd5c0ffd51`, remains running. The last inspected live
terminal was waiting at Terms of Service & Data Use after default theme selection.
The owner has not answered the explicit provider-terms question. No Done input,
interaction-sharing acceptance, replay, service restart or rollout has occurred.

This same consent gate has persisted over the initial request and two subsequent
goal checkpoints. Independent source changes and verification are complete for
now. Actual completion-state inspection, safe first-run wizard finalization and
real-model recovery proof require the owner decision before continuing this live
session. Do not infer approval from an automatic goal continuation message.
The full goal is blocked at that prerequisite, not complete. Resume after the
owner accepts or declines the provider terms with a clear sharing preference.

## Repository UX release verification

Production5882000 replaces two legacy PAT-requesting repository errors with
Settings > Integrations guidance. Targeted project-connect, real-Git push
recovery, queued continuation and same-scope recovery tests passed together
(13.332s). The previous20s command was terminated and is not counted as a pass.
Build/deployment health passed, schema25/no migration. Postdeploy read-only
browser checks passed at390/820/1440, exact assets and GitHub connection retention
passed, zero production inference. See
../evidence/goal-workflow-production/repository-copy-release.json.
Real-Git fixtures do not establish native model remote GitHub delivery.

## Native GitHub scenario: real repository path regression

After the user linked staging GitHub and authorized testing buchin/cuan, the
native Studio proposal was approved. Exactly one task was created:
msg_fadb6835-ebfb-4c5e-a787-8c7f37b09768. Its first attempt failed before provider
execution because reporting could not open the worktree. Relative Git worktree
paths were interpreted from the source repository rather than Cuan's working
directory. Commit04af327 uses absolute destinations and verifies/reuses legacy
nested worktrees without losing pending files. Targeted real-Git regression tests
pass; staging was upgraded with backup and retained configuration/readiness.

A subsequent same-conversation continuation failed at the coordinator's native
tool permission boundary; no second task or successful GitHub push is claimed.
This remains agent-fixable work; do not ask the owner to reconnect working accounts.

## User-focused Projects view

User approved settings-first Projects. Production db8f0f6 now opens project
settings by default, retains work under Activity and task/session message links,
and routes Ask Cuan to Studio. Desktop/tablet/mobile read-only checks passed;
see ../evidence/projects-settings-first/README.md. The release also contains the
relative-worktree fix. Native GitHub continuation remains unresolved, so the full
goal is not complete.

Current project navigation follows the subsequent user-approved revision:
Settings/Activity tabs, Settings default, New project on the list only, and no
Ask Cuan/New task on project detail. Production37845e3 and responsive read-only
checks pass; see projects-settings-first/project-tabs.json. Native GitHub
continuation remains unfinished and is not superseded by this UI delivery.

## Contextual creation and storage follow-up

User-approved contextual creation actions share the desktop secondary-list
placement across Projects, Contacts and Inbox, with mobile/tablet header actions.
Production9fecf67 and nine responsive browser checks passed; see
../evidence/list-create-pattern/production.json. A failed upload exposed a full
shared VPS. Corrected Nginx log rotation and release retention restored8.6GB
free without removing application data or backups; see
../operations/storage-retention.md. Native GitHub same-task continuation remains
unfinished; these changes do not establish overall goal completion.

## Compact Studio and conversation context

Production79c04c0 now includes the approved104px auto-growing Studio composer;
responsive height/growth/shrink/draft/CTA checks passed. It also contains407cd86,
which retains a prior same-conversation task project for follow-ups without an
explicit project name and provides original authorization/execution state.
Targeted project scope tests pass in the clean release. Actual native GitHub
continuation has not yet been repeated on that correction; full goal stays open.

## Native model recovery checkpoint

Staging876358e adds installed-CLI catalog filtering, explicit unknown availability
and recognition of native error_message lifecycle events. Actual no-tool quota
failure now safely advances to the next permitted model; native proof is in
../evidence/native-model-routing/fallback.json. The next model failed with the
provider's location restriction. Original approved GitHub task remains intact,
with one task and only the original failed preparation attempt. Sonnet/GPT-OSS
quota and Gemini region restrictions currently prevent complete verification;
a tiny Opus success did not extend to the actual continuation prompt.

Production activation was refused by its guard while task attempt
att_daa1f32faead20bcaf68157842fa9cc7 remained running with a renewing lease.
Production remains79c04c0. No process was stopped and no production inference
was run for verification. Further native work needs a usable dedicated account
route; the user was offered another test account or waiting for quota recovery.
The full goal remains incomplete.

## Terminal input follow-up

User requested input enabled by default and a working mobile keyboard button.
Candidated030a0b uses interactive handshake mode, applies the acknowledged mode
and focuses within the keyboard click. Three-size isolated WebSocket checks pass;
no input reached production. Physical-device keyboard visibility remains pending.
The user screenshot shows the existing production task terminal waiting after a
quota error, despite status Working. This is distinct from a running model call.
Deployment remains guarded while that session is open; a specific approval to
end it was requested after the candidate was built and verified.

## Production activation and remaining native gate (2026-10-21)

After explicit owner approval, the canonical cancel endpoint stopped only attempt
att_daa1f32faead20bcaf68157842fa9cc7. Its task remains saved and blocked, with one
attempt and a cancellation activity. The release gate then reported no active
Inbox requests, attempts or queued launches. Atomic production deployment activated
972b601469c2a68e0e8ec4ada7a8927b860f8da7; /healthz returned healthy and the public
index served index-BKU_NYgx.js containing Show keyboard. Physical mobile keyboard
visibility still needs device confirmation.

The original dedicated GitHub test task remains unchanged: one failed preparation
attempt, no commit or push result. Saved Google credentials remain present. A fresh
usage read returned quota entries including obsolete Gemini IDs; this does not
prove availability of the installed CLI's supported models. Prior native execution
proved quota/location failures, not GitHub delivery. Full goal remains open.

Inspection identified an additional observability gap: taskOutputRecorder stops
saving output after 1000 lines or 256 KiB. Thus later CLI quota prompts can be
visible in the live terminal while absent from retained logs; absence of a quota
message in saved logs is not evidence that the provider is still working. No
unverified automatic termination or task replay was added in this inspection.

## Native read-file replay classification

Read-only inspection of the saved production Gemini transcript for session
a3af9aa9-90b9-4e6f-91d2-605bd8e1827f found two native `read_file` calls, with
no tool arguments or file contents exported. Cuan recognized the older
`view_file` name but treated `read_file` as unknown activity. Runtime parsing now
recognizes both exact read-only names and labels them Read file. Unknown tools,
write_file and run_command still prohibit replay. A controlled CLI failover test
proves native read_file quota failure can reach the permitted backup exactly
once, preserving the agent binding; this is not a successful production replay
or proof of full native GitHub delivery.

## Clear stopped-work and failure outcomes

Stale automatic reviews now consult the linked task within the same room and
assigned agent. A saved cancellation explains that work stopped and waits for
fresh user instruction; other stale updates make no completion/restart claim.
No AI call or execution is introduced for this status explanation. Existing
cancellation tests also verify the outcome and reject foreign room/agent state.
Failed Inbox bubbles with check-status recovery display the server's safe_error
instead of hiding the specific failure behind a generic stopped-reply message.

## Current-release native GitHub continuation

Dedicated staging was upgraded to5e0fe9a with checksum verification, private DB
and state backups, unchanged configuration, schema25 readiness and no migrations.
The first packaging attempt lacked the installer-required cuan-migrate binary and
failed before activation; the complete checked package installed successfully.

One Inbox continuation of the original approved scope produced request
8a840df7-2ca4-4e4d-92ad-5f5d00d76388: Opus quota, Sonnet quota, then Gemini location
restriction. All routes stopped without tool execution. The canonical response
still contains exactly one task and original attempt att_78cde330a57d6e52a8146f6b90849c7c.
No commit, push or successful native delivery is claimed. See
../evidence/native-model-routing/current-release-blocker.json. This is an external
provider prerequisite; do not repeatedly replay the work, reconnect already-saved
Google/GitHub credentials, or replace native delivery with manual developer work.

## Native GitHub delivery independently verified — 2026-10-21

The original approved task is now Done with exactly one task and four attempts;
only the fourth succeeded. Earlier attempts were preparation failure and two
operator-stopped blocked CLI runs. User follow-ups and runtime fixes were involved;
this is not evidence of an entirely unattended four-attempt recovery.

The independent verifier used only the dedicated installation's saved encrypted
Cuan GitHub connection and a live read-only GitHub ref lookup. Remote branch head
matches fab06900da7d12a44b931ccef8b8fffac4a10c53. The existing worktree is clean,
only the approved check.md was committed, whitespace validation passes, required
document topics are present, and the author is the configured human. Evidence:
../evidence/native-github-delivery/independent-proof.json. Full completion still
requires reconciling this result against the goal's complete requirement audit.

Production3258b97 includes the correction-budget fix and message gesture/detail
layout fix. Production-asset browser checks use fixture messages and simulated
touch; no live message was resent. See ../evidence/chat-gesture-detail/browser.json.

## Current recovery verification — 2026-10-21

Production now runs1ccf2f4, healthy with schema25 and public readiness passing.
Both isolated PostgreSQL scenarios passed without skips: approved interrupted
goal recovery (6.05s) and native-warning quota recovery (5.85s). The provider is
a controlled fixture; this does not prove live provider quota availability.
Recovery retains the original approval, task and artifacts, applies permitted
model routing, and respects cancellation and correction bounds. Evidence:
../evidence/durable-goal-recovery/current-provider-recovery.json.

The gesture/Detail UI from3258b97 remains included. The broader goal audit is
still open; this checkpoint does not claim every onboarding/deployment path
is fully verified.
