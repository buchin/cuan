# Panduan Operasional Automated Environment Provisioner & 1-Click Script Deployer

Dokumen ini merupakan panduan operasional mandiri bagi pengembang mandiri (*Solo Developer*), pembeli script online, agensi digital, dan administrator sistem mengenai arsitektur, konfigurasi brankas rahasia (*Secrets Vault*), penyediaan database terisolasi (*Automated Database Provisioning*), impor skema SQL otomatis, *Build Runner*, *Process Supervisor*, alokasi port dinamis, integrasi reverse proxy Caddy, serta operasional produksi di `cuan.dojo.cc`.

---

## 1. Filosofi & Nilai Produk: "Turnkey 1-Click Script Deployer"

Pengguna non-teknis, agensi digital, dan pembeli source code di marketplace (CodeCanyon, Envato, GitHub) sering mengalami hambatan instalasi (*setup friction*):
1. **Kegagalan Setup Database & Impor SQL:** Kebingungan membuat database dan user terisolasi di phpMyAdmin atau terminal MySQL/PostgreSQL, serta kegagalan sintaks saat mengimpor dump skema `.sql`.
2. **Kompleksitas Variabel Lingkungan (.env):** Kesulitan mengisi variabel wajib seperti `APP_KEY`, `JWT_SECRET`, dan kredensial database yang rawan bocor jika disimpan dalam plaintext.
3. **Kendala Runtime & Eksekusi Build:** Tidak tersedianya environment `composer`, `npm`, atau `pip` pada mesin lokal pengguna, serta kebingungan menjalankan perintah build.
4. **Konflik Port & Konfigurasi Web Server:** Kebingungan memetakan port aplikasi ke reverse proxy Nginx/Caddy serta mengurus sertifikat SSL/HTTPS publik.

**Solusi Cuan Sprint 12:**
- **Automated Provisioning:** Pembuatan database terisolasi (SQLite, PostgreSQL, MySQL) dan impor skema SQL otomatis hanya dengan satu klik.
- **AES-256-GCM Secrets Vault:** Penghasil token rahasia acak berstandar kriptografi tinggi dengan enkripsi menyeluruh di database (zero plaintext leakage).
- **Runtime Build Runner:** Eksekusi instalasi dependensi terisolasi dengan proteksi timeout dan feedback log transparan secara real-time.
- **Process Supervisor & Dynamic Port Allocator:** Pemantauan status aplikasi otomatis dengan alokasi port bebas konflik dan health check poller hingga respons HTTP 200.
- **Caddy Reverse Proxy & On-Demand TLS:** Pendaftaran otomatis rute upstream domain dengan penerbitan sertifikat SSL otomatis.
- **100% Data Retention:** Skema migrasi 000017 murni aditif non-destruktif, menjaga 100% data historis dari Sprint 1 hingga Sprint 11.

---

## 2. Diagram Arsitektur E2E Script Deployer & Provisioner (D2 TALA)

Diagram berikut menjelaskan alur kerja dari onboarding script, konfigurasi rahasia, provisioning database, eksekusi build runner, peluncuran supervisor, hingga perutean traffic publik:

```d2
direction: right

user: "User / Solopreneur" {
  ui: "Cuan Studio Web" {
    kanban: "Deployer Tab / Kanban"
    wizard: "1-Click Deployer Modal"
    console: "Live Build Terminal Console"
  }
}

gateway: "Edge Gateway (cuan.dojo.cc)" {
  nginx: "Nginx SSL Reverse Proxy"
  caddy: "Caddy Reverse Proxy & Auto-SSL"
}

cuan_core: "Cuan Backend Control Plane (Go)" {
  deployer_api: "Script Deployer API & WS"
  vault: "AES-256-GCM Secrets Vault"
  provisioner: "Database Provisioner (MySQL/PG/SQLite)"
  runner: "Build Runner Pipeline"
  port_mgr: "Dynamic Port Allocator"
  supervisor: "Process Supervisor & Health Poller"
}

infra: "Target Runtime Infrastructure" {
  db_isolated: "Provisioned Database (Schema Imported)"
  app_proc: "Running Script Application Process"
}

user.ui.kanban -> user.ui.wizard: "Click 1-Click Deploy Script"
user.ui.wizard -> cuan_core.vault: "Auto-generate Secrets & Encrypt Env"
user.ui.wizard -> cuan_core.provisioner: "Provision DB & Import .sql Schema"
cuan_core.provisioner -> infra.db_isolated: "Execute DDL Schema Migration"
user.ui.wizard -> cuan_core.runner: "Trigger Script Build Pipeline"
cuan_core.runner -> user.ui.console: "Stream Build Logs"
cuan_core.runner -> cuan_core.port_mgr: "Acquire Dynamic Port"
cuan_core.port_mgr -> cuan_core.supervisor: "Launch Service Process"
cuan_core.supervisor -> infra.app_proc: "Start Background Daemon"
cuan_core.supervisor -> gateway.caddy: "Register Proxy Route & Domain"
gateway.caddy -> infra.app_proc: "Reverse Proxy HTTP 200 Traffic"
cuan_core.supervisor -> user.ui.console: "Health Check Ready (HTTP 200)"
```

---

## 3. Skema Database & Model Data (Migrasi 000017)

Migrasi database `000017_script_deployer_and_provisioner.up.sql` menambahkan dua tabel inti ke PostgreSQL:

### A. Tabel `script_deployments`
Mencatat riwayat dan status deployment aplikasi script yang berjalan.
- `id` (VARCHAR(64), PK): ID unik deployment (`sdep_...`).
- `project_id` (VARCHAR(64), FK): Referensi ke tabel `projects` (`ON DELETE CASCADE`).
- `server_id` (VARCHAR(64), FK): Server target BYOS (`ON DELETE SET NULL`), bernilai NULL untuk local sandbox.
- `port` (INTEGER): Port TCP internal tempat aplikasi mendengarkan koneksi.
- `domain` (VARCHAR(255)): Domain publik atau subdomain yang dialokasikan.
- `env_encrypted` (TEXT): Variabel lingkungan `.env` terenkripsi menggunakan AES-256-GCM.
- `build_status` (VARCHAR(32)): Status build (`pending`, `building`, `success`, `failed`).
- `health` (VARCHAR(32)): Status kesehatan proses (`unknown`, `healthy`, `degraded`, `stopped`).
- `created_at`, `updated_at`: Waktu pembuatan dan pembaruan timestamp.

### B. Tabel `provisioned_databases`
Mencatat instance database terisolasi yang dialokasikan untuk project.
- `id` (VARCHAR(64), PK): ID unik database (`pdb_...`).
- `project_id` (VARCHAR(64), FK): Referensi ke tabel `projects` (`ON DELETE CASCADE`).
- `server_id` (VARCHAR(64), FK): Server database target.
- `engine` (VARCHAR(32)): Mesin database (`sqlite`, `mysql`, `postgres`).
- `db_name` (VARCHAR(128)): Nama database terisolasi yang dibuat.
- `db_user` (VARCHAR(128)): Username pengguna database.
- `db_password_encrypted` (TEXT): Password pengguna database terenkripsi AES-256-GCM.
- `status` (VARCHAR(32)): Status provisioning (`provisioning`, `ready`, `failed`).
- `schema_imported` (BOOLEAN): Penanda apakah berkas skema `.sql` telah berhasil diimpor.
- `created_at`, `updated_at`: Waktu pencatatan.

---

## 4. Brankas Kredensial & Secrets Vault (AES-256-GCM)

Modul brankas rahasia (`internal/crypto`) mengamankan seluruh kredensial database dan variabel lingkungan menggunakan enkripsi autentikasi simetris AES-256-GCM:

1. **Master Key Derivation:** Kunci enkripsi diturunkan dari variabel `AUTH_SECRET` atau master key 32-byte unik.
2. **Non-ce Randomization:** Setiap enkripsi menghasilkan 12-byte initialization vector (IV/nonce) acak yang digabungkan ke ciphertext berformat hex atau Base64.
3. **Zero Plaintext Leakage:** Endpoint REST API tidak pernah mengembalikan nilai `db_password` atau `env_encrypted` dalam plaintext. Nilai sensitif dimasking (`********`) atau disembunyikan.
4. **Auto-Generate Application Secrets:**
   - Kunci aplikasi Laravel: `base64:<32-byte-random>`
   - Token JWT / Session Secret: 64-karakter hex string
   - Database Password: 24-karakter string alfanumerik berkekuatan tinggi.

---

## 5. Automated Database Provisioner & Schema Importer

Modul `internal/deployer/provisioner.go` menangani siklus hidup penyediaan database:

### Mesin Database yang Didukung:
1. **SQLite (Zero Setup / Default):**
   - Menghasilkan file database terisolasi di direktori project (`data/projects/{id}/database.sqlite`).
   - Tidak memerlukan port eksternal atau service tambahan.
2. **PostgreSQL:**
   - Mengalokasikan database unik (`cuan_proj_{id}`) dan user terisolasi dengan privilege terbatas.
3. **MySQL:**
   - Mendukung provisioning database via root connection socket/TCP dan grant privileges ke user spesifik.

### Schema Importer (.sql):
- Mesin memindai keberadaan file `.sql` dalam paket zip yang di-onboard.
- SQL diuraikan per-statement untuk menghindari parsing error pada dialek khusus.
- Eksekusi berlangsung di dalam transaksi database. Jika terjadi kegagalan sintaks, perubahan di-rollback secara otomatis dan pesan kesalahan informatif dicatat ke log build.

---

## 6. Build Runner & Process Supervisor Engine

### Build Runner Pipeline (`internal/deployer/script_runner.go`):
1. **Deteksi Runtime:** Membaca tipe stack (`laravel`, `node`, `python`, `html`).
2. **Pemasangan Dependensi:**
   - Laravel/PHP: `composer install --no-dev --prefer-dist --no-interaction --optimize-autoloader`
   - Node.js: `npm install --production`
   - Python: `pip install -r requirements.txt`
3. **Isolasi Lingkungan:** Perintah dijalankan di direktori project dengan batasan timeout yang dapat dikonfigurasi (default: 300 detik).
4. **Log Streaming:** Output stdout dan stderr disalurkan secara simultan ke channel buffer memori dan disiarkan via WebSocket atau Server-Sent Events (SSE).

### Process Supervisor (`internal/deployer/supervisor.go`):
1. **Dynamic Port Pool:** Mengalokasikan port TCP acak yang tersedia (rentang 9000-9999) dan memverifikasi port belum terikat sebelum meluncurkan proses.
2. **Daemon Lifecycle Management:**
   - Memulai daemon latar belakang dengan lingkungan `.env` terdekripsi.
   - Menyimpan Process ID (PID) dan memonitor status exit code (crash detection & auto-restart).
3. **Health Check Poller:** Melakukan query HTTP periodik ke endpoint `/healthz` atau `/` hingga server merespons HTTP 200 OK.
4. **Graceful Shutdown:** Mengirimkan sinyal `SIGTERM` ke grup proses anak sebelum eskalasi ke `SIGKILL`.

---

## 7. Caddy Reverse Proxy & On-Demand TLS Gatekeeper

Untuk menghubungkan port internal aplikasi ke publik dengan sertifikat SSL otomatis:

1. **Rute Upstream Otomatis:** Supervisor mendaftarkan rute reverse proxy upstream ke Caddy (`domain` -> `127.0.0.1:{allocated_port}`).
2. **On-Demand TLS Gatekeeper (`/api/caddy/ask`):**
   - Caddy memanggil `GET https://cuan.dojo.cc/api/caddy/ask?domain={domain}` sebelum meminta sertifikat dari Let's Encrypt / ZeroSSL.
   - Endpoint memverifikasi apakah domain tersebut terdaftar di tabel `script_deployments` atau `custom_domains`.
   - Domain yang valid merespons `200 OK` (`{"status":"authorized"}`).
   - Upaya eksploitasi domain asing langsung ditolak dengan `403 Forbidden` (`{"status":"forbidden"}`) untuk mencegah kehabisan kuota rate limit ACME.

---

## 8. Panduan Langkah Demi Langkah Pengguna (1-Click Deployer Wizard)

1. **Buka Cuan Studio:** Akses project hasil ekstrak ZIP di `https://cuan.dojo.cc/studio`.
2. **Klik Tombol "1-Click Deploy":** Di tab Deployer atau Kanban board, klik tombol **"1-Click Deploy Script"**.
3. **Step 1 - Environment & Secrets:**
   - Form visual menampilkan variabel `.env` yang dideteksi.
   - Klik **"Auto-generate Secrets"** untuk mengisi password database dan kunci enkripsi secara otomatis.
4. **Step 2 - Database Provisioning:**
   - Pilih engine (SQLite/MySQL/PostgreSQL).
   - Centang opsi **"Import SQL Schema"** jika terdapat file dump `.sql`.
   - Klik **"Provision Database"**.
5. **Step 3 - Server & Domain Allocation:**
   - Pilih target server (Local Docker Sandbox atau BYOS VPS).
   - Masukkan domain/subdomain yang diinginkan.
6. **Step 4 - Live Build Console:**
   - Konsol terminal menampilkan log instalasi dependensi secara real-time.
   - Setelah status "Health Check Ready (HTTP 200)" muncul, tombol **"Open Live App"** aktif.
   - Klik **"Open Live App"** untuk mengakses aplikasi web yang telah aktif di internet.

---

## 9. Verifikasi Produksi di cuan.dojo.cc

Pada host produksi `DOJO_TERMAPI_VPS`, verifikasi operasional dapat dijalankan melalui perintah curl:

```bash
# 1. Health check liveness
curl -s https://cuan.dojo.cc/healthz
# Expected: {"app":"cuan","status":"healthy",...} (HTTP 200)

# 2. Readiness check & status migrasi database
curl -s https://cuan.dojo.cc/readyz
# Expected: {"database":"connected","migrations":"applied","migrations_count":17,"status":"ready"} (HTTP 200)

# 3. Info metadata aplikasi
curl -s https://cuan.dojo.cc/api/info
# Expected: {"name":"Cuan","author":"Buchin <buchin@dropsugar.com>",...} (HTTP 200)

# 4. Gatekeeper on-demand TLS
curl -s "https://cuan.dojo.cc/api/caddy/ask?domain=cuan.dojo.cc"
# Expected: {"status":"authorized",...} (HTTP 200)

# 5. Proteksi domain ilegal
curl -s "https://cuan.dojo.cc/api/caddy/ask?domain=malicious-domain.evil.com"
# Expected: {"status":"forbidden",...} (HTTP 403)
```

---

## 10. Troubleshooting & FAQ

| Gejala Masalah | Penyebab Umum | Tindakan Perbaikan |
| :--- | :--- | :--- |
| **Build Gagal: "composer not found"** | Binari runtime belum terpasang di host | Pasang `composer`, `npm`, atau `python3` di server worker target atau gunakan Docker sandbox |
| **Port Collision Error** | Port telah dipakai proses sistem lain | Dynamic Port Allocator akan mencoba port berikutnya; periksa dengan `ss -tulpn` |
| **SQL Import Syntax Error** | Dialek SQL berbeda dengan engine target | Pastikan memilih engine yang sesuai (misal: MySQL dump tidak dapat diimpor langsung ke Postgres) |
| **Reverse Proxy 502 Bad Gateway** | Aplikasi script belum selesai boot | Periksa tab **"Live Logs"** di Studio untuk memverifikasi aplikasi merespons HTTP pada port yang dialokasikan |
| **SSL Not Issued** | DNS A-Record domain belum mengarah ke VPS | Pastikan DNS A-Record mengarah ke IP publik VPS sebelum mendaftarkan custom domain |
