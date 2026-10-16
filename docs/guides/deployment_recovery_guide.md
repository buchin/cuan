# Panduan Deployment, Staging, Backup, dan Pemulihan Bencana (Disaster Recovery) Cuan

Panduan operasional dan arsitektur resmi Cuan untuk instalasi mandiri (*self-hosted*), pemisahan lingkungan Staging dan Stable, pembaruan versi aman (*safe version update*), prosedur pencadangan (*backup*), drill pemulihan (*restore*), mitigasi kegagalan update (*rollback drill*), serta *playbook* pemulihan darurat via CLI/GitHub jika Web UI tidak dapat diakses (Sprint 1, Task T7).

---

## 1. Arsitektur Deployment & Isolasi Lingkungan

Cuan mengadopsi arsitektur *zero-collision isolation* antara instance **Stable** (Production) dan **Staging** di host yang sama maupun terpisah.

```d2
direction: right

client: "Web Browser / API Client\n(Public Internet)"

vps: "VPS Host (Ubuntu Linux / Docker)" {
  proxy: "Reverse Proxy (Caddy / Nginx)\nHTTPS Ports 80 & 443"

  stable_env: "Stable (Production Environment)" {
    app_stable: "Cuan App (Stable)\nImage: cuan:0.1.0\nPort 8080 (cuan-network)"
    db_stable: "PostgreSQL Stable\nPort 5432 (Internal Only)\nVolume: cuan_postgres_data"
  }

  staging_env: "Staging (Preview Environment)" {
    app_staging: "Cuan App (Staging)\nImage: cuan:staging\nPort 8081 (cuan-staging-network)"
    db_staging: "PostgreSQL Staging\nPort 5432 (Internal Only)\nVolume: cuan_staging_postgres_data"
  }
}

client -> proxy: "HTTPS (cuan.domain.com / staging.domain.com)"
proxy -> app_stable: "Reverse Proxy :8080"
proxy -> app_staging: "Reverse Proxy :8081"
app_stable -> db_stable: "Persistent SQL (cuan_postgres_data)"
app_staging -> db_staging: "Isolated SQL (cuan_staging_postgres_data)"
```

### Matriks Isolasi Stable vs Staging

| Komponen | Stable (Production) | Staging (Preview) | Status Isolasi |
| :--- | :--- | :--- | :--- |
| **Compose File** | `docker-compose.yml` | `docker-compose.staging.yml` | Terpisah mandiri |
| **Project Name** | `cuan` | `cuan-staging` | Namespace container terpisah |
| **Application Port** | `8080` | `8081` | Tanpa konflik port host |
| **Database Container** | `cuan-db-1` | `cuan-staging-db-1` | Container terisolasi |
| **Database Name** | `cuan` | `cuan_staging` | Database mandiri |
| **Docker Volume** | `cuan_postgres_data` | `cuan_staging_postgres_data` | Data volume fisik terisolasi |
| **Bridge Network** | `cuan-network` | `cuan-staging-network` | Subnet Docker independen |
| **Default Hostname** | `cuan.example.com` | `staging.cuan.example.com` | Subdomain terdedikasi |

---

## 2. Prosedur Instalasi Bersih (*Fresh Install*)

### Langkah 1: Kloning Repository dan Jalankan Installer
```bash
git clone https://github.com/buchin/cuan.git
cd cuan

# Jalankan installer dengan preflight checks otomatis:
./scripts/install.sh --domain cuan.example.com --email admin@example.com
```

### Langkah 2: Preflight Checks Otomatis
Skrip `install.sh` melakukan pengecekan pra-syarat secara ketat:
1. **OS & Arsitektur**: Memeriksa Linux x86_64 / arm64.
2. **Kapasitas Disk**: Memastikan sisa ruang disk $\ge 2\,\text{GB}$.
3. **Kapasitas RAM**: Memeriksa memori bebas $\ge 1\,\text{GB}$ (rekomendasi $\ge 2\,\text{GB}$).
4. **Port Availability & Smart Detection**:
   - Jika port 80 & 443 bebas: Memilih **Mode A (Standalone Caddy)** dengan sertifikat TLS otomatis.
   - Jika port 80 & 443 sudah digunakan Nginx/Apache host: Memilih **Mode B (Existing Reverse Proxy)** agar tidak menabrak port host.
5. **Konfigurasi Lingkungan (`.env`)**:
   - Jika `.env` belum ada: Membuatnya dari `.env.example` dan meng-generate password database serta auth secret acak 32+ karakter secara otomatis.
   - Jika `.env` sudah ada: **Tidak menimpa** konfigurasi yang ada.

### Langkah 3: Verifikasi Health Check
```bash
# Liveness probe (HTTP 200 OK)
curl -s http://127.0.0.1:8080/healthz

# Readiness probe (memastikan koneksi DB & migrasi terverifikasi)
curl -s http://127.0.0.1:8080/readyz
```

---

## 3. Lingkungan Staging Terpisah

Untuk memasang atau menjalankan instance staging tanpa mengganggu instance stable:

```bash
# Jalankan installer dengan flag --staging
./scripts/install.sh --staging --domain staging.cuan.example.com

# Atau jalankan via Compose staging langsung:
docker compose -f docker-compose.staging.yml up -d
```

Pekerjaan di staging menggunakan volume `cuan_staging_postgres_data`, database `cuan_staging`, dan port 8081, sehingga perubahan kode atau data eksperimen di staging tidak akan berdampak pada instance Stable.

---

## 4. Pencadangan (*Backup*) dan Drill Pemulihan (*Restore*)

### Membuat Backup Database
Skrip `scripts/backup.sh` membuat cadangan SQL yang dikompresi gzip, menghitung *checksum* SHA256 untuk verifikasi integritas, dan menyimpan metadata JSON:

```bash
# Backup otomatis (database aktif)
./scripts/backup.sh

# Backup sebelum update (tagged)
./scripts/backup.sh --tag pre-update

# Backup database spesifik via URL
./scripts/backup.sh --db-url "postgres://cuan:secret@localhost:5432/cuan?sslmode=disable"
```

Hasil backup tersimpan di direktori `backups/`:
- `backups/cuan_backup_YYYYMMDD_HHMMSS_tag.sql.gz` (Arsip SQL terkompresi)
- `backups/cuan_backup_YYYYMMDD_HHMMSS_tag.sql.gz.sha256` (Hash integritas SHA256)
- `backups/cuan_backup_YYYYMMDD_HHMMSS_tag.meta.json` (Metadata tabel, ukuran, dan status)

### Memulihkan Database dari Backup (*Restore Drill*)
Skrip `scripts/restore.sh` menguji integritas checksum SHA256 dan stream gzip sebelum mengeksekusi restore:

```bash
# Restore file backup terbaru secara otomatis
./scripts/restore.sh --latest --force

# Restore file backup spesifik
./scripts/restore.sh --file backups/cuan_backup_20261016_083331Z_test-drill.sql.gz --force
```

---

## 5. Alur Pembaruan Versi Aman (*Safe Version Update*) & Rollback

### Alur Update Aman
Skrip `scripts/update.sh` menerapkan SOP *safe upgrade*:
1. **Pre-check**: Memeriksa `/healthz` dan `/readyz` sebelum update dimulai.
2. **Auto-Backup**: Membuat snapshot database bertag `pre-update` secara otomatis.
3. **Pembaruan Konfigurasi & Image**: Menyetel versi baru di `.env` (`CUAN_APP_VERSION` dan `CUAN_IMAGE`).
4. **Migrasi Skema**: Menjalankan migrasi database (`cuan-migrate`).
5. **Post-check**: Memvalidasi kesiapan `/readyz` dan memastikan seluruh data (task, attempt, riwayat, sesi owner) utuh tanpa data loss.

```bash
# Menjalankan update ke versi 0.2.0
./scripts/update.sh --version 0.2.0
```

### Mitigasi Kegagalan & Rollback Drill (*Prinsip S1*)
> [!IMPORTANT]
> **Rollback image saja tidak dianggap rollback database.**
> Jika update gagal di tahap migrasi atau aplikasi mengalami *crash loop*, skrip `rollback.sh` akan:
> 1. Memulihkan skema dan data database dari snapshot `pre-update` yang bersih.
> 2. Mengembalikan versi image/aplikasi ke versi sebelumnya di `.env`.
> 3. Merestart container aplikasi ke versi stabil.
> 4. Menguji `/readyz` hingga sistem kembali melayani permintaan 100%.

```bash
# Rollback manual jika terjadi insiden:
./scripts/rollback.sh --version 0.1.0 --latest-pre-update --force

# Simulasi drill kegagalan update dan auto-rollback:
./scripts/update.sh --version 0.2.1-broken --simulate-failure
```

---

## 6. Emergency CLI Recovery Playbook (Jika Web UI Tidak Dapat Diakses)

Jika Web UI Cuan tidak dapat diakses (misalnya karena masalah reverse proxy, DNS, atau build frontend):

### 1. Diagnosa Status Container & Service
```bash
# Periksa status container
docker compose ps

# Periksa log aplikasi backend
docker compose logs --tail=100 -f app

# Periksa log reverse proxy
docker compose logs --tail=100 -f caddy
```

### 2. Diagnosis Endpoint Internal via Host
```bash
# Cek apakah backend merespons di port internal 8080
curl -i http://127.0.0.1:8080/healthz
curl -i http://127.0.0.1:8080/readyz

# Cek CLI healthcheck bawaan binary
bin/cuan-server --healthcheck
bin/cuan-server --readycheck
```

### 3. Akses Darurat Database via CLI
```bash
# Buka psql di dalam container PostgreSQL
docker compose exec db psql -U cuan -d cuan

# Query darurat status task dan attempt:
SELECT id, title, status, priority FROM tasks ORDER BY created_at DESC LIMIT 10;
SELECT id, task_id, status, exit_code FROM task_attempts ORDER BY created_at DESC LIMIT 5;

# Periksa status migrasi skema yang sudah diterapkan:
SELECT version, name, applied_at FROM cuan_schema_migrations ORDER BY version ASC;
```

### 4. Menjalankan Migrasi Manual via CLI
```bash
# Cek dry-run pernyataan SQL migrasi
./scripts/migrate.sh --dry-run

# Rollback 1 tingkat migrasi jika migrasi terakhir bermasalah
./scripts/migrate.sh --down
```

### 5. Pemulihan Penuh via GitHub (Disaster Recovery di VPS Baru)
Jika VPS fisik mengalami kerusakan total (*hardware failure*):
1. Siapkan VPS baru (Ubuntu 24.04).
2. Kloning repository Cuan: `git clone https://github.com/buchin/cuan.git && cd cuan`.
3. Salin file backup terbaru dari penyimpanan eksternal (R2/S3/SCP) ke folder `backups/`.
4. Jalankan installer: `./scripts/install.sh --domain cuan.domain.com`.
5. Restore database: `./scripts/restore.sh --latest --force`.
6. Akses kembali Cuan di `https://cuan.domain.com` — seluruh task, project, owner login, dan riwayat pekerjaan kembali utuh 100%.
