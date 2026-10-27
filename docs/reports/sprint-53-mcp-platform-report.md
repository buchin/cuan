# Sprint 53: Cuan OSS Full Platform MCP & Universal Inbox Scrum Master

## Ringkasan Eksekutif

Pada Sprint 53 ini, kami menuntaskan fondasi arsitektur **Cuan OSS (Open Source Software)** untuk pemisahan tugas antara **Inbox (Ruang Koordinasi & Scrum Master)** dan **Task (Ruang Eksekusi Riil Terisolasi)**.

Platform MCP server (`cuan mcp`) kini diperluas secara menyeluruh dengan **11 tools platform universal** (Projects, Tasks, Inbox, CRM Contacts, Scheduler, dan Settings), memberikan visibilitas penuh bagi **semua agen di Inbox** (@sm, @dev, @support, maupun custom agent) ke seluruh ekosistem multi-project Cuan (termasuk project Dojo, Cuan, dan custom user apps).

---

## 1. Arsitektur Agen Cuan OSS: Inbox vs Task Separation

Sesuai arahan dan kesepakatan prinsip arsitektur:

> **"Semua agen di Inbox adalah Scrum Master yang punya akses ke Full MCP, tapi eksekusi tetap dilakukan oleh agen di Task."**

```d2
direction: right

user: User / Product Owner {
  shape: person
}

inbox_scope: INBOX: KOORDINASI & PERENCANAAN (Read-Only / Analytical) {
  inbox_agents: Semua Agen di Inbox\n(@sm, @dev, @support, custom)\nBertindak sebagai Scrum Master
  
  mcp: Full Platform MCP (cuan mcp) {
    p: Projects (Dojo, Cuan, Multi-repo)
    t: Tasks (Kanban, Attempts, Logs)
    i: Inbox (Chat visitor, Komplain)
    c: Contacts & CRM (Profil, Memory)
    s: Scheduler (Cron & Background jobs)
    g: Settings (Telemetri, Agents, Release)
  }

  plan: Formulasi Solusi & Interactive Approval Card (DoD Checklist)

  inbox_agents <-> mcp: query platform data
  inbox_agents -> plan: bentuk rencana terstruktur
}

user <-> inbox_scope.inbox_agents: Chat Interaktif
inbox_scope.plan -> task_launch: User Klik "Setuju" / "Gas"

task_scope: TASK: EKSEKUSI RIIL (Mutasi & Otomasi Terisolasi) {
  runner: Task Runner (agy / claude / shell)\ndi Workspace Target Project
  runbook: Context-Aware Project Runbooks\n(e.g. php artisan di Dojo, npm/go di Cuan)
  pty: Live Terminal WebSocket Streaming\n(TaskTerminal)
  result: Laporan Bukti Verifikasi DoD

  runner -> runbook: eksekusi langsung CLI / Artisan / Code
  runner <-> pty: stream logs real-time
  runbook -> result: verifikasi kriteria selesai
}

task_launch -> task_scope.runner: Luncurkan attempt dengan lock & guardrail
task_scope.result -> inbox_scope.inbox_agents: Laporkan hasil kembali ke Inbox
```

### Karakteristik Desain:
1. **Inbox (Koordinator & Scrum Master)**:
   - Memiliki akses **Full MCP** ke seluruh modul platform Cuan dan multi-project context.
   - Tidak melakukan mutasi / shell execution berat langsung di chat agar chat tetap aman, cepat, dan responsif.
   - Merumuskan rencana dan menyajikan *Approval Card* dengan checklist *Definition of Done (DoD)*.
2. **Task (Eksekutor Terisolasi)**:
   - Terisolasi di workspace project yang tepat (misal: project `dojo` dengan runtime PHP/Laravel, atau project `cuan` dengan Go/React).
   - Dilengkapi *Project Runbook Directives*: untuk perintah operasional (seperti `php artisan payment:mark-paid`), agen langsung mengeksekusi CLI resmi project alih-alih membaca puluhan file dari nol.
   - Melaporkan log streaming via live terminal WebSocket tanpa jeda.

---

## 2. Spesifikasi Tool Full Platform MCP (`cuan mcp`)

Total 17 tools resmi tersedia melalui JSON-RPC 2.0 stdio:

| Domain | Tool Name | Deskripsi |
|---|---|---|
| **Projects** | `list_projects` | Daftar semua project yang dikelola platform Cuan beserta path & branch. |
| **Projects** | `get_project` | Detail project spesifik (ID, nama, git branch, repo URL, workspace path, jumlah task aktif). |
| **Tasks** | `list_tasks` | Daftar task lintas project atau terfilter status (`backlog`, `doing`, `review`, `done`, `blocked`). |
| **Tasks** | `get_task` | Detail lengkap task, instruksi, status, prioritas, DoD checklist, dan riwayat attempt. |
| **Tasks** | `create_task_draft` | Membuat draft usulan task baru dengan judul, instruksi/scope, prioritas, dan checklist DoD. |
| **Inbox** | `list_inbox_conversations` | Daftar room chat customer/visitor dan room project internal. |
| **Inbox** | `get_inbox_messages` | Riwayat pesan pada room/sesi tertentu terformat rapi dengan tipe pengirim & timestamp. |
| **CRM** | `list_contacts` | Daftar kontak customer/visitor, email, tags, dan session binding. |
| **CRM** | `get_contact` | Detail profil kontak dan catatan relationship memory (facts, notes). |
| **Scheduler** | `list_scheduled_jobs` | Status background cron jobs (inbox dispatcher, retention, health poller, dll). |
| **Settings** | `get_system_settings` | Informasi platform, versi server, koneksi database PostgreSQL, telemetri, dan jumlah resource. |
| **Inspection** | `inspect_git_status` | Status git working tree (read-only). |
| **Inspection** | `inspect_git_log` | Commit history git (1-100 commit). |
| **Inspection** | `inspect_git_diff` | Ringkasan perubahan uncommitted/staged (--stat). |
| **Inspection** | `inspect_git_branches` | Daftar branch lokal dan remote. |
| **Inspection** | `inspect_system_health` | Kesehatan host (df, free, load average). |
| **Inspection** | `inspect_typecheck` | Static typechecking (tsc / go vet). |

---

## 3. Hasil Pengujian & Verifikasi Live

### A. Unit Tests Suite
- `internal/diagnostics`: **100% PASS** (termasuk `TestPlatformMCPToolsList` dan `TestPlatformMCPToolsExecution`).
- `internal/agent`: **100% PASS** (verifikasi aturan izin settings.json untuk semua tools MCP).
- `internal/server`: **100% PASS** (verifikasi directive Scrum Master universal dan sandboxing bwrap).
- `web`: **125/125 PASS** (frontend test gate).

### B. Live Production Verification (`cuan.dojo.cc`)
1. **Health & Readiness**:
   - `https://cuan.dojo.cc/healthz`: `status: "healthy"`, commit SHA: `fd3dbb82c9a6baebc2ba79bdad9c9a7f6411752a`.
   - `https://cuan.dojo.cc/readyz`: `status: "ready"`, `database: "connected"`, `migrations_count: 25`.
2. **MCP Live Query**:
   - `cuan mcp` terverifikasi berhasil terkoneksi langsung ke PostgreSQL produksi.
   - Query `list_projects` mengembalikan 3 project riil (`cuan`, `dojo`, `default`).
   - Query `get_project dojo` mengembalikan status project Dojo beserta 4 active tasks.
   - Query `list_tasks dojo` mengembalikan task riil termasuk task aktivasi akses PAY-74.
