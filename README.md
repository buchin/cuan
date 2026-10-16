# Cuan

> Agentic project management for self-hosted teams: connect a repository, assign work to an agent on your server, and review changes with verification evidence.

## Status

**Sprint 1: Sprint Review (Completed T1–T8, Dogfooding Verified, OSS Release Candidate Ready)**.
Monorepo foundation, official agent adapter (`agy`), baseline migrations, persistent PostgreSQL installation, single-owner authentication, GitHub repository integration, task board & queue, runner daemon, Git workspace isolation & PR generator (T6), staging isolation & disaster recovery (T7), and end-to-end dogfooding demo & OSS release gate (T8) are fully implemented and verified.

Single Source of Truth (SSOT): [docs/sprints/S1.md](docs/sprints/S1.md) | [Sprint 1 Review Pack](docs/sprints/evidence/S1/sprint_1_review_pack.md).

### Documentation & Guides

- [Onboarding & Self-Hosting Guide](docs/guides/onboarding.md) — 5-minute setup, system requirements, initial configuration.
- [User & Operational Guide](docs/guides/user_guide.md) — Task lifecycle, review phase, revision cycles, and runner monitoring.
- [Troubleshooting & Recovery Playbook](docs/guides/troubleshooting.md) — Container issues, runner recovery, database locks, and disaster mitigation.
- [Deployment & Recovery Guide](docs/guides/deployment_recovery_guide.md) — Production staging, automated backup, and rollback drill procedures.
- [GitHub PAT Setup Guide](docs/guides/github_token_guide.md) — Generating, configuring, and revoking GitHub Personal Access Tokens.
- [Dogfooding Walkthrough](docs/sprints/evidence/S1/dogfooding_walkthrough.md) — Illustrated end-to-end walkthrough of developing Cuan using Cuan.

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

## Clean Environment Installation & Automated Deployment

Cuan provides an automated installer with system preflight checks (hardware, ports, reverse proxy detection) as well as direct Docker Compose support.

### Option A: Automated Installer (Recommended)

```bash
git clone https://github.com/buchin/cuan.git
cd cuan

# Run preflight checks and interactive/automated installation:
./scripts/install.sh --domain cuan.example.com --email admin@example.com

# For staging environment:
./scripts/install.sh --staging --domain staging.cuan.example.com
```

### Option B: Manual Docker Compose Setup

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
| `GET /api/v1/info` | Metadata | Returns application version, license, repository URL, docs link, and release stage. |
| `cuan-server --info` | CLI Banner | Displays formatted server metadata banner, license, and repository links. |
| `cuan-server --healthcheck` | CLI Probe | Native container healthcheck probe used inside minimal Alpine containers. |
| `cuan-server --readycheck` | CLI Probe | Native readiness check probe testing `/readyz` for zero-downtime health validations. |

---

## Staging, Updates, Backup & Disaster Recovery

### 1. Isolated Staging Environment
Run a staging preview instance with dedicated port `8081`, independent database volume (`cuan_staging_postgres_data`), and isolated network (`cuan-staging-network`) without touching Stable:
```bash
./scripts/install.sh --staging --domain staging.cuan.example.com
```

### 2. Safe Version Update Flow
Perform atomic version upgrades with pre-update health checks, automatic pre-update database backup, schema migration, and post-update data retention validation:
```bash
# Upgrade to version 0.2.0:
./scripts/update.sh --version 0.2.0
```

### 3. Database Backup & Restore Drill
Automated, verified PostgreSQL dumps with SHA256 integrity checksums, gzip stream verification, and retention policy:
```bash
# Create automated backup:
./scripts/backup.sh

# Tagged pre-update backup:
./scripts/backup.sh --tag pre-update

# Restore from latest verified backup:
./scripts/restore.sh --latest --force
```

### 4. Rollback & Disaster Recovery Playbook
> **Core Principle**: *Rollback image saja tidak dianggap rollback database.*
If an update encounters a failing migration or degraded state, `rollback.sh` reverts the application version AND restores the database from the pre-update backup snapshot:
```bash
# Execute safe rollback:
./scripts/rollback.sh --version 0.1.0 --latest-pre-update --force
```
For complete CLI disaster recovery commands (including direct `psql` queries, container log diagnosis, and VPS re-spins), see [docs/guides/deployment_recovery_guide.md](docs/guides/deployment_recovery_guide.md).

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
- [configuration_clean.txt](docs/sprints/evidence/S1/configuration_clean.txt): Audit report verifying zero credential leakage.
- [staging_isolation_verified.txt](docs/sprints/evidence/S1/staging_isolation_verified.txt): Proof of decoupled staging environment (isolated port 8081, volume, database, and network) with zero impact on Stable.
- [update_data_retention_verified.txt](docs/sprints/evidence/S1/update_data_retention_verified.txt): Proof of 100% data retention (tasks, attempts, audit activities, owner session) across version upgrade.
- [backup_restore_drill.txt](docs/sprints/evidence/S1/backup_restore_drill.txt): End-to-end backup snapshot creation, data wipe disaster simulation, and 100% data fidelity restoration.
- [failed_update_rollback_drill.txt](docs/sprints/evidence/S1/failed_update_rollback_drill.txt): Test of simulated failed update (bad migration / broken schema), automatic database rollback, and successful operational recovery.
- [installer_preflight_verified.txt](docs/sprints/evidence/S1/installer_preflight_verified.txt): Preflight hardware/port checks and non-destructive installer dry-run verification.
- [deployment_recovery_guide.md](docs/guides/deployment_recovery_guide.md): Operational guide and emergency CLI disaster recovery playbook.
- [dogfooding_e2e_demo.txt](docs/sprints/evidence/S1/dogfooding_e2e_demo.txt): Complete execution record of end-to-end dogfooding demo (Develop Cuan using Cuan).
- [dogfooding_walkthrough.md](docs/sprints/evidence/S1/dogfooding_walkthrough.md): Illustrated architectural and visual walkthrough of the dogfooding cycle.
- [oss_license_audit.txt](docs/sprints/evidence/S1/oss_license_audit.txt): Comprehensive license audit of Apache-2.0, NOTICE, Go dependencies, and npm libraries.
- [oss_security_and_secrets_audit.txt](docs/sprints/evidence/S1/oss_security_and_secrets_audit.txt): Static analysis and automated secret scanner audit verifying zero leaked credentials.
- [sprint_1_review_pack.md](docs/sprints/evidence/S1/sprint_1_review_pack.md): Master Sprint Review pack for Product Owner evaluation.


---

## License

Licensed under the [Apache License, Version 2.0](LICENSE).
Copyright 2026 Buchin and Cuan contributors. See [NOTICE](NOTICE) for details.
