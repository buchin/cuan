# Architecture Decisions & Threat Boundaries — Sprint 1 (T1)

## 1. Monorepo Structure

Cuan utilizes a standard Go and TypeScript monorepo layout:
- `cmd/server/`: Main HTTP API server entrypoint.
- `cmd/runner/`: Task runner daemon entrypoint.
- `cmd/migrate/`: Database schema migration CLI with dry-run support.
- `internal/agent/`: Official coding agent adapter interface, implementations, secret redactor, and process monitor.
- `internal/config/`: Configuration and environment loader.
- `internal/db/`: Database connectivity, migration loader, and execution runner.
- `internal/runner/`: Task attempt leasing and process lifecycle manager.
- `internal/server/`: HTTP routing (Go `net/http` + `chi`), healthcheck, and REST handlers.
- `migrations/`: SQL migration files (`*.up.sql` and `*.down.sql`) embedded directly into Go binaries.
- `web/`: Frontend SPA (React 18, Vite 5, TypeScript, Tailwind CSS, shadcn/ui components).
- `scripts/`: Dev scripts with environment detection (`dev.sh`, `build.sh`, `test.sh`, `migrate.sh`).
- `Makefile`: Standard development targets (`build`, `test`, `migrate`, `dev`, `clean`).

### Architecture Diagram (D2 + TALA)

```d2
direction: right

client: "Browser / User UI\n(web/ React Vite)" {
  shape: rectangle
}

server: "Cuan API Server\n(cmd/server :8080)" {
  shape: rectangle
}

db: "PostgreSQL Database\n(migrations baseline)" {
  shape: cylinder
}

runner: "Cuan Runner Daemon\n(cmd/runner)" {
  shape: rectangle
}

adapter: "AGY Agent Adapter\n(internal/agent)" {
  shape: rectangle
}

agent_process: "agy CLI Process\n(v1.4.0 Headless)" {
  shape: oval
}

client -> server: "HTTP / REST API"
server -> db: "Read/Write Tasks & Leases"
runner -> db: "Poll / Heartbeat"
runner -> adapter: "Execute Task Attempt"
adapter -> agent_process: "Isolated Process Group\n(SIGINT/SIGKILL Cancelable)"
```

---

## 2. Coding Agent Adapter Selection

### Evaluated Candidates

1. **`agy` (v1.4.0)**:
   - **Auth**: Fully authenticated on server with valid credentials.
   - **Headless Mode**: Native `-p` / `--print` mode with `--dangerously-skip-permissions` for sandbox runs.
   - **Structured Output**: Native `--output-format json` and `stream-json` providing structured conversation ID, status, tokens, and duration.
   - **Cancelability**: Spawns isolated process group; responds immediately to `SIGINT` and `SIGKILL`. Verified in `internal/agent/agent_test.go` with 159ms cancel response.
   - **Result**: **SELECTED** as official coding agent adapter for Sprint 1.

2. **`claude` (Claude Code 2.1.301)**:
   - **Status**: OAuth session expired on host server. Requires interactive web browser re-login, which is unavailable in headless server automation without credential refresh.
   - **Result**: Deferred.

3. **`codex` (Codex CLI 0.163.0)**:
   - **Status**: No credentials found in `~/.codex/auth.json` on host. Missing OpenAI API key / ChatGPT auth.
   - **Result**: Deferred.

4. **`grok` (Grok CLI 1.0.60)**:
   - **Status**: Not signed in (`grok login --device-code` or `XAI_API_KEY` required).
   - **Result**: Deferred.

---

## 3. Threat Boundaries & Security Controls

```d2
direction: down

untrusted: "Untrusted Agent Execution Boundary" {
  agent_sub: "Subprocess Workspace\n(Task Workspace Only)"
  no_docker: "No Docker Socket Access"
  no_infra_creds: "No Global VPS/Cloud Credentials"
}

cuan_core: "Cuan Trusted Boundary" {
  masker: "Secret Masker / Redactor\n(Tokens, Keys, Passwords)"
  db_creds: "Isolated DB Connection"
}

untrusted -> cuan_core: "Logs & Outputs Sanitized ([REDACTED])"
```

1. **Secret Masking & Redaction**:
   - `internal/agent/secret_masker.go` intercepts all stdout and stderr streams before logging or persistence.
   - Any injected tokens, passwords, or API keys are replaced with `[REDACTED]`.
   - Verified in automated tests (`TestMaskSecrets` and `TestAGYAdapter_SpikeFixtureEditAndSecrets`).

2. **Subprocess Isolation**:
   - The runner executes tasks in designated task workspaces.
   - The agent process does not have access to the Docker socket or global infrastructure credentials.
   - Process groups (`Setpgid: true`) ensure that canceling an attempt terminates the entire process tree without leaving orphaned background processes.

3. **Database Exposure**:
   - PostgreSQL is strictly internal; migrations and application logic connect via standard least-privilege credentials.

---

## 4. License Decision

- **License**: Apache-2.0.
- **Copyright**: 2026 Buchin and Cuan contributors.
- **Files**: `LICENSE` and `NOTICE` are placed at the root of the repository.
