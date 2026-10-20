# Install and start using Cuan

Cuan is a self-hosted, single-owner project manager. Start in Studio with what you
want done. The coordinator keeps the conversation in Inbox and creates execution
work when needed. You do not need to choose an agent or fill in a task form.

## Choose a deployment path

Native Linux is the verified path for AGY chat and task execution. Docker Compose
starts the web application and PostgreSQL; the stock image does **not** include
an authenticated AI CLI or a verified nested AGY sandbox. Application health does
not prove agent readiness. See [sandbox prerequisites](chat-sandbox.md).

Allow at least 2 GB RAM for the application/build, with additional capacity for
concurrent AI processes. Use persistent storage, Git, PostgreSQL 16, and HTTPS for
remote access. Source builds require the Go version in `go.mod` (currently 1.27)
and Node.js 24 with npm. Check tool/image availability before choosing a build host.

## Docker application installation

Install Docker Engine and Compose v2 first. Clone the repository, then inspect the
plan before applying it:

```sh
git clone https://github.com/buchin/cuan.git
cd cuan
./scripts/install.sh --dry-run
./scripts/install.sh --existing-proxy
```

For Caddy-managed HTTPS, point DNS to the host and make ports 80/443 available:

```sh
./scripts/install.sh --standalone --domain cuan.example.com --email admin@example.com
```

The installer creates a private `.env` with random secrets, preserves existing
credentials, builds services, and checks both health and database readiness.
`--skip-preflight` skips hardware/port detection, not Docker/readiness validation.
Use `--port 9080` for a different host application port. An existing proxy should
forward to that port and support streaming responses. Compose publishes the app
port on host interfaces; restrict access using your host firewall when placing it
behind a proxy. PostgreSQL is not published.

For an isolated staging application, use `--staging --existing-proxy --port 9081`.
Staging Caddy uses host ports 8082/8443 by default rather than production 80/443;
set `STAGING_HTTP_PORT` and `STAGING_HTTPS_PORT` in `.env.staging` if needed.

The database, `/app/data`, and `/home/cuan` use named volumes. Keep all three:
HOME contains account profiles, while data contains application files/workspaces.
Do not use `docker compose down -v` on an installation you want to retain. Before
upgrading an older container without these mounts, stop work and export its data
and HOME first; adding an empty volume does not recover a removed container.

## Native application setup

Run the application under a dedicated Linux user with a persistent HOME and a
writable working directory. Build from a trusted checkout:

```sh
npm --prefix web ci
npm --prefix web run build
mkdir -p bin data
go build -o bin/cuan-server ./cmd/server
go build -o bin/cuan-migrate ./cmd/migrate
```

Provision PostgreSQL and configure the service environment privately:

```sh
export DATABASE_URL='postgres://USER:PASSWORD@127.0.0.1:5432/cuan?sslmode=disable'
export CUAN_AUTH_SECRET='REPLACE_WITH_A_RANDOM_SECRET_OF_AT_LEAST_32_CHARACTERS'
export CUAN_STATIC_DIR="$PWD/web/dist"
export CUAN_AUTO_MIGRATE=true
export PORT=8080
./bin/cuan-server
```

Replace both examples with real values; generate the auth secret with
`openssl rand -hex 32`. Use a protected service environment file for persistent
operation, not shell history. The binary does not automatically load `.env`.
Keep a stable service HOME, working directory, and PATH across restarts. Put the
process under your host's service manager and reverse proxy before unattended use.
For controlled production upgrades, run migrations separately, then start with
`CUAN_AUTO_MIGRATE=false`; follow the [deployment and rollback guide](deployment_recovery_guide.md).

## Connect an agent and verify readiness

1. Open Cuan and create the initial owner account. Protect first setup from public
   access until the owner exists.
2. Studio shows **Connect your AI** when no agent is available. It opens the
   connection flow and returns you to your saved goal when the agent is added.
   Install the provider CLI under the same Linux user/HOME that runs Cuan.
   For native AGY, run
   `sudo python3 scripts/setup-agent-runtime.py --user cuan`, replacing `cuan`
   with the actual non-root service user. This sets up the verified official CLI
   and sandbox together. Ensure `$HOME/.local/bin` is on the service PATH. Complete Google sign-in inside **Settings → Agents** so the
   account uses its dedicated persistent profile. Retain profiles with backups.
3. Verify with `sudo python3 scripts/setup-agent-runtime.py --user cuan --check`.
   This checks CLI startup inside isolation without accessing accounts or using
   provider quota. It does not establish authentication or task readiness. See the
   [sandbox guide](chat-sandbox.md) for PATH, isolation, and container limits.
4. Check `/healthz` and `/readyz`. Then send one harmless informational message in
   Studio and inspect **Detail** to confirm the provider can return a response.
5. Give a small, reversible work instruction. Confirm progress, evidence, and the
   final result in Inbox. A successful chat alone does not prove task execution.

A separate legacy runner is not required for the integrated local-session path.
Repository and deployment access are separate prerequisites for work that needs
them. Configure those for the intended project rather than granting broad access.

## First daily workflow

After connecting AI, Studio offers **Connect a project (optional)** for a first
project. You can skip it and let Cuan organize the work. Connecting accounts or
projects does not itself start execution; submit the goal when you are ready.

Write your desired outcome in Studio. Cuan routes the conversation and organizes
project/task details. Clear instructions can start directly. When a material choice
is unresolved, the agent asks for that choice; an exact-scope proposal can show
**Approve and start**. Planning is not a mandatory extra step.

Follow progress in Inbox and open the task/session from its message when needed.
See the [workspace guide](user_guide.md), [recovery behavior](message_task_lifecycle.md),
and [troubleshooting](troubleshooting.md).
