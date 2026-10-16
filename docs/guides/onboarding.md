# Onboarding & Self-Hosting Guide for Cuan

Selamat datang di **Cuan** — platform manajemen proyek agentic mandiri (*self-hosted*) untuk developer dan tim kecil yang memadukan coding agent langsung ke repositori Git Anda.

Panduan ini ditujukan bagi solo developer dan tim teknis untuk memulai instalasi mandiri, mengonfigurasi akses single-owner, menghubungkan repositori GitHub pertama, dan menjalankan coding agent headless (`agy`) dalam waktu kurang dari 5 menit.

---

## 1. Persyaratan Sistem & Arsitektur

Cuan dirancang sangat ringan, efisien, dan mandiri (*zero-friction*).

### Spesifikasi Minimum Host (VPS / Bare Metal):
- **Sistem Operasi**: Linux Ubuntu 22.04 LTS / 24.04 LTS x86_64.
- **CPU & RAM**: Minimum 2 vCPU, 2 GB RAM (Rekomendasi: 4 GB RAM untuk menangani kompilasi frontend dan eksekusi subprocess agent bersamaan).
- **Disk**: 20 GB SSD/NVMe luang.
- **Akses Jaringan**: Port `80` (HTTP) dan `443` (HTTPS) terbuka untuk otomatisasi sertifikat TLS Caddy.
- **Dependensi Utama**:
  - Docker Engine 24+ & Docker Compose v2 (untuk mode container).
  - *Atau* Go 1.22+ dan Node.js 20+ (untuk mode native binary tanpa container).
  - Git 2.34+.
  - Coding Agent CLI: `agy` v1.4.0+ terpasang pada `$PATH`.

---

## 2. Pemasangan Kilat 5 Menit (Docker Compose)

### Langkah 1: Clone Repositori
```bash
git clone https://github.com/buchin/cuan.git
cd cuan
```

### Langkah 2: Otomatisasi Instalasi & Preflight
Jalankan skrip instalasi interaktif Cuan:
```bash
./scripts/install.sh --domain cuan.example.com --email admin@example.com
```

Skrip ini secara otomatis:
1. Memeriksa ketersediaan disk space, memory, dan Docker runtime.
2. Mendeteksi port `80`/`443` serta proxy existing (Nginx / Traefik / Caddy).
3. Membuat file `.env` aman dengan kunci enkripsi acak 32-karakter.
4. Menjalankan container PostgreSQL, Cuan Server, dan Caddy reverse proxy.

*Alternatif Manual via Compose:*
```bash
cp .env.example .env
# Edit .env dan atur CUAN_DOMAIN, POSTGRES_PASSWORD, dan CUAN_AUTH_SECRET
docker compose --profile proxy up -d
```

---

## 3. Inisialisasi Akses Single-Owner (First-Time Setup)

Keamanan Cuan mengusung prinsip **Single-Owner Protected**:
1. Buka browser ke alamat domain Anda (misal `https://cuan.example.com` atau `http://localhost:8080`).
2. Antarmuka Cuan akan secara otomatis menampilkan **First-Time Owner Setup Wizard**.
3. Masukkan alamat email admin dan kata sandi yang kuat (minimal 8 karakter).
4. Klik tombol **Inisialisasi & Amankan Instance**.
5. Cuan membuat kredensial akun owner dan sesi terenkripsi via cookie `HttpOnly`.

> [!IMPORTANT]
> **Takeover Lockout**: Setelah akun owner pertama dibuat, endpoint pendaftaran publik `/api/v1/auth/setup` **terkunci permanen**. Setiap percobaan pendaftaran ulang oleh pihak ketiga atau publik akan ditolak dengan respons `403 Forbidden`.

---

## 4. Menghubungkan Repositori GitHub Pertama

Untuk mengizinkan agent membuat branch dan Pull Request pada repositori Anda:
1. Buka tab **Projects** di navigasi kiri atas.
2. Klik **Connect Repository**.
3. Masukkan URL repositori GitHub Anda (contoh: `https://github.com/buchin/cuan`).
4. Masukkan **GitHub Personal Access Token (PAT)** milik Anda.
   - Hak akses token yang dibutuhkan: `repo` (atau `contents:write` dan `pull_requests:write` untuk fine-grained token).
   - Panduan membuat token: lihat [github_token_guide.md](github_token_guide.md).
5. Klik **Hubungkan Repositori**.

> [!NOTE]
> **Keamanan Kredensial**: Token GitHub Anda dienkripsi saat istirahat (*at-rest*) menggunakan **AES-256-GCM**. Nilai token mentah tidak pernah dicatat di log server maupun dikirim kembali ke browser. Tampilan UI hanya menampilkan masked preview (misal `ghp_***5544`).

---

## 5. Menjalankan Task Runner Daemon (`cuan-runner`)

Cuan memisahkan API server dari worker eksekusi agent melalui arsitektur antrean outbound:
1. Jalankan daemon runner pada host (atau VM terpisah):
   ```bash
   ./bin/cuan-runner \
     --server-url http://localhost:8080 \
     --runner-id runner-vps-01 \
     --runner-token your-configured-runner-token
   ```
2. Daemon akan mengirimkan heartbeat berkala (setiap 3 detik) dan memantau CPU, RAM, serta disk host.
3. Runner siap mengambil tugas dari task board secara bergantian (satu task aktif per runner untuk memastikan stabilitas memori).

---

## 6. Membuat Task Pertama Anda

1. Masuk ke halaman **Tasks** dan klik **Buat Task Baru**.
2. Pilih project repository yang telah terhubung.
3. Tulis judul task yang ringkas dan instruksi perbaikan yang jelas.
4. Pilih agen eksekutor (`agy`).
5. Klik **Simpan Task**.
6. Jadwalkan eksekusi (klik **Jalankan Attempt**).
7. Runner akan mengklaim task, mengisolasi repositori ke workspace branch baru (`task/<task_id>`), menjalankan instruksi agent, menjalankan targeted tests, melakukan commit Git atas nama Anda, dan membuka Pull Request di GitHub secara otomatis!
