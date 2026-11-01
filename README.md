# Cuan

> **Agentic Workspace & Operating System for Creators, Solopreneurs, and MSMEs (UMKM)**: Connect repositories and creator blueprints, assign tasks to autonomous AI agents on your own server, collaborate with staff under robust RBAC, manage customer interactions across WhatsApp and Live Chat, and ship verified outcomes with zero SaaS dependencies or double-cost token markups.

[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://golang.org)
[![React](https://img.shields.io/badge/React-18-61DAFB?logo=react)](https://react.dev)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.0+-3178C6?logo=typescript)](https://www.typescriptlang.org)
[![Tailwind CSS](https://img.shields.io/badge/TailwindCSS-shadcn%2Fui-38B2AC?logo=tailwindcss)](https://ui.shadcn.com)
[![Database](https://img.shields.io/badge/Database-SQLite%20%7C%20PostgreSQL-4169E1?logo=postgresql)](migrations/)

---

## Table of Contents

- [Overview](#overview)
- [Key Capabilities](#key-capabilities)
  - [1. Multi-User Collaboration & Staff Management (RBAC)](#1-multi-user-collaboration--staff-management-rbac)
  - [2. Universal AI Providers & Dual-Mode BYOK Engine](#2-universal-ai-providers--dual-mode-byok-engine)
  - [3. Multi-Channel Inbox & Contacts CRM](#3-multi-channel-inbox--contacts-crm)
  - [4. Extensibility & MCP Client Engine](#4-extensibility--mcp-client-engine)
  - [5. Project & Task Workspace Management](#5-project--task-workspace-management)
- [Architecture Overview](#architecture-overview)
  - [System Architecture Diagram](#system-architecture-diagram)
  - [Core Components](#core-components)
- [Quick Start & Installation](#quick-start--installation)
  - [Prerequisites](#prerequisites)
  - [Native Linux Quick Start](#native-linux-quick-start)
  - [Docker Compose Deployment](#docker-compose-deployment)
  - [Local Development](#local-development)
- [Health Probes & Disaster Recovery](#health-probes--disaster-recovery)
- [Documentation & Guides](#documentation--guides)
- [Sprint History & Release Verification](#sprint-history--release-verification)
- [License & Community](#license--community)

---

## Overview

**Cuan** is a self-hosted Agentic Workspace and Operating System designed specifically for creators, online business owners, solopreneurs, and small-to-medium enterprises (UMKM). It bridges the gap between high-level business goals and autonomous technical execution:

- **Zero-Friction for Non-Developers**: All core operations—from staff onboarding and task assignment to AI provider configuration—are managed directly through an intuitive web UI. Technical defaults, headless subprocesses, and sandbox isolation are handled automatically under the hood.
- **Zero Double-Cost AI Economics**: Use official subscription CLIs (`claude`, `codex`, `agy`) without paying double API token fees, or bring your own API keys (OpenAI, DeepSeek V3/R1, Anthropic, Groq, Ollama) directly at cost.
- **Zero External SaaS Lock-in**: Autonomous self-contained Go backend. Authentication, role-based access control, database storage, and integrations run completely on your own hardware or VPS without dependency on third-party SaaS auth or token proxy services.
- **Verifiable Business Outcomes**: Work is executed by autonomous agents and logged with inspectable verification evidence, automated git commits, and optional deployment pipelines.

The interface defaults to English, with full Indonesian localization available via Settings.

---

## Key Capabilities

### 1. Multi-User Collaboration & Staff Management (RBAC)

Cuan incorporates the battle-tested **Fural Permission Model (Owner vs Member)** to enable seamless, secure teamwork between business owners and operational staff:

- **Role Owner**:
  - Full system administration: billing, git repository connections, server settings, and system observability.
  - Exclusive access to AI provider configurations, API keys, and user-space binary installations.
  - Creation, editing, and deletion of business AI assistants.
  - Staff management: issuing invitations and revoking member access.
- **Role Member**:
  - High-productivity operational access: creating and managing tasks, delegating work to pre-configured AI assistants, launching AgentSessions, participating in project discussions, and managing customer inquiries in the Multi-Channel Inbox.
  - **Role Presentation Guards**: Members are strictly prevented from viewing secret AI Provider credentials, installing server binaries, or modifying AI assistants. Unauthorized requests are rejected with `403 Forbidden` by backend RBAC middleware (`internal/auth/rbac.go`).
- **Team Management in TeamView**:
  - Clean two-tab interface organizing **AI Assistants** (`Asisten AI`) and human **Team Members** (`Anggota Tim`).
  - Human staff directory displaying role badges (`Owner` / `Member`), invitation and active statuses, join dates, and administrative controls.
- **Zero-Friction 1-Click WhatsApp & Link Invitations**:
  - Owners can invite staff without configuring SMTP servers, DKIM/SPF DNS records, or paid transactional email services.
  - Generates secure, cryptographic 7-day invitation token links (`/invite/accept?token=...`) with 1-click WhatsApp/Telegram sharing and clipboard copying.
- **Public Staff Activation Page**:
  - Streamlined `/invite/accept?token=...` onboarding page where invited members set their name and password, receive instant auto-login, and are immediately redirected to the workspace.

### 2. Universal AI Providers & Dual-Mode BYOK Engine

Cuan features a flexible **Dual-Mode AI Engine** (`internal/agent/`) supporting both official CLI harnesses and direct API integrations:

- **Official Subscription Harnesses (Zero Double-Cost)**:
  - Run existing paid AI subscriptions (Claude Pro/Team via official `claude` CLI, ChatGPT Plus/Team via `codex`, Gemini/Antigravity via `agy` CLI, CommandCode) headlessly on your server without incurring secondary API token costs.
- **Universal BYOK (Bring Your Own Key)**:
  - Connect directly to global and open-weights model providers: OpenAI (GPT-4o), DeepSeek (V3/R1), Anthropic (Claude Messages API), xAI (Grok), OpenRouter, Groq, Ollama, and Custom OpenAI-compatible LLMs.
  - Built with pure Go standard library (`net/http`) without external runtime dependencies.
- **Automated User-Space Binary Manager**:
  - Install, verify, and update official AI harness CLIs (`claude`, `codex`, `agy`) with one click directly into user-space `~/.local/bin` without requiring `sudo` or `root` permissions.
- **Enterprise-Grade Credential Security**:
  - API keys and tokens are encrypted at rest using **AES-256-GCM**.
  - All credentials are masked in API responses (`sk-...xxxx`) and strictly redacted from server logs (`[REDACTED]`).
- **Visual Latency & Connectivity Testing**:
  - Test connection latency in real time with millisecond ping metrics and colored status badges in `/settings/ai-providers`.
- **Per-Agent AI Provider & Model Binding**:
  - Customize each AI agent with the optimal provider and model for its role (e.g. DeepSeek V3 for cost-effective customer triage, Claude 3.5 Sonnet for deep engineering tasks).

### 3. Multi-Channel Inbox & Contacts CRM

Transform customer interactions into structured business workflows:

- **Unified Multi-Channel Inbox**:
  - Centralized messaging hub supporting website **Live Chat** widgets and direct **WhatsApp** integration (via Baileys / webhook).
- **Autonomous AI Agent Triage & Auto-Response**:
  - AI agents triage incoming customer inquiries, answer questions using persistent business memory and canned responses, classify intent, and escalate complex issues to human team members.
- **Adaptive Tool Budgeting & Infinite-Loop Guards**:
  - Robust turn budget guard for Inbox coordinator chat (`CUAN_CHAT_TOOL_BUDGET`, default 25 turns).
  - Adaptive batching for code/file inspections (3:1 factor allowing up to 75 inspection calls per turn) preventing premature cut-offs during deep repository investigation.
  - Hard safety ceilings and consecutive failure protection prevent runaway infinite loops while enabling seamless direct synthesis recovery.
- **Contacts CRM & Deal Funnel**:
  - Comprehensive customer contact profiles, persistent contact memory, interaction history timelines, deal stage tracking, and customer journey management.
- **Resend Email Alerts & Idle Chat Transcripts**:
  - Securely connect Resend API keys encrypted with **AES-256-GCM** in the server vault with zero secret leakage and in-memory key masking (`re_...xxxx`).
  - Automated idle chat detection engine combining per-room background debounce timers and a recurring 1-minute scheduler job (`chat_idle_email_digest`).
  - Scans for unanswered customer inquiries (> 5-10 minutes idle) and dispatches responsive, branded HTML & plain-text email transcripts with direct 1-click links back to the Cuan Inbox.
  - Automatic debounce & smart cancellation: timers instantly cancel when an operator replies or marks the conversation as read.
  - Cooldown deduplication protects recipient inboxes from notification flooding.

### 4. Extensibility & MCP Client Engine

- **Model Context Protocol (MCP) Client Engine**:
  - Native MCP client implementation in Go (`internal/mcp/`) enabling Cuan AI agents to dynamically discover, inspect, and execute tools provided by external MCP servers.
- **Apps & Skills Registry**:
  - Extensible catalog of modular skills and app connectors for repository inspection (`cuan-inspect`), content publishing, database querying, and custom automations.

### 5. Project & Task Workspace Management

- **Git & Repository Integration**:
  - Pre-flight repository verification, branch management, clone/fetch workspace isolation, and secret masking on git outputs.
- **Decoupled Task Worker Daemon (`cmd/runner` / `cuan-runner`)**:
  - Independent Go worker daemon decoupled from the HTTP web server, guaranteeing zero interrupted work during server restarts or deployments.
  - **Graceful Drain & Zero-Interruption Self-Update**: Automatically finishes in-flight jobs during deployment before swapping to new release binaries.
  - **Progressive Adaptive Timeout**: Adaptively scales execution limits (Attempt 1: 15m -> Attempt 2: 20m -> Attempt 3: 25m max cap) with 20s graceful termination tolerance.
- **Autonomous DoD Quality Loop & Proactive Business Chat**:
  - Background listener captures worker completion events (`task:attempt_succeeded`) and objectively evaluates Definition of Done criteria (clean git working tree, 100% targeted test pass in <10s, zero build errors, lean modularity, and docs/Landing Page updates).
  - **Self-Healing Auto-Requeue**: Automatically returns tasks to the queue with actionable revision instructions and progressive timeouts for fixable gaps (capped at 3 attempts).
  - **Auto-Mark Done & Zero-Jargon Chat**: Automatically marks verified tasks as `done` and proactively delivers warm, business-friendly reports and strategic next steps to store owners in Chat/Inbox.
- **Creator Blueprints & Studio**:
  - Standardized project templates, execution blueprints, and modular workspaces for rapid business asset generation.

---

## Architecture Overview

### System Architecture Diagram

```d2
workspace: "Cuan Workspace & OS" {
  frontend: "Web Frontend (React 18 + Vite + TypeScript + shadcn/ui)" {
    team_view: "TeamView (AI & Human Members)"
    ai_settings: "AI Providers & BYOK (/settings/ai-providers)"
    inbox_ui: "Multi-Channel Inbox & Contacts CRM"
    public_invite: "Public Invite (/invite/accept)"
  }

  backend: "Go Backend Server (net/http + chi)" {
    auth_rbac: "Auth & RBAC Middleware (Owner / Member)"
    ai_engine: "Dual-Mode AI Engine & Binary Manager"
    inbox_core: "Inbox & Triage Router"
    resend_service: "Resend Email & Chat Idle Detector"
    mcp_engine: "MCP Client Engine & Skills Registry"
    db_layer: "Database Layer (31 Auto-Migrations)"
  }

  storage: "Storage & Persistence" {
    sqlite_db: "SQLite (Zero-Config Embedded)"
    pg_db: "PostgreSQL 16"
  }

  ai_layer: "AI Execution Harnesses" {
    subscriptions: "Official CLI Harnesses (~/.local/bin)\n(Claude, ChatGPT/Codex, AGY)"
    byok_providers: "Universal BYOK (Direct HTTP)\n(OpenAI, DeepSeek V3/R1, Anthropic)"
  }

  external_channels: "External Channels" {
    whatsapp: "WhatsApp (Baileys / Webhook)"
    livechat: "Website Live Chat Widget"
    mcp_servers: "External MCP Servers & Tools"
  }
}

workspace.frontend -> workspace.backend: "REST API & WebSocket"
workspace.backend.auth_rbac -> workspace.backend.db_layer
workspace.backend.ai_engine -> workspace.ai_layer.subscriptions: "Headless Subprocess"
workspace.backend.ai_engine -> workspace.ai_layer.byok_providers: "Go net/http AES-256-GCM"
workspace.backend.mcp_engine -> workspace.external_channels.mcp_servers: "JSON-RPC (MCP)"
workspace.external_channels.whatsapp -> workspace.backend.inbox_core
workspace.external_channels.livechat -> workspace.backend.inbox_core
workspace.backend.db_layer -> workspace.storage.sqlite_db
workspace.backend.db_layer -> workspace.storage.pg_db
```

### Core Components

- **Backend API Server (`cmd/server`, `internal/server`)**:
  - Go `net/http` + `chi` lightweight router delivering REST endpoints for `/healthz`, `/readyz`, `/api/v1/info`, authentication, RBAC, projects, tasks, inbox, and AI providers.
  - Serves the production single-page application (SPA) directly at root `/`.
- **Database & Persistence (`internal/db`, `migrations/`)**:
  - **Dual Database Engine**: Supports **SQLite** (embedded, zero-config via `modernc.org/sqlite`) for lightweight self-contained setups and **PostgreSQL 16** for larger deployments.
  - **30 Embedded Auto-Migrations**: Migrations (`000001_baseline` to `000030_multi_user_and_invites`) execute automatically upon application startup with strict backward compatibility.
  - **In-Memory Mock Repositories**: Thread-safe memory stores for instant local testing without database infrastructure.
- **Authentication & RBAC (`internal/auth`)**:
  - Secure bcrypt password hashing, cryptographic session tokens, HttpOnly cookies, and strict RBAC authorization middleware (`internal/auth/rbac.go`).
- **Security & Cryptography (`internal/crypto`)**:
  - Industry-standard **AES-256-GCM** encryption for GitHub Personal Access Tokens and AI Provider API keys at rest.
- **Frontend SPA (`web/`)**:
  - Built with React 18, Vite, TypeScript, Tailwind CSS, and shadcn/ui components (`web/src/components/ui/`).
  - Master-detail responsive layouts, dark/light themes, keyboard navigation, and full i18n support.

---

## Quick Start & Installation

### Prerequisites

- **Native Linux**:
  - Linux (Ubuntu 22.04+ or Debian 12+ recommended)
  - Python 3 & Bubblewrap (`bwrap`) for sandboxed agent execution
  - Node.js 20+ and npm (for frontend compilation if building from source)
  - Go 1.22+ (if building backend binaries from source)
- **Docker**:
  - Docker Engine 24+ and Docker Compose v2+

### Native Linux Quick Start

```bash
# 1. Clone the repository
git clone https://github.com/buchin/cuan.git
cd cuan

# 2. Set up agent execution runtime sandbox (Bubblewrap)
sudo python3 scripts/setup-agent-runtime.py --user $(whoami)

# 3. Build backend binaries and web bundle
./scripts/build.sh

# 4. Start Cuan Server (uses embedded SQLite by default)
./bin/cuan-server
```

Open `http://localhost:8080` in your browser to complete the initial Owner setup wizard.

### Docker Compose Deployment

```bash
# 1. Clone repository
git clone https://github.com/buchin/cuan.git
cd cuan

# 2. Run the automated installer
./scripts/install.sh --dry-run
./scripts/install.sh --existing-proxy
```

The installer verifies environment readiness, creates isolated named Docker volumes (`cuan_postgres_data`, `cuan_app_data`), configures random secrets, and validates health checks.

### Local Development

To run backend and frontend concurrently with live reloading:

```bash
# Start Go backend (:8080) and Vite dev server (:3000)
./scripts/dev.sh
```

- **API Server**: `http://localhost:8080` (health: `http://localhost:8080/healthz`)
- **Web UI**: `http://localhost:3000`

#### Running Tests

```bash
# Run fast unit tests (Go backend + Vitest frontend):
./scripts/test.sh --fast

# Run Go backend tests only:
./scripts/test.sh --backend-only

# Run frontend build and typecheck only:
./scripts/test.sh --frontend-only
```

---

## Health Probes & Disaster Recovery

### Health & Readiness Endpoints

Cuan provides standard health endpoints for container orchestrators and monitoring systems:

| Endpoint | Type | Description |
| :--- | :--- | :--- |
| `GET /healthz` | Liveness | Verifies the HTTP process is responsive. Returns `200 OK`. |
| `GET /readyz` | Readiness | Verifies database connectivity, connection pool health, and migration state. Returns `200 OK` or `503 Service Unavailable`. |
| `GET /api/v1/info` | Metadata | Returns application version, license, repository URL, docs link, and release stage. |
| `cuan-server --healthcheck` | CLI Probe | Container healthcheck probe for Alpine/minimal environments. |
| `cuan-server --readycheck` | CLI Probe | Container readiness check probe testing `/readyz` for zero-downtime rollouts. |

### Backup, Restore & Disaster Recovery

- **Automated Backup**:
  ```bash
  ./scripts/backup.sh
  ./scripts/backup.sh --tag pre-update
  ```
- **Verified Restore**:
  ```bash
  ./scripts/restore.sh --latest --force
  ```
- **Atomic Rollback**:
  ```bash
  ./scripts/rollback.sh --version 0.1.0 --latest-pre-update --force
  ```

For detailed operational procedures, see the [Deployment and Recovery Guide](docs/guides/deployment_recovery_guide.md).

---

## Documentation & Guides

- [Onboarding & Self-Hosting Guide](docs/guides/onboarding.md) — Prerequisites, storage setup, and agent readiness.
- [Navigation & Responsive UI](docs/guides/navigation.md) — Workspace layout, keyboard shortcuts, and device support.
- [Multi-User & Unified Team Guide](docs/guides/unified_team_and_global_tasks_guide.md) — Team management, staff invites, and task delegation.
- [Contacts CRM & Live Chat](docs/guides/contacts-crm.md) — Multi-channel inbox, WhatsApp integration, and contact funnel.
- [Tasks Workspace & Lifecycle](docs/guides/tasks-workspace.md) — Task lifecycles, execution attempts, and sessions.
- [GitHub Integration & PAT Setup](docs/guides/github_token_guide.md) — Connecting repositories and managing access tokens.
- [Deployment & Recovery Playbook](docs/guides/deployment_recovery_guide.md) — Production staging, automated backup, and rollback drills.
- [Troubleshooting Playbook](docs/guides/troubleshooting.md) — Common operational issues, logs diagnosis, and service recovery.

---

## Sprint History & Release Verification

Cuan is engineered under strict **Agentic Scrum** standards where every sprint produces a documented Single Source of Truth (`docs/sprints/S{number}.md`) and Definition of Done (DoD) verification evidence:

- **Sprint 51 (Current)**: [Multi-User Collaboration & Staff Management with RBAC](docs/sprints/S51.md) — Fural permission model (Owner vs Member), TeamView staff management, 1-click WhatsApp invitations, `/invite/accept` activation page, and migration `000030`.
- **Sprint 50**: [Dual-Mode AI Engine & Universal BYOK](docs/sprints/S50.md) — Official subscription harnesses (Claude, Codex, AGY), Universal BYOK (OpenAI, DeepSeek V3/R1, Anthropic), user-space binary manager in `~/.local/bin`, and AES-256-GCM encryption.
- **Sprint 1–49**: Historical sprint specifications and DoD verification records are preserved under [`docs/sprints/`](docs/sprints/) and [`docs/sprints/evidence/`](docs/sprints/evidence/).

---

## License & Community

Cuan is open-source software licensed under the [Apache License, Version 2.0](LICENSE).  
Copyright © 2026 Buchin and Cuan contributors. See [NOTICE](NOTICE) for details.
