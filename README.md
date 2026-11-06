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
  - [6. Calm Design System & Unified Chat Workspace](#6-calm-design-system--unified-chat-workspace)
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
  - Robust turn budget guard for Inbox coordinator chat (`CUAN_CHAT_TOOL_BUDGET`, default 50 turns).
  - Adaptive batching for code/file inspections (5:1 factor allowing up to 250 inspection calls per turn) preventing premature cut-offs during deep repository investigation.
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
  - **Detached Worker Process (Zero-Kill Deployment)**: Task worker executions run in isolated execution scopes (`setsid` and `systemd-run --scope` with `KillMode=process`), completely decoupling active worker processes from the supervisor daemon lifecycle. Deployments restart supervisor daemons in <1s without waiting for in-flight tasks, and running workers finish LLM/tool execution, git commits, and database updates uninterrupted.
  - **Startup Worker Reconciliation**: Newly started supervisor daemons automatically discover running detached workers via PID and heartbeat, renew leases on the API server, prevent double-claims, and immediately claim subsequent tasks from the queue.
  - **Graceful Drain & Zero-Interruption Self-Update**: Automatically finishes in-flight jobs during deployment before swapping to new release binaries.
  - **Progressive Adaptive Timeout**: Adaptively scales execution limits (Attempt 1: 15m -> Attempt 2: 20m -> Attempt 3: 25m max cap) with 20s graceful termination tolerance.
- **Autonomous Scrum Engine & Proactive Review with AGENTS.md Injection**:
  - Background event listener captures worker completion events (`task:attempt_succeeded`, `task:attempt_failed`) and objectively evaluates Definition of Done criteria (clean git working tree, 100% targeted test pass in <10s, zero build errors, lean modularity, and docs/Landing Page updates).
  - Automatically injects repository rules and guidelines (`AGENTS.md` and Scrum rules §1–§13) into system prompts of Chat Agent and Runner Agent.
  - **Self-Healing Auto-Requeue**: Automatically returns tasks to the queue with actionable revision instructions and progressive timeouts for fixable gaps (capped at 3 attempts).
  - **Auto-Mark Done & Zero-Jargon Chat**: Automatically marks verified tasks as `done` and proactively delivers warm, business-friendly reports and strategic next steps to store owners in Chat/Inbox without waiting for human prompts.
- **Pure Live PTY Terminal, Deep Runner Formatter & Mobile Safe-Area Ergonomics**:
  - Pure full-viewport `xterm.js` terminal directly displaying execution streams without artificial visual step abstractions.
  - Deep recursive runner payload parsing (`ndjsonFormatter.ts`) unwrapping nested runner parameters into human-readable action cards (`Inspect Tasks`, `Read App.tsx`, `Update TaskTerminal.tsx`).
  - Decoupled runner daemon stream resilience: graceful handling of 404 responses without false disconnect alarms, paired with a calm Things 3 `• Berjalan` / `• Live` pulsing micro-indicator.
  - Bidirectional steering bridge (`/api/v1/tasks/:id/steer`) and interactive `TerminalComposer` with always-accessible Quick Action Chips (Stop Ctrl+C, Clear, Tab, y, n).
  - Mobile safe-area cushion (`pb-[max(0.75rem,env(safe-area-inset-bottom))]`) positioning composer controls comfortably above Android 3-button system bars and iOS gesture indicators with WCAG 2.1 AA touch targets (min 44x44px).
- **Canonical Task Hierarchy & Breathing Room Layout**:
  - Unified routing consolidates task inspection and live execution under canonical `/tasks/:taskId`, eliminating route fragmentation while retaining contextual return navigation (`taskReturnTo`).
  - Reduced outer padding on mobile (390px) and desktop gives tasks breathing room, free of nested border boxes.
- **Smart Conditional Auto-Deploy Gate, Dual-Channel Envelope & Dynamic Context**:
  - Intelligent runner compliance gate: tasks with code modifications must compile, pass tests, and verify live deployment before being marked `done`; read-only investigations and RCA tasks complete cleanly with evidence without forced fake deployments.
  - Dual-Channel Output Envelope (`summary`, `draft`, `evidence`, `requires_deploy`) delivering operator status and visitor reply drafts with strict technical jargon filtering (`Definition of Done`, `DoD`, `heartbeat lost`, `lease expired`, `attempt #`).
  - Dynamic context adaptor replaces static "toko Anda" assumptions with actual project identity (`d.ProjectName`), greeting drama, SaaS, or creator workspaces in natural context.
  - Hybrid Actionable Task Card in chat timeline offering direct Live Terminal access, Retry, and 1-click visitor draft insertion, while delegating secondary diagnostics to on-demand `SlideDrawer`.
  - Chat bubble normalization, infrastructure fault isolation, and single-bubble idempotency protecting user revision quotas from runner disconnects.
- **Creator Blueprints & Studio**:
  - Standardized project templates, execution blueprints, and modular workspaces for rapid business asset generation.

### 6. Calm Design System & Unified Chat Workspace

Cuan incorporates a **Calm & Layered UI System** inspired by world-class productivity tools **Things 3** and **Bear Notes**:

- **5-Pillar Navigation & Header Quick Access**:
  - Primary bottom navigation organized into 5 ergonomic thumb-zone pillars: **Chat** (`/chat`), **Tasks** (`/tasks`), **Projects** (`/projects`), **Contacts** (`/contacts`), and **User Profile** (compact avatar popover).
  - Top header features MoreVertical `[⋮]` quick access menu directly navigating to **Agents** (`/agents`), **Schedules** (`/schedules`), and **Settings** (`/settings`).

- **Apple Mail & Bear Inset List in Inbox (Zero Screen-Wide Clutter Bars)**:
  - Subpanel conversations formatted into an Apple Mail & Bear Notes 3-tier inset list: sender & tabular timestamp on row 1, clean Geist subject with micro-indicator `StatusDot` on row 2, and 2-line snippet in muted zinc-400 on row 3.
  - Total elimination of full-width `[+ New Idea / Task]` quick bars and loud neon blue filter buttons in favor of calm, neutral Things 3 segments (`All`, `Agents`, `Customers`).
  - Separated by elegant hairline dividers (`border-b border-border/40`) with zero nested cards.
- **Calm Low-Contrast Chat Bubbles & Quiet Date Dividers**:
  - Chat bubbles designed with monochromatic micro-tints (`#1b1c1f` vs `#25272c`) on a deep `#141416` dark canvas, eliminating harsh 1px solid white wireframe borders.
  - Date dividers (`Today`, `Yesterday`) use quiet muted text with zero solid white `#ffffff` glowing pills in dark mode.
  - In-stream agent status summaries and task cards flow borderless with subtle typographic hierarchy.
- **Progressive Disclosure SlideDrawer for Task References in Chat**:
  - Clicking task links or task cards inside the chat stream opens an on-demand `SlideDrawer` from the right edge, eliminating canvas disruptions and preserving 100% of the active conversation stream.
- **Things 3 Pure Task Rows (Zero Action Buttons on Rows)**:
  - Task lists across desktop and mobile adhere to pure Things 3 design: subtle `StatusDot` micro-indicators on the left, spacious Geist typography in the center, and muted tabular relative timestamps on the right.
  - Complete elimination of loud `action-primary` buttons (*View result*, *Live Terminal*, *Retry*) from row surfaces. Every row is an expansive tap target that opens deep task actions inside an on-demand side drawer (`SlideDrawer` / `TaskSidePeek`).
- **Bear Notes Chat Stream & Borderless Artifacts**:
  - Chat stream streams smoothly without thick nested borders or harsh framing. Approval cards, task completion cards, and status summaries render as calm typographic text blocks with subtle inline badges.
- **Interactive Mermaid Diagrams in Chat**:
  - Automatically renders ```mermaid and ```flowchart code blocks into rich, visual SVG diagrams instead of raw code blocks.
  - Fully responsive on mobile viewports (360px–390px) with `max-width: 100%`, `height: auto`, and touch momentum horizontal scrolling (`-webkit-overflow-scrolling: touch; touch-action: pan-x pan-y`).
  - Interactive Pop-up / Fullscreen Viewer: provides zoom controls (Zoom In, Zoom Out, Reset 100%), interactive drag pan, syntax copying, and toggle between visual SVG and raw code view.
  - Graceful Fallback: broken or invalid syntax cleanly renders an alert badge and safe formatted fallback code without crashing the chat canvas.
- **Card Diet & Calm Layout Primitives (`HairlineList`, `StatusDot`, `SlideDrawer`, `GroupedSection`)**:
  - Eliminates visual clutter, SaaS education banners, and thick nested cards (`box inside box`). Replaced with flat serene surfaces, crisp hairline dividers (`divide-border/40`), micro-indicators (`StatusDot`), and Apple-style grouped rows (`GroupedSection`) across Apps & Skills and General Settings.
- **Progressive Disclosure in Agents View**:
  - Streamlines agent profile cards from 14 cluttered DevOps elements down to clean, essential identity profiles (Avatar, Name, Role Handle, Short Description, and StatusDot). Advanced engine parameters, BYOK configs, vision toggles, and token controls are revealed on-demand via `SlideDrawer`.
- **Inclusive Workspace Language**:
  - 100% elimination of legacy "Toko" terminology across all UI views and locale dictionaries (`id.json` and `en.json`) in favor of universal "Workspace" / "Ruang Kerja" language suited for creators, solopreneurs, and agencies.
- **Standalone Studio Canvas & Calm Chat Stream**:
  - Standalone Studio Canvas (`/studio`): Spacious, focused ideation canvas (`max-w-[75ch]`) with back navigation (`←`) and smooth redirect to execution threads upon submission.
  - Complete Elimination of Mobile Clashes: Removed conflicting FABs in mobile bottom nav and eliminated cramped slide-up sheets.
  - Clean Thread List & Status-by-Exception: Replaced noisy 4-badge stacks on thread lists with a single subtle `StatusDot` micro-indicator active only when agents are working.
  - Clean JSON Snippets: Stripped raw JSON payloads in thread previews into human-friendly text.
- **Clean Account vs Settings Separation & Schedules Consolidation**:
  - Dedicated `/account` workspace for personal preferences (Profile, Appearance, Notifications, Sign Out) using Apple-style Inset Grouped Sections.
  - Dedicated `/settings` workspace for operational configuration (General Workspace, Schedules, AI Providers & Models, Agent Memory, Agents, Apps & Skills).
  - Schedules consolidated directly into Settings (`/settings/schedules`), establishing a single calm center for background system automations.
- **Information Layering & Progressive Disclosure**:
  - Essential glanceable operational state stays on the surface; deep technical data (git branches, terminal sessions, raw stdout/stderr logs) is revealed smoothly on-demand via `SlideDrawer` side drawers.
- **Status-by-Exception**:
  - Calm neutral palette with high-contrast accent colors reserved strictly for exceptions requiring user intervention, preventing cognitive fatigue.
- **Ergonomic Desktop Column Constraints & Geist Typography**:
  - Strict 65–80ch reading widths (`max-w-[75ch]`) preventing eye strain on widescreen monitors, paired with the razor-sharp Geist typeface.
- **5-Pillar Bottom Navigation & Calm Chat Canvas**:
  - Primary navigation streamlined into 5 ergonomic thumb-zone pillars: **Chat** (`/chat`), **Tasks** (`/tasks`), **Projects** (`/projects`), **Contacts** (`/contacts`), and **User Profile** (compact avatar popover).
  - Chat canvas stripped of rigid 1px wireframe border cuts and noisy AI memory chips, featuring a floating borderless soft surface composer ala Bear Notes.
  - Direct PTY Live Terminal with Things 3 visual steps formatter and emerald pulsing live log indicator.

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
- **Atomic Micro-Packages (`pkg/taskfsm`, `pkg/dodeval`)**:
  - **`pkg/taskfsm`**: Standalone, zero-dependency Go Finite State Machine engine managing task and sprint lifecycles (`planning → approved → running → review → completed`, `blocked`, `canceled`) with illegal transition guardrails and JSON audit logs.
  - **`pkg/dodeval`**: Standalone, zero-dependency Definition of Done evaluator and Markdown checklist parser with evidence gap analysis and automated completion decisions.

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
- [Atomic Micro-Packages Architecture](docs/guides/atomic_micro_packages_guide.md) — Design principles, zero context overload, and standalone Go package specifications.
- [GitHub Integration & PAT Setup](docs/guides/github_token_guide.md) — Connecting repositories and managing access tokens.
- [Deployment & Recovery Playbook](docs/guides/deployment_recovery_guide.md) — Production staging, automated backup, and rollback drills.
- [Troubleshooting Playbook](docs/guides/troubleshooting.md) — Common operational issues, logs diagnosis, and service recovery.

---

## Sprint History & Release Verification

Cuan is engineered under strict **Agentic Scrum** standards where every sprint produces a documented Single Source of Truth (`docs/sprints/S{number}.md`) and Definition of Done (DoD) verification evidence:

- **Sprint 65 (Current)**: [Conditional Auto-Deploy Gate, Dual-Channel Envelope & Dynamic Context Adaptor](docs/sprints/S65.md) — Smart runner auto-deploy compliance gate distinguishing code modifications from investigations, dual-channel output envelopes with zero technical jargon, dynamic domain context adaptor, hybrid actionable task cards with 1-click visitor draft insertion, chat bubble un-suppression, runner disconnect infrastructure fault isolation, and single-bubble idempotency.
- **Sprint 64**: [Deep Runner Payload Formatting, Human-Readable Action Cards & Things 3 Terminal Serenity](docs/sprints/S64.md) — Elimination of visual steps tab abstraction in favor of pure full-viewport xterm.js terminal, recursive deep runner payload unrolling into human-readable action cards (`Inspect Tasks`, `Read App.tsx`, `Update TaskTerminal.tsx`), decoupled runner stream resilience without false disconnect alarms, calm Things 3 status indicator (`• Berjalan` / `• Live`), live bidirectional steering bridge (`/api/v1/tasks/:id/steer`), and mobile safe-area padding ergonomics (`env(safe-area-inset-bottom)`).
- **Sprint 63**: [Autonomous Agentic Scrum Engine, Direct AGENTS.md Prompt Injection, Live Terminal Streaming & Task Route Hierarchy](docs/sprints/S63.md) — Autonomous Scrum Engine evaluating Definition of Done criteria with self-healing retries, direct `AGENTS.md` and Scrum rules injection into system prompts, 5-pillar bottom navigation, header MoreVertical menu, and canonical `/tasks/:taskId` routing.
- **Sprint 62**: [Things 3 & Bear Notes Navigation & Visual Calm: Mobile Nav Overhaul, Chat Canvas Cleanup, Universal Inset Group Audit & Terminal Streaming](docs/sprints/S62.md) — Mobile navigation overhaul with Contacts 4-pillar nav, schedules consolidation into `/settings/schedules`, 1px wireframe border and memory chip removal from chat canvas, universal inset grouped sections, and interactive terminal streaming.
- **Sprint 61**: [Things 3 & Bear Notes Total Overhaul: Apple Mail Inset List, Calm Low-Contrast Chat Bubbles & SlideDrawer Task Integration](docs/sprints/S61.md) — 3-tier Apple Mail & Bear Notes inset conversation list in Inbox, calm low-contrast chat bubbles without 1px border lines, quiet date dividers without white pill glare, and progressive disclosure `SlideDrawer` for in-stream task cards.
- **Sprint 60**: [The Real Things 3 & Bear Overhaul + Mobile Bug Fixes & Toko Relics Elimination](docs/sprints/S60.md) — Things 3 pure task rows with 0 loud buttons and mobile glitch fix, total card diet & elimination of SaaS banners in Apps & Skills and General Settings, progressive disclosure in Agents View, 100% elimination of legacy "Toko" relics, and Bear Notes chat stream typography with borderless artifacts.
- **Sprint 59**: [Calm & Layered UI System (Things 3 & Bear Notes Inspired) and Unified Workspace](docs/sprints/S59.md) — Things 3 & Bear Notes design philosophy, Unified Chat Workspace (Studio + Inbox fusion), 3-group desktop navigation and 4-pillar mobile bottom nav, Card Diet refactor across core pages (Projects, Schedules, Contacts, Settings), and Calm Layout Primitives (`HairlineList`, `StatusDot`, `SlideDrawer`, `GroupedSection`).
- **Sprint 58**: [Clean Chat UI, Live Streaming Logs & Interactive Agent Steering in cuan-runner](docs/sprints/S58.md) — Live stdout/stderr streaming pipe, harmonic Web Audio chime notification, tab title status badges, and interactive chat steering.
- **Sprint 51**: [Multi-User Collaboration & Staff Management with RBAC](docs/sprints/S51.md) — Fural permission model (Owner vs Member), TeamView staff management, 1-click WhatsApp invitations, `/invite/accept` activation page, and migration `000030`.
- **Sprint 50**: [Dual-Mode AI Engine & Universal BYOK](docs/sprints/S50.md) — Official subscription harnesses (Claude, Codex, AGY), Universal BYOK (OpenAI, DeepSeek V3/R1, Anthropic), user-space binary manager in `~/.local/bin`, and AES-256-GCM encryption.
- **Sprint 1–49**: Historical sprint specifications and DoD verification records are preserved under [`docs/sprints/`](docs/sprints/) and [`docs/sprints/evidence/`](docs/sprints/evidence/).

---

## License & Community

Cuan is open-source software licensed under the [Apache License, Version 2.0](LICENSE).  
Copyright © 2026 Buchin and Cuan contributors. See [NOTICE](NOTICE) for details.
