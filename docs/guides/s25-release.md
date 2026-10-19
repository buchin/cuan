# Sprint 25 usage and native release

## Agent and task workflow

Open Team → Tambah agen, choose a handle, complete Google login and verify the code inside Cuan. No seeded agents, invented quotas or canned replies are used. Agents registered after a real provider probe appear in Inbox. Credentials stay in the service user's CLI store; an account-backed row alone does not prove a live connection. Inbox reports provider errors and keeps user messages.

Tasks uses a responsive table with All/Todo/Doing/Review/Done/Blocked, project filtering and search. Legacy backlog is displayed under Todo without changing its stored value. Open a title to show Overview, Agent Terminal and Discussion. Save metadata, advance Todo → Doing → Review → Done, or block the task. Comments are persisted using the existing task activity table. `/tasks/:taskId` supports refresh, Back and Forward; missing IDs display an error.

Select a registered agent before Run. Each click starts one actual PTY process, independent of the initiating HTTP request and bounded to ten minutes. Output is streamed with authenticated WebSocket and initial scrollback; do not treat interface banners as agent output. Prompt text is passed literally as argv, never evaluated by a shell. On successful exit, the task moves to Review. Terminal view watches output without exposing the session token in the URL. Runtime run-history is in memory and resets on a service restart; comments, statuses and Inbox messages remain in PostgreSQL.

## Configuration

Keep `/opt/cuan/.env` private and retained. Production service user is ubuntu. The release drop-in adds `/home/ubuntu/.local/bin` to PATH, fixes static files to the active release, and sets `CUAN_AUTO_MIGRATE=false`. Install the official CLI with `python3 scripts/install-antigravity.py`; SHA-512 is checked. The supported native release requires Go, npm, sshpass and the existing `DOJO_TERMAPI_VPS_*` environment settings. Secrets never belong in the repository or command output.

Staging runs on port 8081 at https://cuan-s25-dojo.fural.space with an isolated database clone and separate DB role. Production remains https://cuan.dojo.cc on internal port 8080. Exactly 19 existing migrations are retained. No migration or destructive database operation is part of this release.

## Deploy

1. Finish targeted tests and `npm run build --prefix web`. Verify the actual provider/Inbox and task/PTY evidence under `docs/sprints/evidence/S25/`.
2. Commit and push the tested feature, merge into main with the configured human author, and require a clean working tree.
3. Run `fural-agent repos ship RPcd484fc4229ae192d17a0f01 --mode script --script scripts/fural-deploy.sh`. The same native mechanism is available as `scripts/fural-deploy.sh --help`.
4. The script builds a static server and frontend; uploads only a versioned payload by SCP and synchronizes it with native VPS rsync; verifies its SHA-256 manifest; takes an exclusive host release lock and private `pg_dump` backup; retains the previous release; atomically switches `/opt/cuan/current`; and restarts systemd. It polls health/readiness and requires migration count 19. A failed gate restores the previous pointer and restarts automatically.
5. Verify public `/healthz` and `/readyz`, active frontend bundle and `/tasks/:id` SPA routing, authenticated OAuth/provider probe, real agent registration and a real Inbox reply retrievable after refresh/restart. A Git push alone is not deployment.

The first native atomic release snapshots the previous installation into `releases/legacy-before-s25`. Shared data/sponsors remain under `/opt/cuan/`; each release links to them. Release directories never include .env or account tokens. A pre-release database dump is stored privately under `/opt/cuan/backups/<SHA>.dump` and is not downloadable from the web application.

## Rollback

On the VPS, run `sudo python3 /home/ubuntu/.local/share/cuan-s25/activate-release.py --rollback`. It swaps current/previous under the same lock and verifies health/readiness. To reactivate a retained release, use `--release <full SHA>`. Verify `/healthz`, `/readyz` and the served bundle after every switch.

This release has no schema migration, so normal rollback switches application code only and preserves writes made after release. Never restore a stale DB backup merely to roll back code. Database disaster recovery is a separate approved operation requiring write quiescence and validation; private pg_dump backups are retained for that contingency.

A controlled rollback drill may switch to the retained prior version and immediately reactivate the candidate, with health and 19-migration gates at both steps. Record actual results rather than claiming rollback from a script review.

## Same-owner OAuth continuity

The owner completed consent in staging. For this release, the same account's CLI credential can be migrated privately to the production service user through a one-time forced SSH command (`scripts/import-oauth.py`), with no token in arguments, logs, artifacts or repository. Preserve the old production file privately, then revoke/delete the one-time key immediately and prove a real provider response on the VPS. This mechanism is not for copying another user's account. The app can also perform a fresh Google login directly in production.

The retained private systemd EnvironmentFile takes precedence over Environment directives. Activation therefore writes non-secret `/opt/cuan/release.env` and loads it after `.env` to select the active static payload and disable migrations. Health gates compare the actual `/tasks` HTML with the selected release index, so a healthy API serving stale frontend fails activation.

Fural's ship process may not inherit injected AgentSession secrets. If it fails with an unset DOJO_TERMAPI variable before uploading, run the same checked-in `scripts/fural-deploy.sh` directly from the authorized AgentSession environment. Do not copy credentials into the repository or shared runtime. Upload uses SCP with the injected service-host credential; native VPS rsync performs the final synchronization.

## Verified S25 release

Production release: `7f1bdcd679ebaec063c16d0c936ce14e987f162f` on main. See `docs/sprints/evidence/S25/final-release.json`, `final-rollback-drill.txt`, `production-browser.json` and `production-smoke.json`. Final documentation-only commits can follow this deployed application SHA. Actual rollback and reactivation were exercised. Provider quota can intermittently reject a live request with HTTP 429 even when credentials remain valid; wait for provider recovery or use a supported account/model choice rather than repeating Google consent solely on this error.

## Mobile Inbox correction

Inbox uses one viewport owner and a separately scrolling message stream. On mobile, the shell follows `visualViewport.height` and `offsetTop` for keyboard resize/pan; it falls back to dynamic viewport height. The composer reserves only `max(12px, safe-area-inset-bottom)` and the page declares `viewport-fit=cover`. Portrait inputs use 16px text to avoid iOS focus zoom. Short landscape views reduce message padding, and long drafts scroll inside a capped textarea. Use Back to return to contacts or the chat menu to open navigation. Polling preserves your reading position when no new message arrives.

Verification matrix and screenshots: `docs/sprints/evidence/S25/mobile/`. `verification.cjs` accepts before/after/staging/production; run from the repository root with an existing Playwright installation (`PLAYWRIGHT_MODULE`), optional `CHROMIUM_EXECUTABLE`, and private `CUAN_E2E_TOKEN_FILE` for the selected environment. Before requires `CUAN_BEFORE_DIST`. It reads existing conversation data and types unsent drafts; it does not submit messages or consume provider quota. Keyboard-open screenshots emulate a focused reduced viewport; physical Android/iOS keyboard, browser chrome and OS insets are not certified. The original PO attachment is the visual reference; its exact closed-keyboard gap was not reproducible on headless Chromium. Additional synthetic viewport offset and 34px inset tests exercise the adjustment logic, not physical devices.

Adding an agent currently verifies the service's existing Google account. If that account is quota-limited, verification and registration remain unavailable. Retrying Google consent does not increase quota. Selecting a different Google account is not implemented; this product gap must not be described as completed account switching.

Mobile correction production application SHA: `fb83e133a6ed7babc1ff603568b1e03897139035`. Retained previous release: `db4382f1a606cadd3bf9c15521bec3b566221e68`. Both directions were actually exercised. See `docs/sprints/evidence/S25/mobile/README.md` for screenshots, measurements, unchanged table hashes and precise emulation limits.
