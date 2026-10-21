## Objective

A user states a goal, connects AI without extra agent/project configuration
forms, and approves when needed. Cuan manages execution, progress, verification
and delivery, recovering interruptions without losing context or duplicating
work. Ordinary questions and technical analysis remain conversational.

## Current release and audit status

Production and dedicated staging run `ec0628f54507322123f40bf1635491517c2eef46`,
schema25. Both activated through checksummed native releases with private backups,
configuration preservation and readiness checks. No migration was needed.
Unrelated working-tree changes were excluded from release builds.

The full objective is **not yet marked complete**. Core implementation and the
checks below pass; the original interrupted packing-checklist test still needs
its existing artifacts delivered as downloadable attachments and verified in a
browser. A same-task, original-scope delivery correction is being observed.
Earlier failed requests and operator interventions remain part of the record.

## Requirements and authoritative evidence

| Requirement | Evidence inspected | Finding |
| --- | --- | --- |
| State a goal without choosing an agent, project or task configuration | `web/src/features/inbox/StudioView.tsx`, TeamView connectionOnly, direct-ai-connect/current-browser.json | Current frontend fixtures retain the draft, connect directly, keep project optional, submit once after explicit action, and handle account-fetch/setup errors. Native Studio proposal/delivery also passed. |
| Complete Google AI connection without manual terminal/theme setup | TeamView saved-code recovery, account handlers, account-profile wizard/PTY implementation; saved-google-setup/README.md | Actual new dev2 owner authorization was saved; an unsized-PTY bug was fixed and the same saved account completed the native provider wizard with explicit terms and optional sharing off. Native verification succeeded. This is actual authorization plus repaired setup, not a claim of an uninterrupted fresh OAuth recording. |
| Preserve the user's authorization and execute once | Persisted approval/scope digest and managed launch guards; coordinator-native/native-delivery.json; fresh-install SQL scenarios | A real proposal existed before implementation; actual Approve and start created one task with its saved scope. SQL tests reject duplicate execution and retain approval through reconstruction. Clear direct instructions remain authorized without a separate planning form. |
| Perform work and deliver usable results | coordinator-native downloads/browser-verification.json; native-github-delivery/independent-proof.json | Actual HTML/README downloaded through Inbox and checked in Chromium; anonymous downloads denied. A separate actual GitHub branch/commit was independently checked using dedicated Cuan credentials and the configured human identity. The GitHub scenario required earlier operator follow-ups, not entirely unattended recovery. |
| Review evidence and request concrete in-scope corrections | Canonical report evaluator and coordinator contracts; native final review records; targeted evaluator/delegation tests | Current reports are compared with the original criteria. Missing/failed evidence cannot become completion solely from exit0. Results distinguish developer-reported checks from independently observed checks. Review cannot execute work itself. |
| Preserve context and existing artifacts after interruption | live-goal-recovery/coordinator-boundary.md; durable-goal-recovery/current-provider-recovery.json; task-recovery/README.md | Original native task resumed automatically after restart with the same HTML hash/inode and original approval. Current SQL reconstruction tests prove one task, two attempts, one artifact creation and one final outcome with progress-only context retained. Original native task download delivery is the remaining follow-up below. |
| Avoid overlapping/repeating execution | Supervisor ownership/process-loss checks, idempotent launch/approval identity, cancellation guards and correction budget | Real child-process and supervisor tests cover ownership and parent loss; current recovery excludes cancellation and preserves existing work. Uncertain external effects must be reconciled before repeating them. Arbitrary external effects are not claimed to be exactly-once. |
| Recover provider quota/location failures through permitted routes | Native task warning stop/cooldowns, account/model policy, current-provider-recovery.json and chat-permission-diagnosis/native-analysis.json | SQL quota recovery passed. Actual native analysis automatically recovered an empty reply and routed around account/model restrictions to a successful file read without creating a task. Auto follows the provider default; alternate models/accounts require previously granted permissions. |
| Do not let a slow primary prevent checking a healthy backup | `internal/server/inbox_failover.go`, TestInboxSlowPrimaryDoesNotStarveAllowedBackupProbe and targeted failover tests | Current release checks at most two eligible account usage/catalog probes concurrently within the existing preflight bound. Results remain deterministically ranked and all credentials/permissions are revalidated before execution. |
| Technical analysis works directly in Inbox | Coordinator read/search contract, bounded analysis-only recovery, chat-permission-diagnosis evidence | Actual native go.mod read returned module/version independently matched against the file. Denied-tool recovery is verified with real child fixtures, including repeat denial, unsafe action/tool, malformed stream and failed process. That native success did not exercise the denied-tool recovery branch. |
| Keep first use and daily operation self-contained | README; onboarding/native-runtime guides; onboarding, native-release-installer and docker-install evidence | Actual native installation, owner setup, neutral Default, reinstall/restart and retained session/project are verified. Docker application lifecycle is verified separately; stock Docker AGY execution remains unsupported. Native Linux is the supported AI runtime. |
| Keep credentials isolated and obtain provider consent | AGENTS.md/integration-principles; provider account profiles, explicit terms checks and dedicated GitHub connection evidence | Cuan-owned profiles and saved encrypted integration credentials were used. Fural/ambient developer credentials were not substituted for Cuan accounts. |

Evidence paths in this table are relative to `docs/evidence/` unless a source path
or test name is given. Historical investigations are retained in
[goal-audit-history.md](goal-audit-history.md); their old release/blocker statements
are not the current state.

## Latest targeted checks

- PostgreSQL approved-goal interruption recovery: PASS, no skip,6.05s on1ccf2f4.
- PostgreSQL native-shaped quota-stop recovery: PASS, no skip,5.85s on1ccf2f4.
- Direct PostgreSQL first-goal lifecycle: PASS, no skip,3.02s on1469c31.
- Current denied-tool/schema/trace tests: PASS onb85d444.
- Current slow-primary, quota/model routing, account preference, regional fallback
  and analysis-recovery tests: PASS onec0628f,2.999s.
- Actual native read-only analysis and permitted route recovery: PASS onb85d444;
  no task, independently matched go.mod and no production message replay.
- Production/staging native activation: PASS onec0628f with schema25/config retained.

## Remaining acceptance work

1. Finish user-accessible download delivery for the original interrupted
   packing-checklist task, preserving its existing HTML/README and task identity.
2. Independently download and exercise those original files in Chromium, rather
   than accepting a server-local file path or a developer report as delivery.
3. Reconcile the final result against this table and the current release, then
   close the goal only if every required outcome is proved.

The audit does not require provider quota to be unlimited or a user to repeat a
working Google connection. Local cooldown expiry is not evidence of quota reset.
When no authorized route is available, preserve approved work, report the actual
limitation and offer a concrete recovery option. Experimental container policies,
arbitrary provider availability and arbitrary external-effect atomicity remain
separate limitations, not silently claimed capabilities.
