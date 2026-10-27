# Panduan Operasional Bring Your Own Server (BYOS) & Remote Worker Orchestration

Dokumen ini merupakan panduan operasional komprehensif bagi pemilik instance (*Single-Owner*), pengembang mandiri (*Solo Developer*), agensi digital, dan Indie Hacker mengenai tata cara menghubungkan remote VPS/server pribadi, mengonfigurasi autentikasi SSH keypair yang aman, mengelola hak akses dan port kustom, mendeploy aplikasi web ke worker remote, serta memecahkan kendala konektivitas (*troubleshooting*) pada Cuan BYOS.

---

## 1. Filosofi & Gambaran Umum

Cuan BYOS (Bring Your Own Server) dirancang dengan filosofi **Agentless Remote Orchestration** dan **Zero-Friction Operations**. Pengguna dapat menghubungkan server VPS Linux dari penyedia mana pun (seperti Hetzner, DigitalOcean, Linode, Contabo, AWS EC2, GCP Compute Engine, Vultr, atau server fisik on-premise) ke Cuan Control Plane tanpa perlu menginstal perangkat lunak agen (*agent daemon*) tambahan pada server target.

### Prinsip Utama BYOS Cuan
1. **Agentless SSH Orchestration:** Seluruh interaksi, health probe, transfer berkas, dan manajemen siklus hidup proses dijalankan langsung melalui protokol standar OpenSSH yang aman.
2. **Keamanan Kredensial At-Rest & In-Flight:** Seluruh SSH private key yang didaftarkan dienkripsi simetris menggunakan AES-256-GCM dengan master encryption key vault (`VAULT_MASTER_KEY`). Field private key maupun ciphertext dijamin tidak akan bocor ke publik melalui penanda serialisasi DTO `json:"-"`.
3. **Isolasi Direktori & Anti-Path Traversal:** Setiap aplikasi remote ditempatkan di direktori terisolasi `~/cuan-apps/<deployment_id>/`. Cuan memvalidasi format ID dan berkas arsip secara ketat untuk menangkal eksploitasi path traversal (`../`) dan command injection.
4. **Target Deployment Fleksibel:** Pengembang bebas memilih target deployment pada modal deploy di Studio Dashboard: **Local Sandbox (Co-located)** atau **Remote Server (BYOS)** dengan pengalaman pengguna (*UX*) yang seragam.
5. **Real-Time Streaming & Health Poller:** Output stdout/stderr proses remote disiarkan secara langsung (*live streaming*) ke browser, disertai background poller yang mendeteksi status hidup (*alive*), berhenti (*stopped*), maupun crash (*crashed*) secara otomatis.
6. **100% Data Retention Guarantee:** Skema database PostgreSQL diperluas secara murni aditif (`000009_servers_byos.up.sql`), menjamin seluruh data Sprint 1 (akun owner, task board), Sprint 2 (sponsorships, leads, orders), dan Sprint 3 (local deployments, routing Caddy) tetap 100% utuh tanpa alterasi.

---

## 2. Diagram Arsitektur Sistem (E2E Architecture)

Berikut adalah diagram arsitektur alur kerja Cuan BYOS mulai dari pendaftaran server, enkripsi key vault, inspeksi spesifikasi sistem remote, hingga orkestrasi deploy bundle aplikasi:

```d2
direction: right

client: "Browser / Pengembang (Studio UI)" {
  shape: person
}

cuan_control_plane: "Cuan Control Plane (Go Backend)" {
  api: "Chi HTTP Router\n(/api/servers & /api/deployments)"
  vault: "KeyVault Engine\n(AES-256-GCM At-Rest)"
  ssh_pool: "SSH Connection Pool\n(5s Timeout & Keepalive)"
  driver_factory: "DeployDriver Factory" {
    local_driver: "LocalDriver\n(Co-located S3)"
    remote_driver: "RemoteSSHDriver\n(BYOS Remote Worker)"
  }
  poller: "Process Health Poller\n(PID & Port Monitor)"
}

caddy: "Caddy Dynamic Reverse Proxy\n({slug}.cuan.dojo.cc)"

db: "PostgreSQL Database" {
  table_servers: "servers (000009)"
  table_deployments: "deployments (+ server_id)"
  table_logs: "deployment_logs"
}

remote_vps: "Remote Target Server (BYOS VPS)" {
  sshd: "OpenSSH Daemon (:22 / Custom)"
  sandbox: "Direktori Sandbox\n~/cuan-apps/<deployment_id>/" {
    app_runner: "Application Runner\n(Node.js / PHP / Static)"
    log_file: "app.log (stdout/stderr)"
    pid_file: "app.pid"
  }
}

client -> cuan_control_plane.api: "1. Hubungkan Server (Host, Port, User, SSH Key)"
cuan_control_plane.api -> cuan_control_plane.vault: "Enkripsi Kredensial via AES-256-GCM"
cuan_control_plane.vault -> db.table_servers: "Simpan Record Server Terenkripsi"
cuan_control_plane.api -> cuan_control_plane.ssh_pool: "Uji Koneksi & Health Probe"
cuan_control_plane.ssh_pool -> remote_vps.sshd: "Handshake & Fingerprint (OS, CPU, RAM)"

client -> cuan_control_plane.api: "2. Deploy Project (Target: BYOS Server)"
cuan_control_plane.api -> cuan_control_plane.driver_factory.remote_driver: "Rute ke RemoteSSHDriver"
cuan_control_plane.driver_factory.remote_driver -> cuan_control_plane.vault: "Dekripsi SSH Private Key"
cuan_control_plane.driver_factory.remote_driver -> remote_vps.sshd: "Transfer Bundle via Tar/SSH Stream"
remote_vps.sshd -> remote_vps.sandbox.app_runner: "Eksekusi nohup Runner di Background"
remote_vps.sandbox.app_runner -> remote_vps.sandbox.pid_file: "Catat PID Proses"

cuan_control_plane.driver_factory.remote_driver -> caddy: "Daftarkan Subdomain Virtual Host"
cuan_control_plane.poller -> remote_vps.sshd: "Polling Berkala (kill -0 & Port Status)"
remote_vps.sandbox.log_file -> cuan_control_plane.api: "Streaming Log Real-Time (tail -f)"
cuan_control_plane.api -> client: "Tampilkan Log Langsung di UI Studio"
```

---

## 3. Prasyarat & Persiapan Server Remote (Server Prerequisites)

Sebelum mendaftarkan server ke Cuan, pastikan server remote memenuhi spesifikasi berikut:

### Sistem Operasi yang Didukung
- **Ubuntu:** 20.04 LTS, 22.04 LTS, 24.04 LTS *(Rekomendasi Utama)*
- **Debian:** 11 (Bullseye), 12 (Bookworm)
- **RHEL / CentOS / Rocky Linux / AlmaLinux:** 8, 9
- **Alpine Linux:** 3.16+

### Spesifikasi Minimum Hardware
- **CPU:** 1 vCPU (2+ vCPU untuk workload aplikasi berat)
- **RAM:** 1 GB (2+ GB disarankan untuk build Node.js atau WordPress)
- **Disk:** Minimal 10 GB ruang kosong untuk sistem operasi dan direktori `~/cuan-apps/`

### Utilitas Dasar yang Wajib Tersedia
Pastikan paket sistem berikut sudah terinstal di server target:
```bash
# Ubuntu / Debian:
sudo apt-get update && sudo apt-get install -y openssh-server tar curl ca-certificates

# RHEL / Rocky / AlmaLinux:
sudo dnf install -y openssh-server tar curl ca-certificates
```

### Runtime Sesuai Kebutuhan Aplikasi
- **Aplikasi Node.js:** Node.js (v18+) dan npm (`node -v`, `npm -v`)
- **Aplikasi PHP / WordPress:** PHP CLI dan modul dasar (`php -v`, `php-mbstring`, `php-xml`, `php-curl`)
- **Aplikasi Static HTML:** Paket dasar Linux (Cuan dapat menggunakan built-in static server atau runtime bawaan)

---

## 4. Panduan Konfigurasi SSH Keypair & `authorized_keys`

Cuan memanfaatkan autentikasi berbasis kunci publik/privat (*Public Key Authentication*) untuk koneksi tanpa intervensi kata sandi.

### 4.1 Membuat SSH Keypair Baru (Rekomendasi Ed25519)
Kunci Ed25519 menawarkan keamanan kriptografi tinggi, ukuran kunci ringkas, dan performa handshake yang sangat cepat:

```bash
# Generate Ed25519 keypair khusus untuk Cuan
ssh-keygen -t ed25519 -a 100 -C "cuan-byos-orchestrator" -f ~/.ssh/id_cuan_ed25519 -N ""
```
Perintah di atas menghasilkan 2 berkas:
- `~/.ssh/id_cuan_ed25519`: **Private Key** (Unggah atau salin isi berkas ini ke form Cuan Studio).
- `~/.ssh/id_cuan_ed25519.pub`: **Public Key** (Tambahkan ke server target).

*(Opsi Alternatif: RSA 4096-bit jika server lama belum mendukung Ed25519)*
```bash
ssh-keygen -t rsa -b 4096 -C "cuan-byos-orchestrator" -f ~/.ssh/id_cuan_rsa -N ""
```

### 4.2 Memasang Public Key ke `authorized_keys` di Server Target
Masuk ke remote server Anda via SSH manual terlebih dahulu, lalu tambahkan baris public key:

```bash
# 1. Pastikan direktori .ssh memiliki permission yang tepat
mkdir -p ~/.ssh
chmod 700 ~/.ssh

# 2. Tambahkan public key ke authorized_keys
cat << 'EOF' >> ~/.ssh/authorized_keys
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... cuan-byos-orchestrator
EOF

# 3. Kunci izin akses berkas authorized_keys
chmod 600 ~/.ssh/authorized_keys
```

> [!IMPORTANT]
> Izin akses (*file permissions*) pada `~/.ssh` (700) dan `~/.ssh/authorized_keys` (600) adalah syarat mutlak OpenSSH. Jika permission terlalu longgar (misal 777), OpenSSH daemon akan menolak koneksi secara otomatis (*StrictModes*).

### 4.3 Rekomendasi User & Hak Akses (Privilege Recommendations)
- **User Non-Root (Sangat Direkomendasikan):** Gunakan user standar (misal `ubuntu` atau buat user khusus `cuan`):
  ```bash
  sudo adduser --disabled-password --gecos "" cuan
  sudo usermod -aG sudo cuan  # jika butuh akses manajemen paket
  ```
- **Sudoers NOPASSWD (Opsional jika aplikasi butuh install dependencies):**
  ```bash
  echo "cuan ALL=(ALL) NOPASSWD:ALL" | sudo tee /etc/sudoers.d/99-cuan-nopasswd
  sudo chmod 440 /etc/sudoers.d/99-cuan-nopasswd
  ```

---

## 5. Konfigurasi Port SSH Kustom & Firewall

Mengubah port standar SSH dari `22` ke port kustom (misal `2222` atau `50222`) adalah salah satu mitigasi efektif dari serangan pemindaian otomatis botnet.

### 5.1 Mengubah Port SSH di Server Target
Edit berkas konfigurasi OpenSSH:
```bash
sudo nano /etc/ssh/sshd_config
```
Cari baris `Port 22` dan ubah menjadi:
```text
Port 2222
PubkeyAuthentication yes
AuthorizedKeysFile .ssh/authorized_keys
```

### 5.2 Membuka Port pada Firewall
Sebelum merestart sshd, pastikan port baru telah diizinkan oleh firewall sistem:
```bash
# Jika menggunakan UFW (Ubuntu/Debian):
sudo ufw allow 2222/tcp comment "Cuan SSH Port"
sudo ufw reload

# Jika menggunakan firewalld (RHEL/Rocky/Alma):
sudo firewall-cmd --permanent --add-port=2222/tcp
sudo firewall-cmd --reload
```

Kemudian restart SSH daemon:
```bash
# Ubuntu / Debian:
sudo systemctl restart ssh

# RHEL / CentOS:
sudo systemctl restart sshd
```

> [!WARNING]
> Jangan langsung menutup sesi SSH lama sebelum menguji koneksi baru di terminal lain (`ssh -p 2222 user@host`) untuk mencegah potensi terkunci di luar server (*lockout*).

---

## 6. Langkah Pendaftaran & Pengujian Koneksi di Cuan Studio UI

1. Buka **Studio Dashboard** pada instance Cuan Anda.
2. Navigasikan ke tab **Servers** di sidebar atau menu navigasi atas (`/studio/servers`).
3. Klik tombol **"Connect Server"** di pojok kanan atas.
4. Isi formulir pendaftaran:
   - **Server Name:** Berikan nama pengenal yang deskriptif (contoh: `Hetzner Singapore Node 01` atau `Contabo Worker`).
   - **Host / IP Address:** Masukkan IP Publik server remote (contoh: `203.0.113.50`) atau domain FQDN.
   - **SSH Port:** Masukkan port SSH (default: `22`, atau port kustom misal `2222`).
   - **Username:** Masukkan nama pengguna remote (default: `ubuntu` atau `cuan`).
   - **Authentication Type:** Pilih `SSH Private Key`.
   - **Private Key:** Tempelkan seluruh isi private key yang dibuat pada langkah 4 (dimulai dari `-----BEGIN OPENSSH PRIVATE KEY-----` atau `-----BEGIN PRIVATE KEY-----`).
5. Klik tombol **"Test Connection"**:
   - Sistem akan melakukan handshake SSH aman (dengan batas waktu timeout 5 detik).
   - Memeriksa fingerprint kunci publik dan mengeksekusi probe spesifikasi sistem remote (`/etc/os-release`, core CPU, kapasitas RAM, dan latensi round-trip ping).
   - Menampilkan badge status **"Online"** berwarna hijau dan rincian spesifikasi perangkat.
6. Klik **"Save Server"** untuk menyimpan server ke database. Kredensial private key akan otomatis dienkripsi dengan AES-256-GCM.

---

## 7. Deploy Proyek ke Remote Server (BYOS Deployment Workflow)

Setelah server terhubung, mendeploy aplikasi ke VPS remote semudah mendeploy secara lokal:

1. Klik tombol **"Deploy New Project"** pada Studio Dashboard.
2. Unggah berkas proyek Anda dalam format ZIP (misal `my-app.zip`) atau masukkan URL repositori Git.
3. Pada dropdown **"Deployment Target"**, pilih nama remote server yang telah Anda daftarkan (misal: `Hetzner Singapore Node 01`).
4. Berikan nama proyek dan slug subdomain (misal `api-worker`).
5. Klik **"Deploy"**:
   - Cuan mengemas berkas bundle dan mentransfernya langsung via stream terisolasi ke remote server di jalur `~/cuan-apps/<deployment_id>/`.
   - Cuan mendeteksi runtime stack secara otomatis (PHP, Node.js, atau Static).
   - Sistem mengalokasikan internal port bebas tabrakan (rentang 9000–9999).
   - Mengeksekusi runner di background:
     ```bash
     nohup env PORT=<port> <cmd> > app.log 2>&1 & echo $! > app.pid
     ```
   - Caddy dynamic router otomatis mendaftarkan subdomain `{slug}.cuan.dojo.cc` mengarah ke host dan port aplikasi remote.

---

## 8. Monitoring, Real-time Log Streaming & Health Poller

### Real-Time Streaming Logs
- Pengguna dapat membuka drawer detail proyek di Studio Dashboard dan mengeklik tab **Logs**.
- Cuan melakukan tailing berkas log remote secara streaming langsung ke antarmuka web, menampilkan setiap baris stdout dan stderr secara instan.
- Jika terjadi gangguan jaringan sesaat (*network blip*), tailer akan otomatis menyambung kembali (*auto-reconnect*) tanpa duplikasi entri log.

### Background Process Health Poller
- Cuan menjalankan background worker yang memeriksa status proses aplikasi di remote server secara periodik (setiap 5 detik).
- Poller mengecek PID file dan mengirim sinyal probe liveness `kill -0 <pid>`.
- Jika proses remote berhenti tak terduga (*crash*), Cuan langsung menandai status deployment sebagai `crashed`, melepaskan port lease, mencatat audit log, dan menampilkan peringatan di dashboard.

### Kontrol Siklus Hidup Aplikasi (Lifecycle Controls)
Melalui drawer kontrol di Studio Dashboard, pengguna dapat:
- **Stop:** Menghentikan proses remote secara graceful (SIGTERM) dan mencabut rute subdomain.
- **Start:** Menjalankan ulang aplikasi dan mendaftarkan kembali rute subdomain.
- **Restart (1-Click):** Menghentikan lalu menyalakan ulang proses secara instan tanpa konflik port.
- **Delete:** Menghapus proses dan membersihkan direktori `~/cuan-apps/<id>` di remote VPS.

---

## 9. Panduan Pemecahan Masalah (Troubleshooting & FAQ)

| Gejala Masalah | Penyebab Umum | Langkah Solusi & Verifikasi |
| --- | --- | --- |
| **Connection Refused (Port 22/Kustom)** | SSH daemon mati atau port ditutup firewall | Jalankan `sudo systemctl status ssh` di server remote. Periksa aturan firewall: `sudo ufw status`. Pastikan port yang dimasukkan di form Cuan sesuai dengan `Port` di `/etc/ssh/sshd_config`. |
| **Permission Denied (publickey)** | Public key belum masuk ke `authorized_keys` atau permission salah | Pastikan public key terpasang di `~/.ssh/authorized_keys`. Periksa permission: `chmod 700 ~/.ssh` dan `chmod 600 ~/.ssh/authorized_keys`. Pastikan pemilik berkas adalah user yang bersangkutan (`chown -R $USER:$USER ~/.ssh`). |
| **Connection Timeout (5s Exceeded)** | Security Group / Provider Firewall memblokir IP Cuan | Pastikan firewall di level cloud provider (AWS Security Group, Hetzner Cloud Firewall, DigitalOcean Cloud Firewall) mengizinkan lalu lintas TCP inbound pada port SSH yang ditentukan. |
| **Invalid Private Key Format** | Private key terpotong atau memiliki passphrase | Cuan membutuhkan private key tanpa passphrase (*unencrypted private key file*) agar dapat melakukan enkripsi simetris at-rest via KeyVault. Pastikan header `-----BEGIN ... PRIVATE KEY-----` dan footer lengkap tercopy. |
| **Command Not Found (Node/PHP) saat Deploy** | Runtime belum terinstal di remote server | Masuk ke remote server via SSH dan install runtime yang dibutuhkan (`sudo apt install -y nodejs npm` atau `sudo apt install -y php-cli`). Jalankan `node -v` atau `php -v` untuk memastikan binary berada dalam `$PATH` standar. |
| **Process Exited Immediately (Crashed)** | Kesalahan konfigurasi aplikasi atau port bentrok | Buka tab **Logs** di Studio Dashboard untuk melihat pesan error stdout/stderr dari aplikasi. Periksa apakah script start membutuhkan environment variables tambahan (`PORT`, `NODE_ENV`, dsb). |

---

## 10. Jaminan Keamanan & Audit Kredensial (Zero-Leakage Assurance)

Cuan menjamin standar keamanan tertinggi untuk kredensial server pengguna:
1. **Enkripsi AES-256-GCM At-Rest:** Private key SSH tidak pernah disimpan dalam format teks biasa (*plaintext*) di database. Nilai di kolom `encrypted_key` berupa ciphertext terautentikasi dengan nonce acak 12-byte unik per enkripsi.
2. **DTO Sanitization (`json:"-"`):** Representasi DTO internal maupun respons API HTTP publik (`/api/servers`, `/api/servers/{id}`, `/api/deployments`) tidak pernah menyertakan kolom `encrypted_key` maupun private key mentah.
3. **Pembersihan Memori:** Kunci privat yang didekripsi untuk keperluan handshake hanya bertahan di memori sesi aktif dan segera dibersihkan setelah koneksi terjalin.

Dokumen ini disusun sebagai single source of truth operasional fitur Bring Your Own Server (BYOS) Cuan. Untuk bantuan lebih lanjut, hubungi tim pengembang atau buka issue pada repositori [buchin/cuan](https://github.com/buchin/cuan).
