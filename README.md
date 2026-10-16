# Cuan

> Agentic project management for self-hosted teams: connect a repository, assign work to an agent on your server, and review changes with verification evidence.

## Status

**Sprint 1: Running** (Approved by PO on 2026-10-16).
Status: **Pre-release (Pre-alpha)**. Core monorepo foundation, official agent adapter spike, baseline database migrations, and development tooling are implemented.

Single Source of Truth (SSOT): [docs/sprints/S1.md](docs/sprints/S1.md).

---

## Architecture Overview

Cuan is organized as a Go and React TypeScript monorepo:

- **Backend API (`cmd/server`, `internal/server`)**:
  Go `net/http` + `chi` router providing `/healthz`, `/api/v1/info`, and task management APIs.
- **Task Runner Daemon (`cmd/runner`, `internal/runner`)**:
  Go worker daemon handling execution leases, agent subprocess lifecycle, log streaming, and process cancellation.
- **Coding Agent Adapter (`internal/agent`)**:
  Official adapter for headless coding agents (selected: `agy` v1.4.0). Features:
  - Headless non-interactive execution
  - Cancelable process group signal management (`SIGINT`/`SIGTERM`/`SIGKILL`)
  - Automatic secret masking/redaction from stdout, stderr, and execution logs
  - Peak RSS memory & duration resource telemetry
- **Database & Migrations (`cmd/migrate`, `internal/db`, `migrations/`)**:
  PostgreSQL with versioned SQL migrations (`000001_baseline.up.sql`). Includes `--dry-run` inspection mode that can run without live PostgreSQL.
- **Frontend UI (`web/`)**:
  React 18 + Vite + TypeScript + Tailwind CSS with shadcn/ui components (`web/src/components/ui/`).
- **Development Tooling (`scripts/`, `Makefile`)**:
  Self-contained scripts with intelligent environment detection and graceful fallbacks.

---

## Getting Started & Local Development

### Prerequisites

- **Go**: 1.22+ (tested on Go 1.27)
- **Node.js**: 20+ (tested on Node 24) and **npm**
- **Coding Agent CLI**: `agy` (v1.4.0) installed and authenticated
- *(Optional)* **PostgreSQL**: 15+ (migrations support `--dry-run` inspection if local DB is not yet running)

### Quick Start (Dev Mode)

To start the Go backend and frontend dev server concurrently:

```bash
# Using Makefile
make dev

# Or directly using the dev script
./scripts/dev.sh
```

- API Server: `http://localhost:8080` (healthcheck: `http://localhost:8080/healthz`)
- Web UI: `http://localhost:3000`

### Building the Project

```bash
# Build all Go binaries and frontend dist
make build

# Or using the build script:
./scripts/build.sh
```

Outputs:
- `bin/cuan-server` — API server binary
- `bin/cuan-runner` — Task execution daemon
- `bin/cuan-migrate` — Database migration CLI
- `web/dist/` — Production frontend bundle

### Running Database Migrations

Cuan features zero-friction migration tooling with dry-run support:

```bash
# Preview migration SQL plan without requiring a database
./scripts/migrate.sh --dry-run

# Apply migrations against a PostgreSQL instance:
export DATABASE_URL="postgres://cuan:cuan@localhost:5432/cuan?sslmode=disable"
./scripts/migrate.sh
```

### Running Tests

```bash
# Run all unit tests, migration tests, and agent spike tests:
make test

# Or run fast unit tests only:
./scripts/test.sh --fast
```

---

## License

Licensed under the [Apache License, Version 2.0](LICENSE).
Copyright 2026 Buchin and Cuan contributors. See [NOTICE](NOTICE) for details.
