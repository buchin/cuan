# Cuan

> Agentic project management for self-hosted teams: connect a repository, assign work to an agent on your server, and review changes with verification evidence.

## Status

**Sprint 1: Running** (Approved by PO on 2026-10-16).
Status: **Pre-release (Pre-alpha)**. Monorepo foundation, official agent adapter, baseline migrations, persistent PostgreSQL installation, single-owner authentication, and GitHub existing repository integration (T3) are implemented.

Single Source of Truth (SSOT): [docs/sprints/S1.md](docs/sprints/S1.md).

---

## Architecture Overview

Cuan is organized as a Go and React TypeScript monorepo:

- **Backend API (`cmd/server`, `internal/server`)**:
  Go `net/http` + `chi` router providing `/healthz`, `/readyz`, `/api/v1/info`, authentication, projects, and task management APIs. Serves the production React SPA frontend directly at root `/`.
- **GitHub & Git Engine (`internal/git`)**:
  Pre-flight validation of repository existence, remote branch inspection, clone/fetch workspace management, and automatic secret masking on git process outputs.
- **Projects Management (`internal/project`)**:
  Project connection lifecycle, remote metadata synchronization, branch listing, and non-destructive project deletion (removes Cuan records while keeping remote GitHub repositories intact).
- **Credential Security & Encryption (`internal/crypto`)**:
  Industry-standard **AES-256-GCM** at-rest encryption for GitHub Personal Access Tokens (PATs). Zero plain text exposure in API responses and zero credentials in logs.
- **Single-Owner Authentication (`internal/auth`)**:
  Secure single-owner system with bcrypt password hashing, cryptographic session tokens, HttpOnly cookies, and strict takeover protection (subsequent setup attempts return `403 Forbidden`).
- **Database & Persistent Storage (`internal/db`, `migrations/`, Docker Volume)**:
  PostgreSQL 16 with versioned embedded migrations (`000001_baseline`, `000002_owner_sessions`, `000003_projects_github`). Auto-migration executes safely on server boot. Data is persisted in named Docker volume `cuan_postgres_data`.
- **Network Isolation & Security**:
  PostgreSQL port 5432 is **strictly internal** to the Docker container network (`cuan-network`) and is never published to public host ports.
- **Task Runner Daemon (`cmd/runner`, `internal/runner`)**:
  Go worker daemon handling execution leases, agent subprocess lifecycle, log streaming, and process cancellation.
- **Coding Agent Adapter (`internal/agent`)**:
  Official adapter for headless coding agents (selected: `agy` v1.4.0) with process group cancellation and automatic secret redaction.
- **Frontend UI (`web/`)**:
  React 18 + Vite + TypeScript + Tailwind CSS with shadcn/ui components (`web/src/components/ui/`), featuring First-Time Setup Wizard, Login Screen, Projects & Project Detail Management, and Tasks Board.
- **HTTPS Reverse Proxy (`Caddyfile`, `docker-compose.yml`)**:
  Zero-config automatic HTTPS via Caddy (Let's Encrypt) or flexible integration with existing host reverse proxies (Nginx / Traefik).


---

## Clean Environment Installation (Docker Compose)

The recommended way to deploy Cuan on a VPS or clean Linux server is via Docker Compose.

### Prerequisites

- Docker Engine 24+ and Docker Compose v2+
- DNS A-record pointing to your server IP (for public HTTPS)

### Step 1: Clone Repository & Configure Environment

```bash
git clone https://github.com/buchin/cuan.git
cd cuan

# Copy environment template
cp .env.example .env
```

Edit `.env` to configure your settings (passwords, domain, and secret key):

```bash
# Set your domain and notification email for Let's Encrypt
CUAN_DOMAIN=cuan.example.com
ACME_EMAIL=admin@example.com

# Set secure database and authentication secrets
POSTGRES_PASSWORD=your_strong_random_database_password
CUAN_AUTH_SECRET=your_strong_random_32_character_secret_key
```

> **Note**: `.env.example` contains sanitized placeholders only. Real secrets are never committed to version control.

---

### Step 2: Start Cuan Services

#### Mode A: Standalone VPS with Automatic HTTPS (Caddy)

If your server does not have an existing web server on ports 80/443:

```bash
docker compose --profile proxy up -d
```

Caddy will automatically obtain TLS certificates from Let's Encrypt and forward HTTPS traffic to the Cuan container.

#### Mode B: Behind Existing Reverse Proxy (Nginx / Traefik / Cloudflare)

If you already have Nginx or another reverse proxy managing ports 80/443:

```bash
docker compose up -d
```

The Cuan application server will listen on `http://127.0.0.1:8080`. Point your reverse proxy to this port. See [docs/sprints/evidence/S1/https_proxy_guide.md](docs/sprints/evidence/S1/https_proxy_guide.md) for full Nginx and Traefik configuration examples.

---

### Step 3: First-Time Single-Owner Setup

1. Open your browser and navigate to `https://cuan.example.com` (or `http://localhost:8080` for local testing).
2. The UI shell detects that no owner exists and displays the **First-Time Owner Setup Wizard**.
3. Enter your email and a strong password (minimum 8 characters).
4. Click **Inisialisasi & Amankan Instance**.

> **Crucial Security Gate**: Once the owner account is established, public registration is **permanently locked**. Any subsequent setup attempts (by malicious third parties or public visitors) will be rejected with `403 Forbidden`.

---

### Step 4: Verification of Data Persistence across Container Restart

PostgreSQL data is stored in the Docker volume `cuan_postgres_data`. To verify persistence across container restarts or host updates:

```bash
# Restart containers
docker compose restart

# Or recreate containers completely:
docker compose down
docker compose up -d
```

Open the web UI:
- Your owner account, active sessions, and sprint tasks remain intact.
- No re-initialization is required.
- The setup endpoint remains permanently locked (`403 Forbidden`).

---

## Health & Readiness Probes

Cuan provides standard health endpoints for container orchestrators and monitoring:

| Endpoint | Type | Description |
| :--- | :--- | :--- |
| `GET /healthz` | Liveness | Verifies the HTTP process is responsive. Returns `200 OK`. |
| `GET /readyz` | Readiness | Verifies PostgreSQL connectivity, pool health, and schema migration status. Returns `200 OK` when ready, or `503 Service Unavailable` if database is down. |
| `cuan-server --healthcheck` | CLI Probe | Native container healthcheck probe used inside minimal Alpine containers. |

---

## Local Development (Without Docker)

### Prerequisites

- **Go**: 1.22+ (tested on Go 1.27)
- **Node.js**: 20+ (tested on Node 24) and **npm**
- **Coding Agent CLI**: `agy` (v1.4.0) installed and authenticated
- *(Optional)* **PostgreSQL**: 15+ (in-memory mock store activates automatically if DB is unset)

### Quick Start (Dev Mode)

To start the Go backend and frontend dev server concurrently:

```bash
./scripts/dev.sh
```

- API Server: `http://localhost:8080` (healthcheck: `http://localhost:8080/healthz`)
- Web UI: `http://localhost:3000`

### Building the Project

```bash
./scripts/build.sh
```

Outputs:
- `bin/cuan-server` — API server binary
- `bin/cuan-runner` — Task execution daemon
- `bin/cuan-migrate` — Database migration CLI
- `web/dist/` — Production frontend bundle

### Running Database Migrations

```bash
# Preview migration SQL plan without requiring a database
./scripts/migrate.sh --dry-run

# Apply migrations against a PostgreSQL instance:
export DATABASE_URL="postgres://cuan:cuan@localhost:5432/cuan?sslmode=disable"
./scripts/migrate.sh
```

### Running Tests

```bash
# Run fast unit, auth, migration, and server smoke tests:
./scripts/test.sh --fast

# Run full test suite including agent subprocess spike tests:
./scripts/test.sh
```

---

## Verification Evidence

All DoD verification records for Sprint 1 are stored in `docs/sprints/evidence/S1/`:
- [github_connection_verified.txt](docs/sprints/evidence/S1/github_connection_verified.txt): Real GitHub repository connection verification (`buchin/cuan`), default branch detection, and remote branches retrieval.
- [access_denied_handled.txt](docs/sprints/evidence/S1/access_denied_handled.txt): Informative error handling for 404 (Repo Not Found), 401/403 (Access Denied / Bad PAT), and 400 (Invalid URL).
- [credential_security_audit.txt](docs/sprints/evidence/S1/credential_security_audit.txt): AES-256-GCM encryption audit, zero-leak API response audit, git secret masking, and remote protection verification.
- [projects_ui_walkthrough.md](docs/sprints/evidence/S1/projects_ui_walkthrough.md): Comprehensive walkthrough of the Projects list, Connect Repository modal, and Project Detail UI.
- [github_token_guide.md](docs/guides/github_token_guide.md): Owner guide for generating, configuring, and revoking GitHub Personal Access Tokens.
- [smoke_install_login_restart.txt](docs/sprints/evidence/S1/smoke_install_login_restart.txt): End-to-end smoke verification log of clean install, takeover lockout, login, restart persistence, and logout.
- [health_readiness_response.txt](docs/sprints/evidence/S1/health_readiness_response.txt): HTTP responses for `/healthz`, `/readyz`, and container healthcheck CLI.
- [https_proxy_guide.md](docs/sprints/evidence/S1/https_proxy_guide.md): Reverse proxy HTTPS architecture, port allocations, and Nginx/Caddy guides.
- [configuration_clean.txt](docs/sprints/evidence/S1/configuration_clean.txt): Audit report verifying zero credential leakage.


---

## License

Licensed under the [Apache License, Version 2.0](LICENSE).
Copyright 2026 Buchin and Cuan contributors. See [NOTICE](NOTICE) for details.
