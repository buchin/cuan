# Cuan — Panduan Arsitektur & Operasional Clean Slate Linear Workplace

## 1. Ikhtisar & Mental Model Produk

Cuan mentransformasi pengalaman manajemen proyek menjadi **Linear-Class Agentic Workplace** yang dirancang khusus untuk kreator, solopreneur, dan pengembang teknis.

Prinsip utama antarmuka baru Cuan adalah **Zero Friction, Zero Infrastructure Clutter**:
* **Projects sebagai Entitas Bisnis Utama:** Tempat pengguna mengorganisir produk digital, blog konten, kanal video, atau repositori software.
* **Task Board Terpadu (Kanban & List):** Alur kerja Backlog → To Do → In Progress → In Review → Completed yang terhubung langsung dengan AI Agent.
* **Side-Peek Execution Panel:** Mengklik task membuka panel samping instan tanpa modal berlapis yang mengaburkan board.
* **Unified Settings:** Konfigurasi teknis seperti Git Repository, Custom Domain (BYOD), dan Host Server VPS (BYOS) dipusatkan di tab Settings proyek, menjaga navigasi global tetap bersih.

---

## 2. Diagram Alur Kerja Pengguna (D2 TALA)

```d2
direction: right

user: "Pengguna / Developer"

auth_gate: "Auth Gatekeeper" {
  login_screen: "Tokyo Night Login (/login)"
  auth_verify: "Token Verification (/api/v1/auth/me)"
}

app_shell: "Linear-Class App Shell" {
  header: "Clean Header (Breadcrumb Switcher + Quick Actions)"
  sidebar: "Slim Navigation (Projects, Tasks, Agents, Settings)"
}

workplace_views: "Feature Workspaces" {
  projects_hub: "Projects Hub (Dual View Grid / List)"
  task_board: "Kanban Task Board (5 Columns)"
  side_peek: "Side-Peek Panel (Brief + Agent Terminal)"
  settings_hub: "Project Settings (Repo, BYOD, BYOS, Secrets)"
}

user -> auth_gate.login_screen: "1. Akses Platform (/)"
auth_gate.login_screen -> auth_gate.auth_verify: "2. POST /api/v1/auth/login"
auth_gate.auth_verify -> app_shell.header: "3. Direct Access ke Dashboard"
app_shell.sidebar -> workplace_views.projects_hub: "Pilih / Buat Proyek"
workplace_views.projects_hub -> workplace_views.task_board: "Buka Board Proyek"
workplace_views.task_board -> workplace_views.side_peek: "Klik Kartu Task"
workplace_views.side_peek -> workplace_views.settings_hub: "Kelola Parameter Proyek"
```

---

## 3. Komponen Utama Arsitektur Frontend

### A. Projects Hub & Contextual Switcher
* **Dual View Mode:** Menyediakan toggle instan antara **Grid View** (kartu visual dengan badge status dan link repo) dan **List View** (tampilan padat ala Linear/GitHub Projects).
* **Task Counter:** Setiap proyek menampilkan jumlah tugas aktif secara dinamis.
* **Model Jean Breadcrumb Switcher:** Navigasi di header `Cuan / [Project Name ▾] / [Current Tab]` memungkinkan pengguna berpindah antar proyek secara instan tanpa memuat ulang halaman (*zero page reload*).

### B. Agentic Task Board & Side-Peek Drawer
* **5 Kolom Kanban Standar:**
  1. `Backlog` (Warna: Slate `#565f89`)
  2. `To Do` (Warna: Tokyo Blue `#7aa2f7`)
  3. `In Progress` (Warna: Yellow-Orange `#e0af68`)
  4. `In Review` (Warna: Purple `#bb9af7`)
  5. `Completed` (Warna: Green `#9ece6a`)
* **1-Click Status Advance:** Tombol `ChevronRight` pada kartu task memungkinkan percepatan status tanpa perlu drag-and-drop manual.
* **Side-Peek Execution Panel:** 
  * Tab 1: Brief, deskripsi, prioritas, dan checklist Definition of Done (DoD).
  * Tab 2: Terminal live stream eksekusi AI Agent runner (`POST /api/v1/tasks/{id}/run`).

### C. Unified Settings & Integrations Hub
* Menghilangkan seluruh menu terpisah untuk server dan domain dari sidebar global.
* Mengonsolidasikan 4 pilar teknis dalam satu formulir bersih per proyek:
  1. **Git Repository:** URL repositori dan default branch.
  2. **Custom Domain (BYOD):** Nama domain kustom dengan provisi otomatis Let's Encrypt TLS via reverse proxy Caddy.
  3. **Deployment Host (BYOS):** Alamat IP atau hostname VPS target.
  4. **Environment Variables & Secrets:** Editor pasangan Key-Value aman untuk konfigurasi runtime aplikasi.

---

## 4. Keamanan & Retensi Data

* **100% Backward Compatible:** Seluruh tabel database PostgreSQL (19 migrasi dari Sprint 1 hingga Sprint 15) tetap utuh 100%. Tidak ada penghapusan data skema lama.
* **Clean Bundle Size:** Ukuran bundle JavaScript produksi terpangkas dari **1.164 kB** menjadi **264 kB** (pengurangan 78%), mengeliminasi lag dan beban memori browser.
* **Auth Gatekeeper:** Rute dilindungi secara default. Kredensial default admin: `admin` / `whosyourdaddy`, dilengkapi fitur rotasi password mandiri di NavUser.
