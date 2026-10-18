# Antigravity OAuth checkpoint (Sprint 25)

This implementation is not released. S25.1 requires account-owner consent and real persisted provider output before later backlog/release gates can open.

## Usage

Open Team → Add Agent → Antigravity. The dialog checks server credentials and probes the actual provider. Credentials on disk alone do not imply a verified connection. Click Login dengan Google, open the official link, complete Google consent, and paste the authorization code into the password field. Click Verifikasi kode dan koneksi. Registration is enabled only after verification succeeds. Cancel closes the server login; abandoned sessions expire after five minutes. Invalid/expired codes require a new login. Other harness auth is explicitly unverified by this integration.

## Server configuration

Install `agy` on the service user's PATH. Login and execution use SSH hints to select CLI headless file-backed storage. Run the service as the intended credential owner; do not copy another user's credentials. The canonical store is `~/.gemini/antigravity-cli/antigravity-oauth-token`. Existing resolver overrides `ANTIGRAVITY_TOKEN_PATH`/`ANTIGRAVITY_CONFIG_DIR` indicate credential discovery only; the CLI must itself use the same credential home. Never log or publish URL query/state, authorization codes or token file contents.

`GET /api/v1/agents/auth-status?harness=agy` reports credential presence and `verified:false`. Add `verify=true` for an actual bounded provider probe. `POST /api/v1/agents/auth-init` starts interactive login; `/auth-code` accepts `{session_id,code}`. POST `/auth-code?cancel=true` with `{session_id,code:""}` cancels. JSON token payloads are rejected. Provider probe uses a short no-tools prompt and may consume provider quota. No new database migration is needed.

## Targeted verification and pending release gate

Run the targeted checks recorded under `docs/sprints/evidence/S25/`. `CUAN_VERIFY_REAL_OAUTH=1 go test ./internal/agent -run TestS25RealOAuthURL -v` verifies official URL extraction without completing login; it cancels the process and does not log authorization material. This test must run only in a controlled environment without an owner's concurrent login.

After owner consent, verify code exchange and a fresh real provider response in Inbox; refresh and confirm that response persists. A provider failure returns 502, keeps the user message and creates no canned BYO reply. Do not treat legacy built-in Scrum Master greeting/triage content as provider verification.

Staging port 8081, production release, health/smoke/rollback drill and production database validation remain gated by S25.1→S25.3. No release is authorized by this checkpoint alone. Keep the existing production release until those gates pass; no rollback is needed because this checkpoint has not deployed. Complete the repeatable release and rollback runbook in S25.4 before shipping.
