# Panduan Operasional UI Navigation, Auth Gatekeeper & Multi-Project Workspace OS

Dokumen ini merupakan panduan operasional mandiri (*self-contained operational guide*) berstandar §11 bagi pengguna mandiri (*Solo Developer*), operator agensi digital, pengelola multi-brand studio, dan administrator sistem mengenai arsitektur sistem otentikasi tunggal (*Single-Owner Auth Gatekeeper*), manajemen sesi, sistem desain *Tokyo Night Theme Engine*, navigasi global berbasis *shadcn Sidebar-07*, *Breadcrumb Project Switcher*, pengarsipan proyek (*Project Archiving Lifecycle*), relasi keterikatan multi-proyek (*Multi-Project Task Linking*), serta prosedur rilis produksi di `cuan.dojo.cc` (`DOJO_TERMAPI_VPS`).

---

## 1. Filosofi & Nilai Produk: "Single-Owner Auth & Multi-Project OS"

Saat sebuah platform *developer self-hosted* seperti Cuan di-deploy ke VPS publik (`cuan.dojo.cc`), platform menghadapi tantangan arsitektur dan pengalaman pengguna:
1. **Risiko Akses Publik Tanpa Proteksi:** Seluruh panel manajemen runner, deployment WordPress, terminal PTY, brankas rahasia (*Secrets Vault*), dan telemetri server membutuhkan gerbang otentikasi yang kokoh agar tidak dapat diakses pihak ketiga di Internet publik.
2. **Kebutuhan Multi-Project Terisolasi namun Terpadu:** Pengembang solo sering mengelola beberapa proyek portofolio, situs affiliate, maupun SaaS klien secara simultan. Memisahkan instance untuk tiap proyek sangat memboroskan resource VPS, sementara menggabungkan semua task tanpa penanda proyek menimbulkan kekacauan visual (*clutter*).
3. **Siklus Hidup Proyek (Active vs Archived):** Proyek yang telah selesai atau sedang di-pause membutuhkan fasilitas pengarsipan visual agar tidak mengaburkan fokus harian di dashboard, namun tetap dapat diakses kembali sewaktu-waktu (*unarchive*).
4. **Keterikatan Task Lintas Proyek (*Cross-Project Tasks*):** Pekerjaan infrastruktur umum (misal: "Pembaruan script tracking ke seluruh website" atau "Audit sertifikat SSL bersama") perlu ditautkan ke lebih dari satu proyek secara relasional.
5. **Estetika Kelas Dunia (Tokyo Night & Clean Slate):** Desain UI modern membutuhkan palet warna yang nyaman untuk mata pengembang dalam sesi kerja panjang, dengan layout *zero-border radius* yang tegas, tajam, dan responsif.

**Solusi Cuan Sprint 14:**
- **Zero-Friction Default Admin Seeding:** Sistem secara otomatis menyediakan akun default aman `admin` / `whosyourdaddy` pada inisialisasi skema database 000019, memungkinkan login instan pertama kali tanpa *manual SQL patching*.
- **Tokyo Night Theme Engine:** Implementasi palet tema Tokyo Night adaptif (*Light #eaeef6 / Dark #1a1b26*) dengan radius `0rem` pada seluruh komponen UI shadcn.
- **shadcn Sidebar-07 Shell:** Navigasi 2-tingkat dengan 6 menu global (Dashboard, Tasks, Projects, Discussions, Agents, Team) dan modul proyek kontekstual (Overview, WordPress Cloud, Repos, Deployments, Telemetry, Studio Rooms, Settings).
- **Breadcrumb Project Switcher:** Navbar breadcrumb interaktif yang memungkinkan alih konteks antar-proyek dalam 1-klik dari halaman mana pun, dilengkapi pencarian instan dan filter pills (*Semua*, *Aktif*, *Arsip*).
- **Relational Multi-Project Task Engine:** Tabel relasi `task_projects` untuk mengaitkan 1 task ke $N$ proyek, didukung visual badges dan filter URL query (`?project_ids=...`).
- **100% Data Retention:** Migrasi database 000019 bersifat murni aditif dan non-destruktif, menjaga 100% integritas data Sprint 1 hingga Sprint 13.

---

## 2. Diagram Arsitektur E2E UI Navigation, Auth Gatekeeper & Multi-Project (D2 TALA)

Diagram berikut merinci arsitektur alur kerja otentikasi, session store, navigasi shell, dan penyimpanan multi-proyek di Cuan Core Engine:

```d2
direction: right

user: "User / Solo Developer" {
  browser: "Browser Client (cuan.dojo.cc)" {
    login_gate: "Login Screen (Tokyo Night)"
    sidebar07: "Sidebar-07 App Shell"
    switcher: "Breadcrumb Project Switcher"
    task_board: "Multi-Project Task Kanban"
    pwd_modal: "Change Password Dialog"
  }
}

gateway: "Edge Ingress Gateway" {
  nginx: "Nginx Reverse Proxy (SSL 443)"
  caddy: "Caddy On-Demand TLS Gatekeeper"
}

cuan_server: "Cuan Backend Control Plane (Go :8080)" {
  auth_handler: "Auth HTTP Handler (/api/v1/auth)"
  session_mgr: "Session Validator & Context Middleware"
  proj_svc: "Project Service & Archiving Engine"
  task_svc: "Multi-Project Task Linking Service"
  router: "Gorilla Mux Protected Router"
}

storage: "Persistence Layer (PostgreSQL)" {
  t_owners: "owners (Username, Bcrypt Hash)"
  t_sessions: "owner_sessions (SHA-256 Token Hash)"
  t_projects: "projects (is_archived BOOLEAN)"
  t_task_proj: "task_projects (ManyToMany Mapping)"
  t_legacy: "Historical Sprint 1-13 Tables (Intact)"
}

user.browser.login_gate -> gateway.nginx: "1. POST /api/v1/auth/login (admin/whosyourdaddy)"
gateway.nginx -> cuan_server.auth_handler: "2. Forward Authenticate Request"
cuan_server.auth_handler -> storage.t_owners: "3. Verify Bcrypt Hash"
cuan_server.auth_handler -> storage.t_sessions: "4. Create Session Token"
cuan_server.auth_handler -> user.browser.sidebar07: "5. Set Token & Render AppShell"
user.browser.pwd_modal -> cuan_server.auth_handler: "6. POST /api/v1/auth/password"
cuan_server.auth_handler -> storage.t_owners: "7. Update Password Hash"
user.browser.switcher -> cuan_server.proj_svc: "8. GET /api/v1/projects?include_archived=true"
cuan_server.proj_svc -> storage.t_projects: "9. Query Filtered Projects"
user.browser.task_board -> cuan_server.task_svc: "10. POST /api/v1/tasks (Multi-Project IDs)"
cuan_server.task_svc -> storage.t_task_proj: "11. Insert task_projects Relational Rows"
```

---

## 3. Spesifikasi Teknis Auth Layer & Session Management

### 3.1. Kredensial Default & Auto-Seed
Pada migrasi `000019_auth_and_multiproject.up.sql`, tabel `owners` diperkaya dengan kolom `username VARCHAR(100)` dan indeks unik. Jika belum ada akun owner di database, baris default diinjeksikan secara otomatis:
- **Username Default:** `admin`
- **Email Default:** `admin@cuan.local`
- **Password Default:** `whosyourdaddy`
- **Bcrypt Hash:** `$2a$10$sGcQkuIKOs1j7.lrnh.HzO.5/nm.8sPY2gHjofWzd.vxdyTw3F6/u` (Cost 10)

### 3.2. Session Token & Durasi Masa Berlaku
Sesi otentikasi menggunakan token acak 32-byte hex yang disimpan pada client (localStorage: `cuan_session_token`) dan dikirimkan via HTTP Authorization Header:
```http
Authorization: Bearer <token_hex>
```
Pada database `owner_sessions`, token disimpan dalam bentuk hash SHA-256 (`token_hash`) dengan masa aktif `SessionDuration = 7 hari` (168 jam).

### 3.3. Spesifikasi Endpoint REST API Auth
| Endpoint | Method | Proteksi | Deskripsi |
| :--- | :--- | :--- | :--- |
| `/api/v1/auth/status` | `GET` | Publik | Cek apakah setup owner sudah dilakukan (`setup_required`, `owner_configured`). |
| `/api/v1/auth/login` | `POST` | Publik | Otentikasi dengan payload `{"username": "admin", "password": "..."}` atau email. |
| `/api/v1/auth/password` | `POST` | Bearer Token | Ubah password: `{"current_password": "...", "new_password": "..."}` (min 8 karakter). |
| `/api/v1/auth/me` | `GET` | Bearer Token | Mengambil metadata user/owner aktif yang terotentikasi. |
| `/api/v1/auth/logout` | `POST` | Bearer Token | Invalidation session token di database `owner_sessions`. |

---

## 4. Desain Sistem Tokyo Night & Navigasi AppShell

### 4.1. Tokyo Night CSS Theme Variables
Sistem tema mematuhi spesifikasi [tweakcn Tokyo Night](https://tweakcn.com/themes/cmvbdtdtm000004jp89rgbo5w?p=dashboard#) dengan `border-radius: 0rem`:

```css
/* Tokyo Night Light Mode */
:root {
  --background: 220 40% 94.1%;          /* #eaeef6 */
  --foreground: 226 34.4% 24.5%;        /* #293354 */
  --card: 0 0% 100%;                    /* #ffffff */
  --card-foreground: 226 34.4% 24.5%;   /* #293354 */
  --primary: 224 58.7% 49.4%;           /* #355dc6 */
  --primary-foreground: 0 0% 100%;
  --muted: 220 20% 88%;                 /* #dbe2ed */
  --muted-foreground: 225 15% 45%;      /* #636f88 */
  --border: 220 25% 82%;                /* #c8d3e3 */
  --radius: 0rem;
}

/* Tokyo Night Dark Mode */
.dark {
  --background: 235 19% 13%;            /* #1a1b26 */
  --foreground: 225 25% 88%;            /* #c0caf5 */
  --card: 235 18% 16%;                  /* #202231 */
  --card-foreground: 225 25% 88%;       /* #c0caf5 */
  --primary: 220 95% 73%;               /* #7ba4fb */
  --primary-foreground: 235 19% 13%;
  --muted: 235 16% 22%;                 /* #2c3044 */
  --muted-foreground: 224 15% 60%;      /* #8993b3 */
  --border: 235 15% 25%;                /* #343952 */
  --radius: 0rem;
}
```

### 4.2. Arsitektur Navigasi shadcn Sidebar-07
1. **Navigasi Global (Level 1):**
   - **Dashboard** (`/dashboard`): Ringkasan status platform, metrik aktif, dan status runner.
   - **Tasks** (`/tasks`): Multi-Project Kanban board terpadu.
   - **Projects** (`/projects`): Direktori workspace dan katalog proyek.
   - **Discussions** (`/discussions`): Ruang diskusi tim dan distilasi task.
   - **Agents** (`/agents`): Manajemen persona agen AI otonom.
   - **Team** (`/team`): Kolaborator manusia dan autonomous agent crew.
2. **Navigasi Kontekstual Proyek (Level 2):**
   Saat pengguna memilih sebuah proyek, sidebar bertransformasi ke modul spesifik proyek:
   - `Overview` | `WordPress Cloud` | `Repos` | `Deployments` | `Telemetry` | `Studio Rooms` | `Settings`.
   - Tombol `← Navigasi Global` di bagian atas sidebar mengembalikan konteks ke navigasi global.
3. **Footer Sidebar (NavUser):**
   Menampilkan avatar inisial, username admin aktif, tombol Profil (membuka dialog ganti password), dan tombol Logout.

---

## 5. Multi-Project Workspace & Relasi Task Linking

### 5.1. Siklus Hidup Pengarsipan Proyek (Project Archiving)
Proyek memiliki status boolean `is_archived`:
- **Default Query:** `GET /api/v1/projects` secara default hanya mengembalikan proyek aktif (`is_archived = false`).
- **Include Archived:** Parameter `?include_archived=true` menyertakan proyek yang diarsipkan.
- **Arsipkan Proyek:** `PATCH /api/v1/projects/{id}/archive` (Status 200 OK).
- **Aktifkan Kembali:** `PATCH /api/v1/projects/{id}/unarchive` (Status 200 OK).

### 5.2. Relasi Multi-Project Task Linking (`task_projects`)
Skema relasional PostgreSQL many-to-many:
```sql
CREATE TABLE IF NOT EXISTS task_projects (
    task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (task_id, project_id)
);
```
- **Pembuatan Task:** Request `POST /api/v1/tasks` menerima array `project_ids: ["proj_1", "proj_2"]`.
- **Filtering:** Request `GET /api/v1/tasks?project_ids=proj_1` menyaring task yang tertaut ke proyek tersebut.
- **Visual Kanban:** Kartu task menampilkan badge nama proyek dengan palet warna teridentifikasi.

---

## 6. Prosedur Rilis Produksi di `cuan.dojo.cc` (`DOJO_TERMAPI_VPS`)

### 6.1. Tahap 1: Eksekusi Migrasi Database 000019
Migrasi dieksekusi menggunakan binary `cuan-migrate` terkompilasi langsung pada host produksi:
```bash
# Di host DOJO_TERMAPI_VPS
export $(grep -E '^DATABASE_URL=' /opt/cuan/.env | xargs)
/opt/cuan/bin/cuan-migrate --direction up
```
Verifikasi tabel dan jumlah migrasi:
```sql
SELECT count(*) FROM cuan_schema_migrations; -- Harus mengembalikan 19
SELECT id, username, email FROM owners WHERE id = 'owner_admin';
```

### 6.2. Tahap 2: Kompilasi & Pengiriman Binary & Bundle Frontend
```bash
# Kompilasi lokal di mesin development
./scripts/build.sh --backend-only
(cd web && npm run build)

# Transfer binary dan web/dist ke VPS produksi
sshpass -p "$DOJO_TERMAPI_VPS_PASSWORD" scp -o StrictHostKeyChecking=no \
  bin/cuan-server bin/cuan-migrate bin/cuan-runner \
  $DOJO_TERMAPI_VPS_USERNAME@$DOJO_TERMAPI_VPS_PUBLIC_IP:/opt/cuan/bin/

sshpass -p "$DOJO_TERMAPI_VPS_PASSWORD" rsync -avz -e "ssh -o StrictHostKeyChecking=no" \
  web/dist/ \
  $DOJO_TERMAPI_VPS_USERNAME@$DOJO_TERMAPI_VPS_PUBLIC_IP:/opt/cuan/web/dist/
```

### 6.3. Tahap 3: Restart Service & Validasi Liveness
```bash
sshpass -p "$DOJO_TERMAPI_VPS_PASSWORD" ssh -o StrictHostKeyChecking=no \
  $DOJO_TERMAPI_VPS_USERNAME@$DOJO_TERMAPI_VPS_PUBLIC_IP \
  "sudo systemctl restart cuan && sudo systemctl is-active cuan"
```

### 6.4. Tahap 4: Smoke Tests Publik
Jalankan rangkaian smoke test dari luar jaringan VPS:
1. `curl -sS -i https://cuan.dojo.cc/healthz` (Status: `healthy`, 200 OK)
2. `curl -sS -i https://cuan.dojo.cc/readyz` (Status: `ready`, `migrations_count: 19`, 200 OK)
3. `curl -sS -i https://cuan.dojo.cc/api/info` (Status: 200 OK)
4. `curl -sS -i https://cuan.dojo.cc/api/v1/auth/status` (Status: `owner_configured: true`, 200 OK)
5. `curl -sS -i -X POST https://cuan.dojo.cc/api/v1/auth/login -H "Content-Type: application/json" -d '{"username":"admin","password":"whosyourdaddy"}'` (Status: 200 OK, token returned)
6. Uji koneksi WebSocket terminal PTY: `wss://cuan.dojo.cc/api/tasks/live-drill-s14/terminal` (Status: 101 Switching Protocols).

---

## 7. Rangkuman Verifikasi Kepatuhan Standar Produk (§11)

- **Mandiri (*Self-Contained*):** Panduan ini memuat seluruh perintah, kontrak REST, variabel CSS, diagram D2 TALA, dan langkah verifikasi yang dapat dieksekusi secara independen.
- **Zero-Friction:** Pengguna langsung mendapatkan sistem siap pakai dengan akun admin default dan antarmuka Tokyo Night yang intuitif.
- **Data Retention Terjamin:** Migrasi 000019 murni aditif dan seluruh 34 tabel historis Sprint 1 hingga Sprint 13 tetap utuh 100%.
