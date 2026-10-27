# Panduan Operasional Universal App Deployer & Studio Runtime Cuan

Dokumen ini merupakan panduan operasional komprehensif bagi pemilik instance (*Single-Owner*), pengembang mandiri (*Solo Developer*), dan Indie Hacker mengenai tata cara arsitektur, deployment aplikasi, sandboxing proses terisolasi, manajemen routing dinamis subdomain, konfigurasi mode aplikasi, serta prosedur pemecahan masalah (*troubleshooting*) pada Cuan Deployer.

---

## 1. Filosofi & Gambaran Umum

Cuan Universal App Deployer menghadirkan runtime hosting mandiri yang *self-contained*, *zero-friction*, dan terisolasi secara aman langsung di satu server (*co-located*).

### Prinsip Utama Deployer
1. **Zero-Friction Intake:** Pengembang dapat mendeploy aplikasi web hanya dalam hitungan detik melalui upload berkas ZIP (drag-and-drop) atau memasukkan tautan clone Git tanpa konfigurasi infrastruktur manual.
2. **Auto Stack Detection:** Sistem secara cerdas mendeteksi runtime stack proyek: **Static HTML**, **PHP / WordPress**, atau **Node.js**.
3. **Strict Sandboxing & Anti-ZipSlip Defense:** Arsip ZIP yang diunggah divalidasi dengan sanitasi traversal ketat. Seluruh manipulasi direktori induk (`../`), path absolut (`/`), maupun *escaping symlinks* ditolak secara keras demi menjamin keamanan host.
4. **Dynamic Virtual Host Routing:** Setiap aplikasi mendapatkan subdomain otomatis `{slug}.cuan.dojo.cc` yang dihubungkan ke port lokal dinamis (rentang 9000–9999) melalui integrasi dinamis Caddy Admin API v2 tanpa perlu me-restart web server.
5. **App Mode Flexibility:** Pengaturan `APP_MODE=marketing` melayani Landing Page PAS publik dengan showcase sponsorship, sedangkan `APP_MODE=studio` menyajikan Studio Dashboard langsung di root `/` untuk pengembang solo.
6. **100% Data Retention Guarantee:** Peningkatan skema Deployer bersifat murni aditif (`000008_deployer.up.sql`) sehingga data historis Sprint 1 (akun owner, task board, runner logs) dan Sprint 2 (sponsorships, leaderboard, waiting list) terjamin 100% utuh tanpa alterasi.

---

## 2. Diagram Arsitektur Sistem (E2E Architecture)

Berikut adalah diagram arsitektur alur kerja Cuan Deployer mulai dari intake proyek hingga penyajian via subdomain reverse proxy:

```d2
direction: right

client: "Pengembang / Pengguna" {
  shape: person
}

caddy: "Caddy Reverse Proxy & Dynamic Router" {
  style.fill: "#f8fafc"
}

cuan_core: "Cuan Server (Go Backend)" {
  style.fill: "#eff6ff"
}

intake_engine: "Storage & Intake Engine" {
  style.fill: "#fef3c7"
}

port_allocator: "Port Pool Allocator (9000-9999)" {
  style.fill: "#ecfdf5"
}

sandbox_runners: "Sandboxed Process Runner" {
  style.fill: "#f3e8ff"
  static_proc: "Static HTML Server"
  php_proc: "PHP Built-in Runtime"
  node_proc: "Node.js Microservice"
}

subdomain_live: "Subdomain Live ({slug}.cuan.dojo.cc)" {
  style.fill: "#dcfce7"
}

client -> caddy: "Akses Dashboard Studio / Root"
caddy -> cuan_core: "Proxy ke Port 8080 / 8081"
client -> cuan_core: "Upload ZIP / Input Git URL"
cuan_core -> intake_engine: "Validasi Anti-ZipSlip & Ekstraksi Terisolasi"
intake_engine -> cuan_core: "Deteksi Stack Otomatis (PHP/Node/Static)"
cuan_core -> port_allocator: "Alokasi Port Dinamis Bebas Tabrakan"
cuan_core -> sandbox_runners: "Spawn Subproses dengan PID & Group Isolation"
cuan_core -> caddy: "Injeksi Rute Virtual Host via Caddy Admin API"
caddy -> subdomain_live: "Terbitkan Subdomain Aktif dengan SSL Otomatis"
```

---

## 3. Runtime Stack yang Didukung & Mekanisme Deteksi

Cuan Deployer menganalisis struktur direktori proyek yang diekstrak untuk menentukan runtime yang paling sesuai secara otomatis:

| Runtime Stack | Penanda Deteksi (*Detection Marker*) | Perilaku Eksekusi & Command |
| --- | --- | --- |
| **Static HTML** | `index.html` | Menjalankan server berkas statis terisolasi (`cuan-static-server` / fallback Python3 / PHP) yang melayani HTML, CSS, JavaScript, dan media. |
| **PHP / WordPress** | `index.php` atau `wp-config.php` | Menjalankan built-in PHP web server (`php -S 127.0.0.1:{port} -t {appDir}`) dengan variabel lingkungan terinjeksi. |
| **Node.js Tool** | `package.json` | Menjalankan script utama yang ditentukan di `package.json` (`main` atau script `start`), atau mendeteksi berkas `server.js` / `index.js` / `app.js`. |
| **Custom / Fallback** | Tidak ada penanda spesifik di atas | Disiapkan sebagai basis kustom dengan konfigurasi lingkungan manual. |

---

## 4. Keamanan Sandboxing & Perlindungan Anti-ZipSlip

Keamanan host merupakan prioritas tertinggi pada Cuan Deployer. Mengingat pengembang dapat mengunggah arsip berkas dari berbagai sumber, Cuan menerapkan 4 lapis perlindungan:

### 1. Anti-ZipSlip Traversal Defense
Parser ekstraksi arsip ZIP (`deployer.ExtractZip`) memeriksa setiap entri berkas:
- **Pemeriksaan Pola:** Setiap path berkas yang mengandung komponen traversal `..` langsung digagalkan dan memicu error `zipslip detected: path contains '..'`.
- **Penolakan Path Absolut:** Berkas yang memiliki nama berawalan `/` atau `\` (Windows backslash) ditolak keras.
- **Relatif Boundary Check:** Menggunakan `filepath.Clean` dan `filepath.Rel` untuk memvalidasi bahwa jalur target berkas mutlak berada di dalam direktori aplikasi pengguna (`data/deployments/{id}/app/`).
- **Pencegahan Symlink Escape:** Tautan simbolik (*symlink*) yang mengarah ke luar batas direktori aplikasi atau menunjuk path sistem divalidasi dan ditolak.

### 2. Process Group Isolation
Setiap proses aplikasi anak (*child process*) dijalankan dalam *process group* tersendiri (`SysProcAttr.Setpgid = true`). Hal ini memastikan bahwa ketika sebuah aplikasi dihentikan (*Stop*), seluruh sub-proses anak dan worker-nya ditutup tuntas tanpa menyisakan proses yatim (*zombie/orphan process*).

### 3. Port Pool Allocator (Zero-Collision)
Rentang port lokal `9000–9999` dikelola secara *thread-safe* oleh `deployer.PortPool`. Sebelum port diberikan, sistem melakukan probe soket TCP riil untuk memastikan port tersebut benar-benar belum digunakan oleh proses lain di host.

---

## 5. Tata Cara Melakukan Deployment Proyek

Terdapat tiga cara untuk mendeploy aplikasi ke Cuan Deployer:

### Cara 1: Upload Arsip ZIP via Studio Dashboard (Direkomendasikan)
1. Buka browser dan arahkan ke `http://cuan.dojo.cc/studio` (atau root `/` jika `APP_MODE=studio`).
2. Klik tombol **Deploy New Project** di sudut kanan atas.
3. Pada tab **Upload ZIP**:
   - Tarik dan lepas berkas `.zip` proyek Anda (atau klik untuk memilih berkas).
   - Masukkan **Project Name** (misalnya `Portfolio Saya`).
   - Sistem akan membuat **Subdomain Slug** otomatis (misalnya `portfolio-saya`). Anda dapat mengubah slug ini sesuai keinginan.
   - (Opsional) Buka menu akordeon **Environment Variables** untuk menambahkan variabel konfigurasi dalam format JSON (misalnya `{"APP_ENV": "production", "DB_NAME": "cuan_db"}`).
4. Klik **Deploy Application**.
5. Indikator progress akan menampilkan tahapan: upload berkas, ekstraksi aman, deteksi stack otomatis, alokasi port, spawning proses, hingga aktivasi rute subdomain.

### Cara 2: Deploy dari Repositori Git
1. Pada modal wizard **Deploy New Project**, pilih tab **Git Repository**.
2. Masukkan URL repositori Git publik atau privat Anda (misalnya `https://github.com/buchin/my-tool.git`).
3. Tentukan nama branch (default: `main`).
4. Klik **Deploy Application**. Sistem akan melakukan shallow clone (`--depth 1`), mendeteksi runtime stack, dan menjalankan aplikasi.

### Cara 3: Melalui CLI / cURL API
Anda dapat mendeploy proyek langsung dari terminal menggunakan endpoint REST API:

```bash
# Upload ZIP via cURL
curl -X POST http://127.0.0.1:8080/api/deployments/upload \
  -F "file=@/path/to/my-project.zip" \
  -F "project_name=My PHP App" \
  -F "slug=my-php-app" \
  -F 'env_vars={"APP_ENV":"production"}'
```

Respons JSON:
```json
{
  "deployment": {
    "id": "dep_a1b2c3d4e5f6",
    "project_name": "My PHP App",
    "slug": "my-php-app",
    "runtime": "php",
    "status": "pending",
    "domain": "my-php-app.cuan.dojo.cc"
  },
  "detected_runtime": "php",
  "status": "pending"
}
```

Untuk menyalakan proses dan menghubungkan subdomain:
```bash
curl -X POST http://127.0.0.1:8080/api/deployments/dep_a1b2c3d4e5f6/start
```

---

## 6. Konfigurasi Lingkungan (*Environment Variables*)

Berikut adalah variabel lingkungan utama untuk mengatur perilaku Cuan Server dan Caddy:

| Variabel Lingkungan | Nilai Default | Deskripsi & Dampak Operasional |
| --- | --- | --- |
| `APP_MODE` | `marketing` | Menentukan tampilan root `/`: `marketing` menyajikan Landing Page PAS & Sponsorship Cuan; `studio` menyajikan Studio Dashboard langsung di root `/`. |
| `BASE_DOMAIN` | `cuan.dojo.cc` | Domain dasar untuk wildcard reverse proxy Caddy. Aplikasi pengguna akan dipetakan ke `{slug}.${BASE_DOMAIN}`. |
| `CADDY_ADMIN_URL` | `http://localhost:2019` | Alamat endpoint Caddy Admin API v2 untuk mendaftarkan dan mencabut rute reverse proxy secara dinamis. |
| `PORT` | `8080` (prod) / `8081` (staging) | Port listening server HTTP utama Cuan. |
| `CUAN_AUTH_SECRET` | *(string acak 32 karakter)* | Kunci rahasia enkripsi sesi Single-Owner untuk melindungi dashboard internal. |
| `CUAN_STATIC_DIR` | `web/dist` | Direktori berkas hasil kompilasi frontend React + Vite. |
| `CUAN_AUTO_MIGRATE` | `true` | Menjalankan migrasi skema database secara otomatis saat server boot. |

### Injeksi Variabel Otomatis ke Proses Pengguna
Saat Cuan Deployer menjalankan aplikasi pengguna, sistem secara otomatis menginjeksi:
- `PORT={internal_port}` (misalnya `9002`)
- `HOST=127.0.0.1`
- `CUAN_PORT={internal_port}`
- Seluruh key-value kustom yang dimasukkan pengguna pada field `env_vars`.

---

## 7. Kontrol Siklus Hidup (*Lifecycle Management*) di Dashboard

Pada Studio Dashboard (`/studio`), setiap deployment memiliki **Project Drawer** interaktif yang menyediakan kontrol 1-klik:

### Aksi Kontrol
- **Start:** Mengalokasikan port baru dari pool, menyalakan subproses runtime, mengaktifkan streaming log, dan mendaftarkan rute reverse proxy di Caddy.
- **Stop:** Mengirimkan sinyal `SIGTERM` secara anggun (*graceful*) ke seluruh process group, menunggu hingga 2 detik (fallback ke `SIGKILL` jika macet), mengembalikan port ke pool, dan mencabut rute Caddy.
- **Restart:** Menghentikan proses yang sedang berjalan dan menyalakannya kembali secara atomik tanpa risiko tabrakan port.
- **Delete:** Menghentikan proses, mencabut rute subdomain, menghapus seluruh berkas aplikasi di `data/deployments/{id}`, dan menghapus record database secara permanen.

### Real-Time Log Viewer
Project Drawer menyediakan tab **Logs** yang menyiarkan log `stdout`, `stderr`, dan log sistem secara real-time. Hal ini memudahkan proses debugging tanpa harus membuka terminal server host.

### Graceful Fallback Booting (503 Page)
Jika pengguna mengakses subdomain `{slug}.cuan.dojo.cc` saat aplikasi sedang dalam proses inisialisasi boot atau backend belum siap mengikat port, Caddy menyajikan halaman fallback ramah pengguna (*Aplikasi Sedang Dimuat*) dengan status HTTP 503 dan auto-refresh otomatis setiap 5 detik.

---

## 8. Panduan Troubleshooting & FAQ Operasional

### Q1: Upaya upload ZIP gagal dengan pesan "security violation: zipslip detected"
**Penyebab:** Arsip ZIP yang diunggah memuat berkas dengan jalur relatif `..` atau path absolut yang berpotensi menembus direktori luar.
**Solusi:**
1. Ekstrak arsip di komputer lokal Anda.
2. Buat ulang file ZIP baru dari dalam folder proyek (jangan menyertakan symlink yang rusak atau direktori induk `..`).
3. Unggah kembali berkas ZIP yang baru dibersihkan.

### Q2: Subdomain mengembalikan 502 Bad Gateway atau 503 Service Unavailable
**Penyebab:** Subproses aplikasi mati (*crash*) segera setelah start atau port backend belum siap menerima koneksi.
**Solusi:**
1. Buka Studio Dashboard dan klik proyek terkait untuk membuka **Project Drawer**.
2. Masuk ke tab **Logs** dan periksa output `stderr`.
3. Untuk aplikasi Node.js, pastikan `package.json` menyertakan script `start` atau berkas entri (`server.js` / `index.js`).
4. Untuk aplikasi PHP, periksa apakah terdapat error sintaks fatal di berkas `index.php`.
5. Klik tombol **Restart** untuk mencoba kembali inisialisasi runtime.

### Q3: Status mode aplikasi salah (Landing Page tampil alih-alih Studio)
**Penyebab:** Variabel lingkungan `APP_MODE` bernilai `marketing` atau belum disetel ke `studio`.
**Solusi:**
1. Buka berkas konfigurasi lingkungan host (`.env`).
2. Setel nilai: `APP_MODE=studio`.
3. Restart Cuan server (`systemctl restart cuan` atau jalankan ulang biner server).
4. Anda juga dapat menggunakan tombol **Mode Switcher** yang tersedia di navigation header web untuk beralih tampilan seketika.

### Q4: Port Pool Exhaustion (Semua port 9000-9999 terpakai)
**Penyebab:** Terdapat banyak deployment yang berjalan simultan atau ada proses yatim yang belum terlepas.
**Solusi:**
1. Hentikan aplikasi yang sudah tidak aktif via Studio Dashboard (tombol *Stop*).
2. Periksa status proses host menggunakan command: `ss -tulpn | grep 900`.
3. Cuan server secara otomatis melepaskan port saat deployment dihentikan atau dihapus.

---

## 9. Verifikasi Retensi Data & Jaminan Mutu

Cuan Deployer telah melalui targeted automated drill E2E menyeluruh (`internal/server/verify_s03_6_staging_e2e_test.go`):
- Seluruh 3 runtime (Static, PHP, Node.js) terbukti live dan responsif via subdomain.
- Penetrasi keamanan anti-ZipSlip (5 vektor serangan traversal) terbukti digagalkan 100%.
- Migrasi database `000008_deployer.up.sql` diverifikasi murni aditif tanpa menyentuh tabel pengguna Sprint 1 dan tabel sponsorship Sprint 2.
- Bukti audit tersimpan di `docs/sprints/evidence/S3/s03_6_staging_e2e_release_gate_verified.txt`.
