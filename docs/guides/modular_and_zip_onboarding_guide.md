# Panduan Operasional Modular Workspace OS, AI Script Onboarding & Creator Publishing Hub

Dokumen ini merupakan panduan operasional mandiri bagi pengembang mandiri (*Solo Developer*), pembeli skrip online, kreator konten, dan pemilik instance Cuan (*Single-Owner*) mengenai arsitektur, konfigurasi, alur kerja onboarding source code dari file ZIP (*AI Script Onboarding*), navigasi modular adaptif (*Dynamic Workspace OS*), integrasi penerbitan konten ke WordPress REST API (*CMS Publishing Engine*), penggunaan Teleprompter fullscreen, serta pengoperasian produksi di domain `cuan.dojo.cc`.

---

## 1. Filosofi & Nilai Produk: "Modular Workspace OS for Creators & Script Owners"

Cuan Sprint 11 mentransformasi Cuan menjadi sistem operasi ruang kerja modular (*Modular Workspace OS*) yang menyelesaikan pain point krusial pengguna non-teknis maupun kreator konten:

1. **Zero-Friction Script Onboarding:** Jutaan orang membeli source code web application di marketplace seperti CodeCanyon, Envato, atau GitHub, namun script tersebut sering mengendap di penyimpanan lokal karena pembeli kesulitan mengonfigurasi file `.env`, mengimpor skema database `.sql`, dan memasang dependensi runtime (`composer`/`npm`). Di Cuan, pengguna cukup melakukan drag-and-drop file `.zip`. Backend AI Inspector secara otomatis mendeteksi tech stack, membaca variabel environment yang dibutuhkan, mendeteksi file SQL, dan menyusun task pipeline instalasi terpandu di Kanban board.
2. **Dynamic & Decluttered Workspace:** Berbeda dengan software project management yang penuh bloatware, antarmuka Cuan beradaptasi secara dinamis sesuai modul yang aktif (`enabled_modules`) pada proyek yang dibuka. Proyek konten murni hanya menampilkan modul Tasks dan Discussions tanpa menu Server atau Deployer yang membingungkan.
3. **In-App Modular Expansion:** Pengguna dapat mengaktifkan atau menonaktifkan modul kapan saja melalui modal **"+ Add Modules"** (misal: mengaktifkan modul Servers ketika proyek siap di-deploy ke VPS).
4. **Secure 1-Click WordPress Publishing:** Konten atau naskah artikel yang disusun oleh agen AI di dalam kartu task dapat langsung dipublikasikan ke website WordPress tujuan (sebagai draf maupun artikel terbit) melalui integrasi resmi WordPress REST API dengan enkripsi kredensial AES-256-GCM.
5. **Fullscreen Teleprompter View:** Naskah skrip video hasil generate agen dapat langsung dibaca di layar teleprompter dengan auto-scroll kecepatan terkontrol, flip mirror mode, dan kontrol shortcut keyboard.
6. **100% Data Retention Guarantee:** Skema database migrasi `000016_modular_workspaces_and_publishing.up.sql` bersifat murni aditif non-destruktif, menjaga 100% integritas seluruh data historis dari Sprint 1 hingga Sprint 10.

---

## 2. Diagram Arsitektur E2E Modular Workspace & Script Onboarding (D2 TALA)

Diagram berikut mengilustrasikan alur kerja lengkap dari penerimaan berkas ZIP script, deteksi stack dan konfigurasi, pembentukan proyek modular di PostgreSQL, penyesuaian navigasi antarmuka, hingga penerbitan konten ke WordPress dan deployment ke VPS:

```d2
direction: right

user: "User / Script Owner / Creator" {
  ui: "Cuan Studio Web (Port 8080 / 8081)" {
    zip_wizard: "ZIP Onboarding Wizard (/studio)"
    dynamic_nav: "Reactive Sidebar & Navbar"
    module_toggle: "+ Add Modules Modal"
    publisher: "1-Click WordPress Publisher Drawer"
    teleprompter: "Fullscreen Teleprompter View"
  }
}

gateway: "Edge Gateway (cuan.dojo.cc)" {
  nginx: "Nginx SSL Reverse Proxy"
  caddy_gate: "Caddy On-Demand TLS Gatekeeper"
}

cuan_core: "Cuan Backend Control Plane (Go)" {
  unpacker: "Safe ZIP Unpacker (Zip-Slip Defense)"
  detector: "Stack Detector (Laravel/Node/Python/WP/HTML)"
  parser: "Env & SQL Parser (.env.example / *.sql)"
  pipeline: "Setup Task Pipeline Generator"
  pub_engine: "CMS Publishing Engine (WP REST Client)"
  vault: "KeyVault (AES-256-GCM Encryption)"
}

destinations: "External Environments & Targets" {
  wp_site: "WordPress Site (REST API /wp/v2/posts)"
  vps_node: "Remote BYOS Worker (DOJO_TERMAPI_VPS)"
}

database: "PostgreSQL Database (Migration 000016)" {
  projects_tbl: "projects (enabled_modules, source_type, source_metadata)"
  integrations_tbl: "project_integrations (config_encrypted)"
  tasks_tbl: "project_tasks (scheduled_at, published_at, publication_url)"
  sprint_retention: "Sprint 1-10 Historical Records (100% Intact)"
}

user.ui.zip_wizard -> gateway.nginx: "1. Upload script.zip via Dropzone"
gateway.nginx -> cuan_core.unpacker: "2. POST /api/v1/projects/from-zip"
cuan_core.unpacker -> cuan_core.detector: "3. Scan signatures in isolated sandbox"
cuan_core.detector -> cuan_core.parser: "4. Extract .env.example & parse .sql schemas"
cuan_core.parser -> cuan_core.pipeline: "5. Determine stack & recommended modules"
cuan_core.pipeline -> database.projects_tbl: "6. Insert modular project record"
cuan_core.pipeline -> database.tasks_tbl: "7. Generate setup checklist & guidance tasks"

cuan_core.pipeline -> user.ui.dynamic_nav: "8. Return project with enabled_modules"
user.ui.module_toggle -> gateway.nginx: "9. PATCH /api/v1/projects/:id/modules"
gateway.nginx -> database.projects_tbl: "10. Update enabled_modules JSONB"

user.ui.publisher -> gateway.nginx: "11. POST /api/v1/tasks/:id/publish"
gateway.nginx -> cuan_core.pub_engine: "12. Initiate content publication"
cuan_core.pub_engine -> cuan_core.vault: "13. Decrypt WordPress Application Password"
cuan_core.pub_engine -> destinations.wp_site: "14. POST /wp/v2/posts with Basic Auth"
destinations.wp_site -> database.tasks_tbl: "15. Update published_at & publication_url"
```

---

## 3. Struktur Database & Model Data (Migrasi 000016)

Skema database migrasi `migrations/000016_modular_workspaces_and_publishing.up.sql` menambahkan ekstensi aditif non-destruktif ke tabel `projects` dan `project_tasks`, serta membuat tabel baru `project_integrations`:

```sql
-- Cuan Migration 000016: Modular Workspace OS, Integrations & Creator Publishing Hub
-- (000016_modular_workspaces_and_publishing.up.sql)

ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS enabled_modules JSONB NOT NULL DEFAULT '["tasks", "discussions"]'::jsonb,
    ADD COLUMN IF NOT EXISTS source_type VARCHAR(64) NOT NULL DEFAULT 'blank',
    ADD COLUMN IF NOT EXISTS source_metadata JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX IF NOT EXISTS idx_projects_source_type ON projects(source_type);

CREATE TABLE IF NOT EXISTS project_integrations (
    id VARCHAR(64) PRIMARY KEY,
    project_id VARCHAR(64) NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    integration_type VARCHAR(64) NOT NULL,
    config_encrypted TEXT NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_project_integrations_project_id ON project_integrations(project_id);
CREATE INDEX IF NOT EXISTS idx_project_integrations_type ON project_integrations(integration_type);
CREATE INDEX IF NOT EXISTS idx_project_integrations_status ON project_integrations(status);

ALTER TABLE project_tasks
    ADD COLUMN IF NOT EXISTS scheduled_at TIMESTAMP WITH TIME ZONE NULL,
    ADD COLUMN IF NOT EXISTS published_at TIMESTAMP WITH TIME ZONE NULL,
    ADD COLUMN IF NOT EXISTS publication_url VARCHAR(512) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_project_tasks_scheduled_at ON project_tasks(scheduled_at);
CREATE INDEX IF NOT EXISTS idx_project_tasks_published_at ON project_tasks(published_at);
```

### Penjelasan Kolom dan Tabel Kunci:
- **`projects.enabled_modules`**: Array JSONB yang mendefinisikan menu dan modul yang aktif untuk proyek tersebut (contoh: `["tasks", "discussions", "servers", "repos"]`).
- **`projects.source_type`**: Indikator asal proyek (`blank`, `template`, atau `zip`).
- **`projects.source_metadata`**: Informasi ekstraksi berkas ZIP (tech stack, versi framework, path berkas `.env.example`, daftar tabel database dari file `.sql`, dsb.).
- **`project_integrations`**: Tabel penyimpanan integrasi pihak ketiga per proyek (misal `integration_type: "wordpress"`), dengan konfigurasi sensitif (Application Password) terenkripsi AES-256-GCM pada `config_encrypted`.
- **`project_tasks.published_at` & `publication_url`**: Jejak audit publikasi artikel langsung ke CMS WordPress.

---

## 4. Panduan AI Script Onboarding (New Project from ZIP)

### 4.1. Alur Melalui Web Interface
1. Buka antarmuka Studio di `https://cuan.dojo.cc/studio` (atau port 8080/8081).
2. Klik tombol **"+ New Project"** dan pilih opsi **"From Script (ZIP)"**.
3. Drag-and-drop file script `.zip` (maksimum 150 MB).
4. Wizard akan menampilkan animasi pemrosesan:
   - **Unpacking & Security Audit:** Memverifikasi integritas arsip dan mencegah eksploitasi path traversal (*Zip-Slip*).
   - **Stack Detection:** Menganalisis berkas manifest (`composer.json`, `package.json`, `requirements.txt`, `wp-config.php`, dsb.).
   - **Configuration Parsing:** Membaca variabel environment pada `.env.example` dan mendeteksi skema `.sql`.
5. Tinjau kartu ringkasan hasil deteksi (Stack terdeteksi, versi PHP/Node yang disarankan, database requirement, dan modul aktif).
6. Klik **"Create Project & Launch Pipeline"**. Proyek langsung terbentuk dengan daftar task persiapan siap jalan.

### 4.2. Alur Melalui REST API
Endpoint inspeksi pra-buat:
```http
POST /api/v1/projects/inspect-zip
Content-Type: multipart/form-data

file: <binary zip content>
```
Respon JSON (200 OK):
```json
{
  "detected_stack": "laravel",
  "framework_version": "11.x",
  "database_type": "mysql",
  "sql_files": ["database/schema.sql"],
  "env_vars": ["DB_DATABASE", "DB_USERNAME", "DB_PASSWORD", "APP_KEY"],
  "recommended_modules": ["tasks", "discussions", "servers", "repos"]
}
```

Endpoint pembuatan proyek langsung dari ZIP:
```http
POST /api/v1/projects/from-zip
Content-Type: multipart/form-data

file: <binary zip content>
name: My CodeCanyon Script Project
```

---

## 5. Panduan Navigasi Modular & In-App Module Toggle

Navigasi sidebar dan navbar secara otomatis membaca atribut `enabled_modules` milik proyek yang sedang aktif.

### 5.1. Daftar Modul Tersedia
| Modul | Deskripsi | Menu Sidebar |
| --- | --- | --- |
| `tasks` | Kanban board pengelolaan tugas dan eksekusi AI | Tasks, Kanban Board |
| `discussions` | Chat room diskusi tim dan thread kolaboratif | Discussions, Channels |
| `servers` | Manajemen server remote BYOS via SSH | Servers, Infrastructure |
| `repos` | Integrasi Git repository dan file browser | Repositories, Branches |
| `deployments` | Universal App Deployer dan proxy routing | Deployments, Domains |
| `campaigns` | Batch campaign engine & multi-project fan-out | Campaigns, Rollout |
| `creator` | Creator blueprints, teleprompter, dan naskah | Creator Hub, Blueprints |

### 5.2. Mengubah Modul Proyek Aktif
Pengguna dapat mengklik tombol **"+ Add Modules"** pada header studio untuk memunculkan modal seleksi modul, atau menggunakan REST API:
```http
PATCH /api/v1/projects/{project_id}/modules
Content-Type: application/json

{
  "enabled_modules": ["tasks", "discussions", "servers", "deployments"]
}
```

---

## 6. Panduan Integrasi WordPress REST API & Content Publishing

Cuan menyediakan CMS Publishing Engine bawaan untuk mempublikasikan draft naskah agen langsung ke blog WordPress self-hosted tanpa copy-paste manual.

### 6.1. Menghubungkan Website WordPress
1. Masuk ke dashboard admin WordPress tujuan (`/wp-admin`).
2. Masuk ke menu **Users > Profile**.
3. Gulir ke bagian **Application Passwords**, beri nama aplikasi (misal: `Cuan Engine`), lalu klik **Add New Application Password**.
4. Salin sandi 24 karakter yang dihasilkan.
5. Di Cuan Studio, buka task atau drawer pengaturan integrasi proyek, lalu masukkan:
   - **WordPress Site URL:** `https://myblog.com`
   - **Username:** Nama pengguna admin/editor WordPress
   - **Application Password:** Sandi aplikasi yang dibuat
6. Klik **Test & Save Integration**. Kredensial akan diuji terhadap endpoint `/wp/v2/users/me` dan dienkripsi dengan AES-256-GCM.

### 6.2. Mempublikasikan Konten dari Task
Pada Task Detail Drawer:
1. Pastikan tab **Agent Output** telah memuat konten naskah/artikel.
2. Buka tab **Publish to WordPress**.
3. Pilih status publikasi:
   - `draft`: Menyimpan artikel di WordPress sebagai draft untuk ditinjau editor manusia.
   - `publish`: Langsung mempublikasikan artikel ke publik.
4. Klik **Publish Now**.
5. Sistem mengirim permintaan ke `/wp/v2/posts` dan memperbarui `published_at` serta `publication_url` pada kartu task.

### 6.3. Ekspor Konten Mandiri
Selain ke WordPress, konten task dapat diekspor langsung dalam format file bersih:
- Markdown (`GET /api/v1/tasks/{id}/export?format=md`)
- HTML (`GET /api/v1/tasks/{id}/export?format=html`)
- Plain Text (`GET /api/v1/tasks/{id}/export?format=txt`)

---

## 7. Panduan Penggunaan Teleprompter View

Untuk kreator video YouTube, TikTok, dan presenter webinar:
1. Buka kartu task yang memiliki naskah (`agent_output`).
2. Klik tombol **"Teleprompter View"** di sudut kanan atas Task Drawer.
3. Modal teleprompter fullscreen akan terbuka:
   - **Play / Pause:** Tekan tombol `Spasi` pada keyboard.
   - **Kecepatan Scroll:** Gunakan tombol slider kecepatan atau tombol panah `Atas` (tambah cepat) / `Bawah` (kurang cepat).
   - **Mirror Mode:** Klik tombol *Mirror* atau tekan tombol `M` untuk membalik tampilan secara horizontal bila menggunakan cermin teleprompter fisik.
   - **Ukuran Teks:** Sesuaikan ukuran font teks agar terbaca jelas dari jarak kamera.
   - **Keluar:** Tekan tombol `Esc`.

---

## 8. Prosedur Deployment & Standar Operasional Produksi (§12)

### 8.1. Lingkungan Produksi
- **Host VPS:** `DOJO_TERMAPI_VPS` (`VM-1-11-ubuntu` pada `43.173.33.11`)
- **Domain Publik:** `https://cuan.dojo.cc`
- **Reverse Proxy:** Nginx (Port 443 SSL Let's Encrypt) mem-proxy ke internal port `8080`
- **Systemd Service:** `cuan.service` di `/etc/systemd/system/cuan.service`
- **Database:** PostgreSQL 16 pada `127.0.0.1:5432` dengan database `cuan`

### 8.2. Prosedur Build & Deployment Mandiri
```bash
# 1. Build frontend React Vite
cd web && npm run build

# 2. Build backend Go binary
go build -o bin/cuan-server ./cmd/server

# 3. Sinkronisasi aset ke host produksi
scp bin/cuan-server user@host:/opt/cuan/bin/cuan-server
scp migrations/000016* user@host:/opt/cuan/migrations/
rsync -avz --delete web/dist/ user@host:/opt/cuan/web/dist/

# 4. Restart service dan verifikasi auto-migration
ssh user@host "sudo systemctl restart cuan"
```

### 8.3. Smoke Test Produksi
```bash
# 1. Health check liveness
curl -fsSL https://cuan.dojo.cc/healthz
# Expected: {"app":"cuan","status":"healthy",...} (HTTP 200)

# 2. Readiness & migration count
curl -fsSL https://cuan.dojo.cc/readyz
# Expected: {"database":"connected","migrations":"applied","migrations_count":16,...} (HTTP 200)

# 3. Server info
curl -fsSL https://cuan.dojo.cc/api/info
# Expected: HTTP 200 dengan metadata rilis Cuan
```

### 8.4. Prosedur Rollback Cepat
Bila terjadi anomali pada migrasi atau binary:
```bash
# 1. Kembalikan skema database via migration down
psql -d cuan -f /opt/cuan/migrations/000016_modular_workspaces_and_publishing.down.sql

# 2. Kembalikan backup binary versi sebelumnya
cp /opt/cuan/bin/cuan-server.bak /opt/cuan/bin/cuan-server
sudo systemctl restart cuan
```
