# Panduan Mandiri Arsitektur UI Bersih: Strict Minimalist Scope Cuan OS

Dokumen ini merupakan panduan operasional mandiri (*self-contained operational guide*) berstandar §11 bagi *Digital Solopreneur*, pengembang mandiri (*Solo Coder*), pimpinan teknis (*Technical Lead*), dan administrator sistem mengenai arsitektur antarmuka bersih Cuan OS pasca-transformasi **"UI Radical Simplification: Strict Minimalist Scope"** (Sprint 15).

Panduan ini mendokumentasikan eliminasi total elemen pemasaran (*zero-marketing overhead*), penyederhanaan rute aplikasi (*app-first access*), sistem desain *Tokyo Night* (radius 0rem), 6 menu global navigasi, *Breadcrumb Switcher* model Jean, *Dashboard Empty State*, *Project Archiving*, serta penautan multi-proyek (*Multi-Project Task Linking*).

---

## 1. Arsitektur & Filosofi Antarmuka Bersih (§11)

### 1.1. Prinsip Desain
1. **App-First Direct Access:** Rute root `/` tidak lagi menampilkan brosur pemasaran statis atau banner promosi. Akses root langsung mengevaluasi status otentikasi: mengarahkan ke *Tokyo Night Auth Gatekeeper* (`/login`) jika belum login, atau langsung ke *Dashboard Workspace* (`/dashboard`) jika terotentikasi.
2. **Zero-Marketing Friction:** Seluruh artefak sponsorship, formulir sponsor, toggle runtime mode (`Mode: studio/marketing`), dan tombol 'Beranda' telah dieliminasi dari shell aplikasi.
3. **Fokus Kognitif Tinggi:** Navigasi disederhanakan secara murni menjadi **sembilan (9) pilar inti PO**, menjaga konsentrasi developer tanpa visual clutter.
4. **Tokyo Night Design System:** Menggunakan palet kontras tinggi `#1a1b26` (Dark) dan `#eaeef6` (Light) dengan border radius ketat `0rem` (tajam, presisi, industrial).

### 1.2. Diagram Arsitektur Antarmuka (D2 TALA)

```d2
direction: right

user: "Developer / Operator"

auth_gate: "Tokyo Night Auth Gatekeeper" {
  login: "Login Mandatory (/login)"
  auth_api: "POST /api/v1/auth/login"
  seed_creds: "Default Admin: admin / whosyourdaddy"
  login -> auth_api: "Credentials"
  auth_api -> seed_creds: "Verify & Issue Token"
}

minimalist_shell: "Minimalist App Shell (Tokyo Night 0rem)" {
  navbar: "Clean Breadcrumb Navbar" {
    sidebar_trigger: "Sidebar Trigger"
    jean_switcher: "Breadcrumb Switcher model Jean"
    theme_toggle: "Theme Toggle (Light / Tokyo Night)"
  }

  sidebar: "shadcn Sidebar-07 Shell" {
    global_nav: "6 Menu Global Sidebar" {
      menu_dashboard: "1. Dashboard"
      menu_tasks: "2. Tasks"
      menu_projects: "3. Projects"
      menu_discussions: "4. Discussions"
      menu_agents: "5. Agents"
      menu_team: "6. Team"
    }
    contextual_nav: "Contextual Project Modules" {
      mod_overview: "Overview"
      mod_wp: "WordPress Cloud"
      mod_repos: "Repos"
      mod_deploy: "Deployments"
      mod_telemetry: "Telemetry"
      mod_rooms: "Studio Rooms"
      mod_settings: "Settings"
    }
    nav_user: "NavUser Footer (Profile, Password Modal, Logout)"
  }

  core_views: "9 Core Feature Views" {
    view_dash: "Dashboard & Empty State"
    view_proj: "Projects Directory (Active / Archived)"
    view_task: "Task Board (Kanban / List Toggle & Multi-Project)"
    view_disc: "Discussions & Studio Rooms"
    view_agents: "AI Agents Hub"
    view_team: "Team View"
  }
}

user -> auth_gate.login: "Unauthenticated Request (/)"
auth_gate.seed_creds -> minimalist_shell.navbar: "Authenticated Redirect (/dashboard)"
minimalist_shell.navbar -> minimalist_shell.sidebar: "Sync Workspace Context"
minimalist_shell.sidebar -> minimalist_shell.core_views: "Mount Clean Minimalist View"
```

---

## 2. Sembilan (9) Pilar Antarmuka Minimalis

| No | Pilar Utama | Deskripsi & Kontrak Teknis |
| :--- | :--- | :--- |
| **1** | **Auth Gatekeeper** | Login wajib dengan default admin `admin` / `whosyourdaddy`. Rute API terproteksi (`/api/v1/auth/me`, `/api/v1/projects`) mengembalikan `401 Unauthorized` jika unauthenticated. |
| **2** | **Design System Tokyo Night** | Palet gelap `#1a1b26` dan terang `#eaeef6`, border radius konsisten `0rem` pada seluruh komponen UI (Button, Card, Input, Modal, Badge). |
| **3** | **6 Menu Global Sidebar** | Tepat enam navigasi utama pada sidebar global: `Dashboard`, `Tasks`, `Projects`, `Discussions`, `Agents`, `Team`. Sub-grup infra/sponsor dihapus. |
| **4** | **NavUser Footer** | Panel di bagian bawah sidebar menampilkan identitas admin aktif, modal dialog ganti password (`POST /api/v1/auth/password`), dan tombol logout aman. |
| **5** | **Contextual Project Sidebar** | Transisi mulus saat pengguna memilih proyek: menampilkan 7 modul esensial (`Overview`, `WordPress Cloud`, `Repos`, `Deployments`, `Telemetry`, `Studio Rooms`, `Settings`) plus tombol '← Navigasi Global'. |
| **6** | **Dashboard Empty State** | Tampilan edukatif bersih ketika sistem memiliki 0 proyek aktif (`activeProjects.length === 0`), menyajikan panduan dan tombol aksi langsung *Create New Project*. |
| **7** | **Breadcrumb Switcher model Jean** | Format `Dashboard / [Project Switcher Dropdown] / [Modul]` dengan pencarian instan, filter otomatis menyembunyikan proyek arsip, dan peralihan konteks 1-klik. |
| **8** | **Project Archiving** | Pemisahan tab *Active Projects* vs *Archived Projects*, fitur arsip (`PATCH /api/v1/projects/:id/archive`) dan pulihkan (`PATCH /api/v1/projects/:id/unarchive`). |
| **9** | **Multi-Project Task Linking** | Dropdown checkbox multi-proyek pada pembuatan/pengeditan task, relasi `task_projects` di PostgreSQL, filter task `?project_ids=...`, dan toggle view Kanban vs List. |

---

## 3. Alur Kerja Navigasi & Alih Konteks Proyek (D2 TALA)

```d2
direction: right

global_state: "Global Workspace State" {
  global_dashboard: "Global Dashboard Overview"
  projects_dir: "Projects Directory (Active / Archived Tabs)"
  tasks_board: "Universal Tasks Board (Kanban & List Modes)"
}

breadcrumb_action: "Breadcrumb Switcher Action" {
  click_switcher: "Click Switcher Dropdown"
  search_filter: "Instant Project Search"
  select_project: "Select Active Project"
}

project_context_state: "Project Context State" {
  contextual_sidebar: "Contextual Modules Sidebar"
  project_workspace: "Project Active Module Workspace"
  back_button: "Click '← Navigasi Global'"
}

global_state.projects_dir -> breadcrumb_action.click_switcher: "Select from Navbar"
breadcrumb_action.click_switcher -> breadcrumb_action.search_filter: "Filter by name/slug"
breadcrumb_action.search_filter -> breadcrumb_action.select_project: "Confirm Project"
breadcrumb_action.select_project -> project_context_state.contextual_sidebar: "Mount Project Modules"
project_context_state.contextual_sidebar -> project_context_state.project_workspace: "Render Context View"
project_context_state.project_workspace -> project_context_state.back_button: "Navigate Home"
project_context_state.back_button -> global_state.global_dashboard: "Return to Global Workspace"
```

---

## 4. Panduan Operasional Fitur Utama

### 4.1. Dashboard & Empty State
- **Kondisi 0 Proyek:** Saat instance Cuan baru pertama kali dideploy atau seluruh proyek diarsipkan, halaman `/dashboard` menampilkan komponen `DashboardEmptyState`. Pengguna disambut tombol *Create New Project* yang langsung membuka modal pembuatan proyek.
- **Kondisi Proyek Aktif:** Menampilkan kartu ringkasan 4 metrik operasional (*Active Projects*, *AI Agents*, *Running Deployments*, *Connected Servers*) serta kisi kartu proyek aktif dengan pintasan cepat menuju *Tasks* dan *Rooms*.

### 4.2. Direktori Proyek & Siklus Pengarsipan
- **Tab Active Projects:** Menampilkan daftar proyek yang sedang aktif bekerja. Proyek dapat dicari via kotak pencarian instan. Tombol *Arsipkan* akan memindahkan proyek ke status non-aktif dan mengecualikannya dari Breadcrumb Switcher.
- **Tab Archived Projects:** Menampilkan proyek-proyek terdahulu yang telah rampung atau dihentikan sementara. Tombol *Pulihkan* mengembalikan proyek ke tab aktif dan switcher.

### 4.3. Task Board: Kanban vs List & Multi-Project Linking
- **Toggle View Mode:** Di bagian kanan atas Task Board, terdapat tombol toggle antara **Kanban View** (tampilan kolom status *Backlog*, *Todo*, *In Progress*, *Testing*, *Done*) dan **List View** (tampilan tabel tabular terstruktur dengan progress checklist DoD).
- **Multi-Project Task Linking:** Saat membuat atau mengedit task via `DistillTaskModal`, pengguna dapat memilih lebih dari satu proyek pada dropdown checkbox `project_ids`. Task akan muncul di board masing-masing proyek yang ditautkan.
- **Edit Task:** Pengguna dapat mengklik tombol *Edit* pada kartu Kanban atau drawer detail task untuk memperbarui judul, deskripsi, prioritas, kriteria DoD, dan relasi proyek.

---

## 5. Prosedur Rilis Produksi di `cuan.dojo.cc` (`DOJO_TERMAPI_VPS`)

### 5.1. Kompilasi Binary & Frontend Minimalis
Di mesin development:
```bash
# 1. Kompilasi binary Go
./scripts/build.sh --backend-only

# 2. Kompilasi bundle React Vite frontend minimalis
(cd web && npm run build)
```

### 5.2. Pengiriman Binary & Bundle Frontend ke VPS
Gunakan SSH dan rsync terenkripsi:
```bash
# Transfer binary Cuan ke /opt/cuan/bin/
sshpass -p "$DOJO_TERMAPI_VPS_PASSWORD" scp -o StrictHostKeyChecking=no \
  bin/cuan-server bin/cuan-migrate bin/cuan-runner \
  $DOJO_TERMAPI_VPS_USERNAME@$DOJO_TERMAPI_VPS_PUBLIC_IP:/opt/cuan/bin/

# Transfer bundle web dist ke /opt/cuan/web/dist/
sshpass -p "$DOJO_TERMAPI_VPS_PASSWORD" rsync -avz --delete -e "ssh -o StrictHostKeyChecking=no" \
  web/dist/ \
  $DOJO_TERMAPI_VPS_USERNAME@$DOJO_TERMAPI_VPS_PUBLIC_IP:/opt/cuan/web/dist/
```

### 5.3. Validasi Migrasi Database & Restart Service
```bash
# Verifikasi migrasi 000019 utuh di host DOJO_TERMAPI_VPS
sshpass -p "$DOJO_TERMAPI_VPS_PASSWORD" ssh -o StrictHostKeyChecking=no \
  $DOJO_TERMAPI_VPS_USERNAME@$DOJO_TERMAPI_VPS_PUBLIC_IP \
  "export \$(grep -E '^DATABASE_URL=' /opt/cuan/.env | xargs) && /opt/cuan/bin/cuan-migrate --direction status"

# Restart systemd service cuan
sshpass -p "$DOJO_TERMAPI_VPS_PASSWORD" ssh -o StrictHostKeyChecking=no \
  $DOJO_TERMAPI_VPS_USERNAME@$DOJO_TERMAPI_VPS_PUBLIC_IP \
  "sudo systemctl restart cuan && sudo systemctl is-active cuan"
```

### 5.4. Rangkaian Smoke Tests Publik
Jalankan pengujian dari terminal:
```bash
# 1. Liveness health check (200 OK)
curl -sS -i https://cuan.dojo.cc/healthz

# 2. Readiness check & migrations count (200 OK, migrations_count: 19)
curl -sS -i https://cuan.dojo.cc/readyz

# 3. Application info check (200 OK, Author: Buchin <buchin@dropsugar.com>)
curl -sS -i https://cuan.dojo.cc/api/info

# 4. Auth login default admin check (200 OK, token issued)
curl -sS -i -X POST https://cuan.dojo.cc/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"whosyourdaddy"}'

# 5. Caddy On-Demand TLS gatekeeper probe (200 OK)
curl -sS -i https://cuan.dojo.cc/api/caddy/ask?domain=cuan.dojo.cc

# 6. WebSocket PTY terminal handshake (101 Switching Protocols)
wscat -c wss://cuan.dojo.cc/api/tasks/live-drill-s15/terminal
```

---

## 6. Standar Kepatuhan Produk (§11) & Matriks DoD

- **Mandiri (*Self-Contained*):** Seluruh konfigurasi, prosedur deploy, artefak, dan kontrak REST terdokumentasi tanpa ketergantungan sesi internal.
- **Zero-Friction:** Pengguna langsung masuk ke workspace kerja terproteksi tanpa distraksi landing page, toggle mode rumit, atau iklan promosi.
- **100% Data Retention:** Seluruh data Sprint 1–14 di database PostgreSQL tetap utuh 100% dengan total 19 migrasi skema.
- **Tokyo Night Polish:** Radius 0rem dan kontras tinggi memastikan tampilan seragam di seluruh resolusi layar desktop maupun mobile.
