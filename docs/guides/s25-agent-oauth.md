# Antigravity OAuth checkpoint (Sprint 25)

This implementation is available on S25.1 staging at https://cuan-s25-dojo.fural.space/team; it is not released to production. S25.1 requires account-owner consent and real persisted provider output before later backlog/release gates can open.

## Usage

Open Team → Add Agent → Antigravity. The dialog checks server credentials and probes the actual provider. Credentials on disk alone do not imply a verified connection. Click Login dengan Google, open the official link, complete Google consent, and paste the authorization code into the password field. Click Verifikasi login. Registration is enabled only after verification succeeds. Cancel closes the server login; abandoned sessions expire after five minutes. The dialog explicitly expires stale sessions; restarting login cancels the old session and starts a new one. Invalid/expired codes require a new login. The simplified Add Agent dialog exposes Antigravity only; it does not offer unimplemented login methods.

## Server configuration

Install `agy` on the service user's PATH. Login and execution use SSH hints to select CLI headless file-backed storage. Run the service as the intended credential owner; do not copy another user's credentials. The canonical store is `~/.gemini/antigravity-cli/antigravity-oauth-token`. Existing resolver overrides `ANTIGRAVITY_TOKEN_PATH`/`ANTIGRAVITY_CONFIG_DIR` indicate credential discovery only; the CLI must itself use the same credential home. Never log or publish URL query/state, authorization codes or token file contents.

`GET /api/v1/agents/auth-status?harness=agy` reports credential presence and `verified:false`. Add `verify=true` for an actual bounded provider probe. `POST /api/v1/agents/auth-init` starts interactive login; `/auth-code` accepts `{session_id,code}`. POST `/auth-code?cancel=true` with `{session_id,code:""}` cancels. JSON token payloads are rejected. Provider probe uses a short no-tools prompt and may consume provider quota. No new database migration is needed.

## Targeted verification and pending release gate

Run the targeted checks recorded under `docs/sprints/evidence/S25/`. `CUAN_VERIFY_REAL_OAUTH=1 go test ./internal/agent -run TestS25RealOAuthURL -v` verifies official URL extraction without completing login; it cancels the process and does not log authorization material. This test must run only in a controlled environment without an owner's concurrent login.

After owner consent, verify code exchange and a fresh real provider response in Inbox; refresh and confirm that response persists. A provider failure returns 502, keeps the user message and creates no canned BYO reply. Built-in fake agents and canned greetings/triage replies have been removed.

The S25.1 OAuth staging environment runs on port 8081 with an isolated PostgreSQL clone (19 migrations, auto-migrate disabled). The complete S25.4 drill, production release, health/smoke/rollback drill and production database validation remain gated by S25.1→S25.3. No release is authorized by this checkpoint alone. Keep the existing production release until those gates pass; no rollback is needed because this checkpoint has not deployed. Complete the repeatable release and rollback runbook in S25.4 before shipping.

## Runtime and staging preparation

`python3 scripts/install-antigravity.py --help` describes the official CLI installer. It verifies the manifest SHA-512 before atomically installing the executable under the current user's `.local/bin`; Alpine selects the official musl build. Add that directory to the production service PATH during the supported release step.

Run `scripts/prepare-s25-db.py` on the VPS as the Cuan service user with sudo access to PostgreSQL. It clones the current DB into `cuan_s25` under a separate role and writes connection details privately under `~/.local/share/cuan-s25/`. It reuses a completed receipt, never recreates a working staging DB, and removes its temporary dump after restore. No production migrations/data are changed by preparation. An interrupted preparation without a receipt requires inspecting the staging role/database before retrying; do not drop production resources.

The managed sandbox uses `scripts/boot-s25-staging.sh` and `scripts/run-s25-staging.py`: build the server with bounded Go concurrency, install the correct CLI, load private staging config outside the repository, open the restricted SSH database tunnel, then run the actual backend with the built frontend. Publish source (non-test Go + migrations + go.mod/go.sum + built web/dist + boot scripts) with `fural-agent sandboxes serve --name cuan-s25 --dir SOURCE_DIR --cmd 'bash scripts/boot-s25-staging.sh' --port 8081`. Source must fit the supported serve limits; do not publish bin/, node_modules, .env, or tokens. First boot may take longer than the request timeout; inspect the existing sandbox and boot log before retrying. No second sandbox/session is needed.

Private staging config consists of database.json, ssh.json, a dedicated tunnel-key and pinned known_hosts. The boot script installs these with mode 0600 and removes the incoming private config directory. The SSH key is restricted on the VPS to forwarding PostgreSQL, with shell commands disabled. Rotate/revoke that dedicated key and remove the isolated DB/role after staging is retired; never remove the production DB. Preserve the custom sandbox home while login is in progress because it contains the account's CLI credential store. Do not restart/publish a new boot during a pending consent flow.

Owner workflow: open staging → log into Cuan → Team → Tambah agen → Login dengan Google → complete Google consent → paste the code into Cuan → Verifikasi login → Tambah agen. The agent then appears in Inbox from the persisted registry. A registered row is labeled Terdaftar, not automatically Online/Verified; actual chat output comes exclusively from the harness/provider. There are no seeded agents, fabricated token quotas or role cards.

Registration creates no artificial welcome reply. It must save the Inbox room successfully; otherwise the new agent/connection is rolled back. CLI connection linkage survives SQL reads without changing the existing 19-migration schema.

## If an earlier code timed out

The previous backend used a newline instead of the CLI's terminal Enter key, leaving even a valid code waiting unsubmitted. The staging fix sends the correct Enter key and distinguishes a definite Google rejection from an unconfirmed timeout. Refresh the page, start a new Google login and use the new code inside Cuan. An old code belongs to its old login session and must not be reused. Never paste authorization codes into chat or support evidence.
