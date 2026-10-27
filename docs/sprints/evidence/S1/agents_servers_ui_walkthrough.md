# Walkthrough UI Agents & Servers serta Observabilitas Runner (Sprint 1 — T5)

Dokumen ini mendokumentasikan fitur antarmuka pengguna (UI) **Agents & Servers**, visualisasi metrik resource host runner, profil agent, kontrol attempt (Cancel & Retry), serta penampil log streaming dengan credential masking dan rotasi log yang diimplementasikan pada task T5 sesuai [S1.md](../../../docs/sprints/S1.md).

---

## 1. Navigasi & Menu "Agents & Servers"

Pada bar navigasi utama Cuan (header aplikasi web), telah ditambahkan tab navigasi baru:
- **`Agents & Servers`** (dengan ikon server 🖥️).
- Tab ini dapat diakses oleh owner secara langsung berdampingan dengan tab **Projects** dan **Tasks Board**.

---

## 2. Bagian 1: Runner Nodes & Infrastruktur (Server Observability)

Tampilan utama tab **Agents & Servers** menyajikan dasbor monitoring node runner yang terhubung:

### 2.1 Kartu Node Runner & Status Heartbeat
- **Runner ID & Nama Host**: Menampilkan identitas unik runner (contoh: `default-runner` atau nama instance VM/bare-metal).
- **Badge Status Real-Time**:
  - `🟢 Online / Active`: Runner mengirimkan heartbeat secara berkala dalam batas lease (30 detik).
  - `🔴 Offline`: Runner tidak mengirimkan heartbeat melebihi durasi lease, menandakan runner crash atau koneksi terputus.
- **Timestamp Heartbeat Terakhir**: Menampilkan waktu tepat perpanjangan sewa terakhir (contoh: `2 detik yang lalu`).
- **Antrean Tugas (Queue Depth)**: Menampilkan jumlah attempt yang sedang `running` (maksimum 1 per runner sesuai batas konkurensi) dan jumlah attempt yang sedang `queued` (menunggu giliran).

### 2.2 Visualisasi Metrik Resource Host (Host Telemetry)
Setiap runner melaporkan metrik performa aktual host Linux tempat runner berjalan:
1. **CPU Load Average**:
   - Menampilkan angka beban CPU untuk 1 menit, 5 menit, dan 15 menit dari `/proc/loadavg`.
   - Dilengkapi *progress bar visual* dengan indikator batas aman beban sistem.
2. **Penggunaan Memori (RAM)**:
   - Dihitung dari `/proc/meminfo` secara real-time (`MemTotal`, `MemAvailable`).
   - Menampilkan total RAM sistem (contoh: `31.3 GB`), RAM terpakai, RAM tersedia, serta persentase pemakaian.
3. **Penyimpanan Disk (Storage)**:
   - Dihitung menggunakan syscall `statfs` pada direktori kerja runner.
   - Menampilkan kapasitas total disk (contoh: `1.76 TB`), disk terpakai, ruang bebas, dan persentase utilisasi filesystem.

---

## 3. Bagian 2: Profil Agent & Kapabilitas Eksekusi

Di bawah panel server node, terdapat daftar profil AI coding agent yang terintegrasi:

### 3.1 Kartu Profil Agent (`agy` — Antigravity Adapter)
- **Nama Adapter**: `agy` (Google Antigravity CLI Adapter — hasil validasi spike T1).
- **Status Adapter**: `🟢 Siap Digunakan (Ready)`.
- **Lokasi Binari**: `/home/dev/.gemini/antigravity-cli/bin/agy` (terverifikasi mandiri pada sistem).
- **Isolasi Proses**: `Process Group (PGID)` — mendukung terminasi pohon proses lengkap (`KillProcessTree`).
- **Daftar Kapabilitas**:
  - `git_commit`: Pembuatan commit git dengan identitas author yang valid.
  - `pr_create`: Pembuatan Pull Request otomatis ke upstream repository.
  - `shell_exec`: Eksekusi command shell terisolasi di workspace task.
  - `log_streaming`: Streaming log output baris-demi-baris dengan mask credential otomatis.
- **Metrik Historis Puncak RAM**:
  - Puncak konsumsi RSS memori terukur pada pengujian T1/T5: **~207.1 MB** (jauh di bawah batas aman kapasitas host).

---

## 4. Bagian 3: Kontrol Attempt & Penampil Log pada Task Detail View

Pada tampilan **Task Detail View** (diakses dari tab *Tasks Board*), telah ditambahkan kontrol eksekusi runner dan penampil log langsung:

### 4.1 Tombol Kontrol Attempt Baru: Cancel & Retry
1. **Tombol "Cancel Task" (Merah)**:
   - Tersedia saat attempt task sedang berada dalam status `running`.
   - Mengirim sinyal pembatalan ke backend Cuan Server yang langsung diteruskan ke runner.
   - Runner memanggil `KillProcessTree` untuk mematikan PID parent dan seluruh subprocess background (`-PID`) tanpa meninggalkan proses zombie.
   - Status attempt bertransisi menjadi `canceled`.
2. **Tombol "Retry Task" (Biru)**:
   - Tersedia jika attempt task berada pada status `failed` atau `interrupted`.
   - Mendaftarkan attempt baru berstatus `queued` (`Attempt #2`, `#3`, dst.) tanpa menghapus data audit attempt sebelumnya.
   - Runner yang online akan langsung mengambil attempt baru tersebut.

### 4.2 Terminal Monospace Penampil Log (Log Streaming Viewer)
- Terletak di bawah panel attempt pada Task Detail View.
- Mengadopsi tema gelap monokrom (*dark terminal*) dengan font monospace (`Courier New` / `Fira Code`).
- **Informasi Tiap Baris Log**:
  - Timestamp ISO (`[HH:MM:SS]`).
  - Indikator stream: `[stdout]` (biru kehijauan) atau `[stderr]` (oranye kemerahan).
  - Pesan log aktual.
- **Penyensoran Kredensial Otomatis (Credential Masking)**:
  - Seluruh token rahasia (contoh: `ghp_xxxxxxxxxxxx`, `Bearer ey...`, kata sandi database `postgres://user:pass@host/db`) otomatis disensor menjadi `[REDACTED]`.
  - Terverifikasi pada unit test `secret_masker_test.go` dan `verify_t5_test.go`.
- **Perlindungan Memori & Rotasi Log (Bounded Logs)**:
  - Jumlah baris log dibatasi hingga maksimal 1.000 baris per attempt (dapat dikonfigurasi melalui `RUNNER_MAX_LOG_LINES`).
  - Baris tertua otomatis dirotasi sehingga tidak membebani memori browser atau menghabiskan kapasitas disk server.

---

## 5. Ringkasan Kesesuaian Kriteria DoD
- ✅ Visualisasi status runner & host telemetry (CPU, RAM, Disk).
- ✅ Profil agent `agy` dengan kapabilitas dan isolasi proses.
- ✅ Tombol Cancel untuk menghentikan process tree secara instan.
- ✅ Tombol Retry untuk membuat attempt baru secara eksplisit.
- ✅ Log streaming dengan terminal viewer, credential masking `[REDACTED]`, dan batas rotasi log.
