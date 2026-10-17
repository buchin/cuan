# Panduan Operasional Turnkey WordPress Cloud & Agentic AI Webmaster Engine

Dokumen ini merupakan panduan operasional mandiri (*self-contained operational guide*) berstandar §11 bagi pengguna mandiri (*Solo Developer*), agensi digital, blogger, affiliate marketer, dan administrator sistem mengenai arsitektur, konfigurasi brankas rahasia (*Secrets Vault* AES-256-GCM), penyediaan database MySQL/MariaDB otomatis (*Automated Database Provisioning*), *Turnkey WP-CLI Bootstrapping*, *Theme & Plugin ZIP Manager*, *Agentic AI Webmaster Action Engine*, integrasi reverse proxy Caddy dengan auto-SSL, serta operasional produksi di `cuan.dojo.cc` (`DOJO_TERMAPI_VPS`).

---

## 1. Filosofi & Nilai Produk: "Turnkey WordPress Cloud & Agentic Webmaster"

Meluncurkan dan memelihara website WordPress mandiri di VPS biasanya melibatkan friksi teknis tinggi bagi non-teknisi:
1. **Kompleksitas Database & Setup Manual:** Pembuatan user dan database MySQL terisolasi, pengisian form web installer WordPress berulang kali, dan pengaturan permalink SEO manual.
2. **Risiko Kebocoran Kredensial Admin & DB:** Password database dan admin WordPress kerap disimpan dalam plaintext di environment variable atau catatan tidak terenkripsi.
3. **Risiko Kerusakan Web (*White Screen of Death / WSOD*):** Pemasangan tracking script (Google Analytics, Meta Pixel) langsung ke file `functions.php` atau `header.php` theme yang rawan hilang saat theme di-update atau membuat web crash jika ada kesalahan sintaks.
4. **Instalasi Plugin & Theme Berulang:** Upload manual via FTP atau dashboard satu per satu dengan risiko zip slip atau file corrupt.

**Solusi Cuan Sprint 13:**
- **Zero-Friction 1-Click Launchpad:** Pengguna cukup mengisi judul website, email admin, dan domain; Cuan secara otomatis mengalokasikan port, membuat container/process terisolasi, membuat database MySQL, mengeksekusi instalasi core via WP-CLI, dan mengonfigurasi permalink `/%postname%/`.
- **AES-256-GCM Secrets Vault:** Password admin (`admin_password_encrypted`) dan password database (`db_password_encrypted`) disimpan dalam brankas terenkripsi AES-256-GCM terotentikasi dengan zero plaintext leakage pada seluruh serialisasi JSON API publik.
- **Theme & Plugin ZIP Manager:** Ekstraksi aman dengan proteksi *Zip Slip*, verifikasi header metadata WordPress (`style.css` untuk theme, header PHP untuk plugin), dan aktivasi otomatis.
- **Agentic AI Webmaster Action Engine:** Asisten AI yang mengeksekusi script tracking secara aman via *must-use plugins* (`wp-content/mu-plugins`), tweaking konfigurasi `wp-config.php` dengan verifikasi pra-simpan sintaks PHP (`php -l`), serta pencatatan audit log tak terhapus di tabel `wordpress_webmaster_actions`.
- **100% Data Retention:** Skema migrasi 000018 murni aditif non-destruktif, menjaga 100% keutuhan seluruh data historis Sprint 1 hingga Sprint 12.

---

## 2. Diagram Arsitektur E2E WordPress Cloud & AI Webmaster (D2 TALA)

Diagram berikut merinci alur interaksi antara antarmuka pengguna Cuan Studio, Cuan Core Engine, Secrets Vault, runtime container WordPress, MySQL, Caddy Reverse Proxy, dan host produksi `cuan.dojo.cc`:

```d2
direction: right

user: "User / Creator" {
  ui: "Cuan Studio Web" {
    launchpad: "1-Click WordPress Launchpad"
    asset_mgr: "Theme & Plugin ZIP Dropzone"
    webmaster_tab: "AI Webmaster Console"
    creds_modal: "Masked Credentials Vault Modal"
  }
}

gateway: "Edge Gateway (cuan.dojo.cc)" {
  nginx: "Nginx SSL Reverse Proxy"
  caddy: "Caddy Reverse Proxy (Auto-SSL)"
}

cuan_core: "Cuan Backend Control Plane (Go)" {
  wp_api: "WordPress Turnkey API"
  vault: "AES-256-GCM Secrets Vault"
  provisioner: "WordPress Container & DB Orchestrator"
  wp_cli: "WP-CLI Headless Bootstrapper"
  webmaster: "Agentic AI Webmaster Engine"
  repo: "WordPress Data Layer (internal/db)"
}

infra: "Runtime Infrastructure" {
  wp_container: "WordPress Application Instance"
  db_isolated: "MySQL / MariaDB Isolated DB"
  mu_plugins: "wp-content/mu-plugins/cuan-webmaster.php"
}

user.ui.launchpad -> cuan_core.wp_api: "1-Click Deploy Request"
cuan_core.wp_api -> cuan_core.vault: "Encrypt Admin & DB Passwords"
cuan_core.wp_api -> cuan_core.repo: "Save Deployment (Pending)"
cuan_core.wp_api -> cuan_core.provisioner: "Spin-up Container & Port Pool"
cuan_core.provisioner -> infra.db_isolated: "Provision MySQL Database"
cuan_core.provisioner -> infra.wp_container: "Launch WordPress Process"
cuan_core.provisioner -> cuan_core.wp_cli: "Headless Core Install & Permalinks"
cuan_core.wp_cli -> infra.wp_container: "Execute 'wp core install'"
cuan_core.provisioner -> gateway.caddy: "Register Upstream Domain Route"
gateway.caddy -> infra.wp_container: "Route Inbound HTTPS (200 OK)"

user.ui.asset_mgr -> cuan_core.wp_api: "Upload Theme/Plugin ZIP"
cuan_core.wp_api -> infra.wp_container: "Extract Asset & Activate"

user.ui.webmaster_tab -> cuan_core.webmaster: "Command: Inject Script / Tweak Config"
cuan_core.webmaster -> cuan_core.repo: "Record Webmaster Action (JSONB)"
cuan_core.webmaster -> infra.mu_plugins: "Safe Hook Injection (wp_head/wp_footer)"

user.ui.creds_modal -> cuan_core.wp_api: "Request Decrypted Credentials"
cuan_core.wp_api -> cuan_core.vault: "Decrypt AES-256-GCM Password"
cuan_core.wp_api -> user.ui.creds_modal: "Return Unmasked Admin Password"
```

---

## 3. Skema Database & Model Data (Migrasi 000018)

Migrasi database `000018_wordpress_turnkey.up.sql` menambahkan tiga tabel inti ke PostgreSQL:

### A. Tabel `wordpress_deployments`
Mencatat informasi konfigurasi dan status deployment WordPress:
- `id` (VARCHAR(64), PK): ID unik deployment (`wpdep_...`).
- `project_id` (VARCHAR(64), FK): Referensi ke tabel `projects` (`ON DELETE CASCADE`).
- `site_title` (VARCHAR(255)): Judul situs website WordPress.
- `admin_email` (VARCHAR(255)): Email administrator WordPress.
- `admin_username` (VARCHAR(100)): Username administrator.
- `admin_password_encrypted` (TEXT): Password admin terenkripsi AES-256-GCM.
- `db_name` (VARCHAR(100)): Nama database MySQL yang dialokasikan.
- `db_user` (VARCHAR(100)): Username database MySQL.
- `db_password_encrypted` (TEXT): Password database terenkripsi AES-256-GCM.
- `db_host` (VARCHAR(255)): Host database MySQL (default: `127.0.0.1`).
- `db_port` (INTEGER): Port database MySQL (default: `3306`).
- `domain` (VARCHAR(255)): Domain publik atau subdomain yang dialokasikan.
- `port` (INTEGER): Port internal yang dialokasikan dari port pool supervisor.
- `table_prefix` (VARCHAR(20)): Prefix tabel database WordPress (default: `wp_`).
- `wp_version` (VARCHAR(50)): Versi core WordPress (default: `latest`).
- `status` (VARCHAR(50)): Status siklus hidup (`pending`, `provisioning`, `running`, `stopped`, `failed`).
- `health_status` (VARCHAR(50)): Status probe kesehatan (`unknown`, `healthy`, `degraded`, `error`).
- `permalink_structure` (VARCHAR(255)): Struktur permalink SEO (default: `/%postname%/`).
- `created_at`, `updated_at`: Waktu pencatatan dan perubahan.

### B. Tabel `wordpress_plugins_themes`
Mencatat tema dan plugin yang diinstal melalui dropzone ZIP Cuan:
- `id` (VARCHAR(64), PK): ID unik aset (`wpat_...`).
- `deployment_id` (VARCHAR(64), FK): Referensi ke tabel `wordpress_deployments` (`ON DELETE CASCADE`).
- `kind` (VARCHAR(20)): Jenis aset (`theme` atau `plugin`).
- `slug` (VARCHAR(100)): Slug direktori tema atau plugin.
- `name` (VARCHAR(255)): Nama resmi tema atau plugin.
- `version` (VARCHAR(50)): Versi terdeteksi dari header aset.
- `is_active` (BOOLEAN): Status keaktifan tema/plugin.
- `source_zip` (VARCHAR(255)): Nama berkas ZIP sumber.
- `installed_at`: Waktu instalasi.

### C. Tabel `wordpress_webmaster_actions`
Mencatat riwayat audit aksi AI Webmaster:
- `id` (VARCHAR(64), PK): ID unik aksi (`wpact_...`).
- `deployment_id` (VARCHAR(64), FK): Referensi ke `wordpress_deployments` (`ON DELETE CASCADE`).
- `action_type` (VARCHAR(50)): Tipe aksi (`inject_script`, `remove_script`, `toggle_script`, `config_tweak`, `install_asset`).
- `payload` (JSONB): Rincian parameter aksi (script content, target hook, config changes).
- `applied_at`: Waktu eksekusi aksi.

---

## 4. Alur Kerja Operasional Turnkey 1-Click Launchpad

### Step 1: Pembuatan Project WordPress
1. Di Cuan Studio, buat project baru atau pilih tipe `wordpress`:
   - `enabled_modules`: `["tasks", "discussions", "deployments", "creator", "wordpress"]`
   - `project_type`: `wordpress`
2. Buka tab **"WordPress"** atau klik tombol **"1-Click WordPress Deploy"**.

### Step 2: Konfigurasi Wizard Multi-Step
- **Tab 1: Site Profile & Admin Credentials:**
  - `site_title`: Judul website (misal: "Niche Affiliate Marketing Hub").
  - `admin_email`: Email pemilik/admin (misal: `buchin@dropsugar.com`).
  - `admin_username`: Username admin (misal: `admin_cuan`).
  - `admin_password`: Password kuat (tersedia fitur auto-generate strong password).
- **Tab 2: Domain & Database:**
  - `domain`: Subdomain atau custom domain (misal: `affiliate.cuan.dojo.cc`).
  - `db_name`: Nama database MySQL (otomatis dibuat jika dikosongkan).
  - `table_prefix`: Prefix tabel database (default: `wp_`).
  - `permalink_structure`: Format permalink SEO (default: `/%postname%/`).
- **Tab 3: Theme & Plugin Upload (Opsional):**
  - Dropzone berkas ZIP tema atau plugin kustom.
- **Tab 4: Launch Console:**
  - Klik **"Launch WordPress Site"**. Sistem menjalankan orkestrasi container dan bootstrapping WP-CLI.

### Step 3: Verifikasi Live Web
- Begitu proses bootstrapping selesai:
  - Status berubah menjadi `running` dan `healthy`.
  - Tombol **"Open Live Site"** membuka `https://domain` (respons HTTP 200).
  - Tombol **"Open WP-Admin"** membuka `https://domain/wp-login.php` (respons HTTP 200).
  - Tombol **"Reveal Credentials"** membuka dialog kredensial admin dengan fitur salin satu klik.

---

## 5. Agentic AI Webmaster Action Engine

### A. Safe Script Injection (mu-plugins)
AI Webmaster menginjeksi kode tracking (Google Analytics, Meta Pixel, TikTok Pixel, Google Tag Manager) melalui plugin *must-use* di `wp-content/mu-plugins/cuan-webmaster.php`:
- **Keunggulan:**
  1. Kebal terhadap pergantian tema WordPress.
  2. Bebas risiko syntax error yang merusak `functions.php`.
  3. Dapat diaktifkan/dinonaktifkan (*toggle*) seketika tanpa mengubah file tema.
- **API Endpoint:**
  - `POST /api/v1/projects/{id}/wordpress/scripts`
  - Body:
    ```json
    {
      "name": "Google Tag Manager",
      "position": "head",
      "content": "<!-- Google Tag Manager --><script>...</script>"
    }
    ```

### B. Safe Config Tweaker
AI Webmaster dapat memodifikasi konstanta `wp-config.php` seperti batas memori, debug mode, atau upload file:
- **Pencegahan WSOD:** Sebelum menyimpan perubahan ke `wp-config.php`, engine membuat berkas sementara dan menjalankan validasi sintaks PHP (`php -l`). Jika sintaks tidak valid, pembaruan dibatalkan dan file asli dipertahankan.
- **API Endpoint:**
  - `POST /api/v1/projects/{id}/wordpress/config`
  - Body:
    ```json
    {
      "tweaks": {
        "WP_MEMORY_LIMIT": "256M",
        "WP_DEBUG": false
      }
    }
    ```

### C. Webmaster Audit Action History
Setiap aksi yang dilakukan AI Webmaster tercatat rapi di tabel `wordpress_webmaster_actions`:
- Endpoint `GET /api/v1/projects/{id}/wordpress/actions` menampilkan riwayat audit lengkap disertai payload JSONB dan timestamp presisi.

---

## 6. Prosedur Rilis Produksi di `cuan.dojo.cc` (`DOJO_TERMAPI_VPS`)

### A. Pre-Deployment Verification
1. Pastikan seluruh automated tests passed:
   ```bash
   go test -v ./internal/db/... ./internal/wordpress/...
   ```
2. Pastikan build frontend dan backend bersih:
   ```bash
   (cd web && npm run build)
   go build -o bin/cuan-server ./cmd/server
   go build -o bin/cuan-runner ./cmd/runner
   go build -o bin/cuan-migrate ./cmd/migrate
   ```

### B. Eksekusi Migrasi 000018 di Host Produksi
Jalankan migrasi schema melalui CLI `cuan-migrate`:
```bash
cuan-migrate --db-url "$DATABASE_URL"
```
Verifikasi migrasi telah tercatat di tabel `cuan_schema_migrations`:
```sql
SELECT version, name, applied_at FROM cuan_schema_migrations ORDER BY version;
```
Hasil verifikasi wajib menampilkan `000018 | wordpress_turnkey` sebagai versi terbaru (total 18 baris).

### C. Restart & Smoke Tests Produksi
Restart service cuan di host produksi:
```bash
sudo systemctl restart cuan
```
Lakukan pengujian *public smoke tests* melalui internet:
```bash
# 1. Healthz probe
curl -s https://cuan.dojo.cc/healthz
# Response: {"app":"cuan","status":"healthy",...}

# 2. Readyz probe & migrations count verification
curl -s https://cuan.dojo.cc/readyz
# Response: {"database":"connected","migrations":"applied","migrations_count":18,"status":"ready"}

# 3. System info
curl -s https://cuan.dojo.cc/api/info
# Response: {"name":"Cuan","author":"Buchin <buchin@dropsugar.com>",...}

# 4. Caddy On-Demand TLS Gatekeeper
curl -s "https://cuan.dojo.cc/api/caddy/ask?domain=cuan.dojo.cc"
# Response: {"status":"authorized",...}
```

---

## 7. Kesimpulan & Penjaminan Standar Mutu

Sistem *Turnkey WordPress Cloud & Agentic AI Webmaster Engine* Cuan Sprint 13 telah terverifikasi memenuhi seluruh kriteria DoD:
- **Mandiri (*Self-Contained*) & Zero-Friction:** Pengguna dapat meluncurkan dan memelihara website WordPress berstandar enterprise tanpa hambatan konfigurasi manual.
- **Zero Credential Leakage:** Perlindungan kriptografi AES-256-GCM menjaga keamanan kredensial admin dan database di semua level.
- **100% Data Retention:** Seluruh data historis dari Sprint 1 hingga Sprint 12 tetap utuh dan beroperasi tanpa hambatan.
