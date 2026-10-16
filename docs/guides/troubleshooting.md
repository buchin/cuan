# Cuan Troubleshooting & Recovery Guide

Panduan pemecahan masalah dan pemulihan darurat untuk administrator server dan pengguna Cuan. Dokumen ini merangkum skenario kegagalan umum, langkah diagnosis, dan solusi penanganannya.

---

## 1. Masalah Boot & Jaringan Container

### Skenario 1.1: Konflik Port 80 / 443 Saat Menjalankan Caddy Proxy
**Gejala:** `docker compose --profile proxy up -d` gagal dengan error `bind: address already in use` pada port 80 atau 443.
**Penyebab:** Host VPS sudah memiliki web server existing (Nginx, Traefik, Apache, atau LiteSpeed) yang mengikat port 80/443.
**Solusi:**
Gunakan **Mode B (Behind Existing Proxy)** tanpa menyalakan profil proxy Caddy internal:
```bash
# Matikan Caddy proxy internal Cuan
docker compose down

# Nyalakan hanya container Cuan API server dan PostgreSQL
docker compose up -d
```
Container Cuan akan mendengarkan port lokal `127.0.0.1:8080`. Arahkan reverse proxy existing Anda ke port tersebut.
Contoh konfigurasi Nginx:
```nginx
server {
    server_name cuan.example.com;
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

---

### Skenario 1.2: Database PostgreSQL Tidak Siap Saat Boot
**Gejala:** Endpoint `/readyz` mengembalikan respons `503 Service Unavailable` dengan pesan `database pool is not initialized` atau `connection refused`.
**Penyebab:** PostgreSQL container masih dalam proses inisialisasi awal (*cold boot*), atau volume database terkunci.
**Langkah Diagnosis:**
```bash
# Periksa status container database
docker compose ps cuan-db

# Periksa log database
docker compose logs cuan-db --tail 50
```
**Solusi:**
Cuan menyertakan retry loop otomatis (5 kali percobaan dengan interval 2 detik) pada saat startup. Jika PostgreSQL memerlukan waktu booting lebih lama, server akan memulai dalam degraded mode dan pulih begitu database siap. Jalankan migrasi manual jika diperlukan:
```bash
./scripts/migrate.sh
```

---

## 2. Masalah Autentikasi & Repositori GitHub

### Skenario 2.1: `403 Forbidden` pada `/api/v1/auth/setup`
**Gejala:** Percobaan registrasi akun owner menghasilkan error `403 Forbidden: setup already completed: single owner already exists`.
**Penyebab:** Cuan mengimplementasikan kebijakan keamanan ketat **Single-Owner**. Setelah satu akun owner didaftarkan, pendaftaran publik ditutup selamanya.
**Solusi:**
Jika Anda adalah pemilik sah yang lupa kata sandi dan ingin mengatur ulang instance di lingkungan lokal/pengujian:
```bash
# Reset database ke kondisi bersih (HATI-HATI: menghapus seluruh data):
docker compose down -v
docker compose up -d
```

---

### Skenario 2.2: GitHub PAT Kadaluarsa atau Akses Ditolak (401/403)
**Gejala:** Saat menghubungkan repositori atau saat runner hendak membuat PR, log menampilkan: `fatal: Authentication failed for ...` atau `HTTP 401 Unauthorized / Bad credentials`.
**Penyebab:** Token GitHub Personal Access Token kadaluarsa, dicabut, atau kekurangan scope `repo` / `pull_requests:write`.
**Solusi:**
1. Masuk ke GitHub Developer Settings dan buat token baru dengan scope yang sesuai.
2. Buka antarmuka Cuan pada menu **Projects**.
3. Klik tombol sinkronisasi / perbarui token pada project yang bersangkutan.
4. Token baru akan otomatis dienkripsi dengan AES-256-GCM dan digunakan untuk percobaan berikutnya.

---

## 3. Masalah Runner & Subprocess Agent

### Skenario 3.1: Runner Tidak Mengklaim Task
**Gejala:** Task berada dalam status `doing` dengan attempt `queued`, namun runner tidak kunjung memproses.
**Langkah Diagnosis:**
1. Periksa log daemon runner:
   ```bash
   # Jika dijalankan manual:
   tail -f runner.log
   ```
2. Pastikan `CUAN_RUNNER_TOKEN` pada konfigurasi runner sama persis dengan `CUAN_RUNNER_TOKEN` yang disetel pada server.
3. Pastikan runner tidak sedang sibuk mengeksekusi attempt lain (aturan batas konkurensi: 1 task aktif per runner).
4. Periksa apakah adapter agent (`agy`) terpasang pada `$PATH`:
   ```bash
   which agy
   agy --version
   ```

---

### Skenario 3.2: Runner Terputus / Server Mati Saat Task Berjalan
**Gejala:** Task tertahan dengan attempt `running`, padahal runner sudah tidak aktif (*orphaned attempt*).
**Solusi:**
Cuan menyediakan rekonsiliasi otomatis berbasis sewa (*lease timeout*):
- Setiap runner memperbarui heartbeat setiap 3 detik.
- Jika sewa habis (> 30 detik tanpa kabar), sistem menandai attempt sebagai `interrupted`.
- Anda dapat memicu rekonsiliasi sewa secara manual melalui endpoint API:
  ```bash
  curl -X POST http://localhost:8080/api/v1/runner/reconcile \
    -H "Authorization: Bearer <OWNER_TOKEN>"
  ```
- Task dapat di-retry secara aman via antarmuka UI atau API `POST /api/v1/tasks/{id}/retry`.

---

## 4. Prosedur Pemulihan Bencana & Rollback (Disaster Recovery)

> [!IMPORTANT]
> **Prinsip Utama Pemulihan Cuan**: *Rollback image saja tidak dianggap rollback database.*

Jika update versi baru mengalami kegagalan migrasi atau regresi kritis, jangan hanya me-rollback container image, karena skema database mungkin sudah berada dalam keadaan setengah termigrasi.

### Langkah 1: Buat Cadangan Darurat (Jika Belum Ada)
```bash
./scripts/backup.sh --tag emergency-pre-rollback
```

### Langkah 2: Eksekusi Rollback Lengkap (Image + Database Snapshot)
Gunakan skrip rollback teruji Cuan:
```bash
# Rollback ke versi sebelumnya dan pulihkan database dari snapshot pre-update:
./scripts/rollback.sh --version 0.1.0 --latest-pre-update --force
```

Skrip ini secara otomatis:
1. Memeriksa ketersediaan berkas snapshot cadangan database gzip beserta checksum SHA256.
2. Mematikan service Cuan untuk mencegah mutasi data konkuren.
3. Memulihkan skema dan seluruh baris data PostgreSQL dari snapshot cadangan.
4. Mengembalikan image atau binary aplikasi ke versi yang stabil (`0.1.0`).
5. Memverifikasi liveness (`/healthz`) dan readiness (`/readyz`).
6. Memastikan seluruh task, attempt, dan akun owner kembali utuh 100%.

---

### Langkah 3: Restore Manual dari Berkas Cadangan Spesifik
Jika Anda ingin memulihkan snapshot cadangan tertentu:
```bash
./scripts/restore.sh --file backups/cuan_backup_YYYYMMDD_pre-update.sql.gz --force
```

Setelah pemulihan selesai, periksa kesiapan sistem:
```bash
curl http://localhost:8080/readyz
# Expected output: {"status":"ready","database":"connected","migrations":"applied",...}
```
