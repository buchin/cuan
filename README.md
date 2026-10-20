# Cuan

> Agentic Project Management for Creators: connect repositories and blueprints, assign tasks to AI agents on your server, and turn ideas into finished content with verification evidence.

## Status

Cuan provides a self-hosted, single-owner workspace for projects, tasks, agent
conversations, and live chat contacts. The interface and new system messages use
English by default; UI language can be changed in Settings. User content retains its language.

Start with the [English workspace guide](docs/guides/navigation.md) and
[Contacts & live chat](docs/guides/contacts-crm.md). Historical sprint plans and
release evidence remain under `docs/sprints/` and `docs/evidence/` and may use
Indonesian. They describe earlier releases rather than current product behavior.

### AGY chat sandbox prerequisite

Native Linux AGY chat analysis now requires an external Bubblewrap sandbox.
Run `sudo python3 scripts/setup-inbox-sandbox.py --user cuan` with the actual
Linux service user before enabling chat. See [setup, checks, container limitations,
and rollback](docs/guides/chat-sandbox.md). Docker AGY readiness is not yet verified;
application health alone does not prove chat readiness.

## Documentation & Guides

- [Notifications](docs/guides/notifications.md) — Context, read state, handled updates, and notification settings.

- [Navigation & Responsive UI](docs/guides/navigation.md) — Inbox, Settings → Agents, project settings, keyboard controls, and device layouts.
- [Messages & Tasks Guide](docs/guides/message_task_lifecycle.md) — Durable chat, follow-up queue, live activity, message-to-task capture, execution attempts, and recovery.
- [Onboarding & Self-Hosting Guide](docs/guides/onboarding.md) — deployment prerequisites, persistent storage, and agent readiness.
- [User & Operational Guide](docs/guides/user_guide.md) — Studio, Inbox, agent-managed work, and recovery.
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
  PostgreSQL 16 with versioned embedded migrations. Controlled production upgrades require release-specific migration and rollback checks. Docker persists PostgreSQL, application data, and account HOME in separate named volumes.
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

## Installation

Follow the [onboarding guide](docs/guides/onboarding.md) for native Linux or Docker
application setup, owner creation, and agent readiness checks. Native Linux is the
verified AGY execution path; the stock Docker image does not provide a turnkey
AI runtime.

```sh
git clone https://github.com/buchin/cuan.git
cd cuan
./scripts/install.sh --dry-run
./scripts/install.sh --existing-proxy
```

The Docker installer requires a reachable Docker Engine and Compose, creates
private configuration with random secrets, preserves existing configuration,
and checks application/database readiness. Persist the database, application-data,
and account-HOME volumes. Export existing container data before adding mounts to
an older installation. Do not use `down -v` to upgrade.

For production changes, use the [deployment and recovery guide](docs/guides/deployment_recovery_guide.md).

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
> **Core Principle**: *Rolling back an application image does not roll back its database.*
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

## Backers & Sponsors

<!-- SPONSORS:START -->

### v1.0 Founding Backers

_No sponsors are registered for this batch yet. Become the first founding backer at [cuan.dev/sponsor](https://cuan.dropsugar.com/sponsor)!_

<!-- SPONSORS:END -->

---

## License

Licensed under the [Apache License, Version 2.0](LICENSE).
Copyright 2026 Buchin and Cuan contributors. See [NOTICE](NOTICE) for details.

- [Contacts CRM dan integrasi live chat](docs/guides/contacts-crm.md)
- [Tasks workspace, sessions, terminal, and release procedure](docs/guides/tasks-workspace.md)
