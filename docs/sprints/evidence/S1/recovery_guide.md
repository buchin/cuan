# Panduan Pemulihan (Recovery Guide): Runner Disconnect, Crash & Attempt Interrupted (Sprint 1 — T5)

Dokumen ini menyediakan panduan operasional standar (SOP) untuk mendeteksi, menginvestigasi, dan memulihkan kondisi ketika runner mengalami disconnect, crash, atau restart saat sedang mengeksekusi task coding agent pada sistem Cuan.

---

## 1. Arsitektur Heartbeat, Lease & Prinsip Safety

Cuan mengadopsi model *outbound-only authenticated runner* dengan mekanisme sewa waktu terbatas (*distributed lease*):
- **Runner Heartbeat**: Runner mengirimkan heartbeat berkala (setiap 10 detik) ke Cuan Server melalui endpoint POST `/api/v1/runner/heartbeat` yang memuat identitas runner, status host metrics (CPU, RAM, Disk), dan attempt ID yang sedang dikerjakan.
- **Lease Expiration**: Setiap perpanjangan heartbeat memberikan masa sewa aktif (*lease*) selama 30 detik (`RunnerLeaseDuration`).
- **Prinsip "No Quiet Auto-Reruns" (Tanpa Eksekusi Ulang Otomatis yang Diam-diam)**:
  Apabila runner terputus koneksinya atau mengalami kernel panic / crash, Cuan Server **TIDAK PERNAH** secara otomatis menjalankan ulang attempt yang terputus. 
  Hal ini krusial karena sebuah coding agent mungkin telah melakukan perubahan parsial (*side-effects*) seperti:
  - Commit git lokal di branch task,
  - Pembuatan file atau instalasi dependensi,
  - Eksekusi script pengujian atau migrasi database.
  Menjalankan ulang task secara diam-diam tanpa verifikasi dapat menyebabkan konflik git, inkonsistensi status, atau korupsi data.

---

## 2. Deteksi Kegagalan Runner & Status Interrupted

Ketika runner offline melebihi durasi lease (30 detik):
1. **Reconciliation Service**:
   Server menjalankan rekonsiliasi lease (melalui background scheduler atau saat pemanggilan endpoint `/api/v1/runner/reconcile`).
2. **Transisi Status**:
   - Status Runner pada tabel `runners` berubah menjadi `offline`.
   - Attempt yang sebelumnya berstatus `running` otomatis diubah statusnya menjadi **`interrupted`**.
   - Kolom `interrupted_at` dicatat dengan timestamp server saat sewa dinyatakan kedaluwarsa.
3. **Task Tetap Aman**:
   - Status Task induk tetap bertahan pada status kerja (`doing`) atau `review` dan tidak ditutup.
   - Di antarmuka UI, badge status attempt akan berubah menjadi oranye dengan label **`interrupted`**.

---

## 3. Langkah Investigasi Pra-Pemulihan (Pre-Recovery Inspection)

Sebelum memicu retry atau restart, operator atau owner wajib melakukan audit cepat:

### Langkah 3.1: Periksa Status Runner & Log Server
Cek daftar runner terdaftar dan status kesehatannya:
```bash
curl -s -H "Authorization: Bearer <OWNER_TOKEN>" http://localhost:8080/api/v1/runners | jq .
```
Pastikan apakah runner `offline` karena crash proses, network timeout, atau host restart.

### Langkah 3.2: Periksa Log Streaming Attempt yang Terputus
Ambil histori log eksekusi hingga detik terakhir sebelum runner terputus:
```bash
curl -s -H "Authorization: Bearer <OWNER_TOKEN>" \
  http://localhost:8080/api/v1/tasks/{task_id}/attempts/{attempt_id}/logs | jq .
```
Periksa log terakhir:
- Apakah agent sedang menjalankan command bash?
- Apakah proses terhenti karena Out of Memory (OOM)? (Cek metrik RAM puncak).
- Apakah ada commit git parsial yang sudah dibuat?

### Langkah 3.3: Bersihkan State Workspace / Process Tree Lokal Runner
Jika host runner masih menyala tetapi daemon runner hang:
1. Pastikan seluruh process tree yang terisolasi telah dimatikan:
   Daemon Cuan Runner menggunakan `KillProcessTree` yang membunuh seluruh process group (`-pid`) dengan sinyal `SIGKILL`.
2. Jalankan kembali daemon `cuan-runner`:
   ```bash
   ./bin/cuan-runner \
     --server-url http://localhost:8080 \
     --runner-id default-runner \
     --runner-token ${RUNNER_TOKEN}
   ```
   Setelah runner aktif kembali, runner akan mengirim heartbeat pertama dan statusnya di server otomatis kembali menjadi `active`.

---

## 4. Prosedur Melakukan Pemulihan (Explicit Retry)

Setelah runner kembali sehat dan workspace siap, pemulihan dilakukan secara eksplisit:

### Opsi A: Melalui Antarmuka Web (UI Tasks Detail)
1. Buka antarmuka Cuan pada browser di menu **Tasks Board**.
2. Klik task terkait untuk membuka **Task Detail View**.
3. Di bagian panel attempt, Anda akan melihat Attempt terakhir berstatus `interrupted`.
4. Klik tombol **Retry Task** (berwarna biru).
5. Sistem akan:
   - Mempertahankan attempt lama (`Attempt #1`) dengan status `interrupted` untuk audit trail lengkap.
   - Membuat attempt baru (`Attempt #2`) berstatus **`queued`**.
   - Runner yang aktif akan langsung mengklaim Attempt #2 tersebut secara eksklusif.

### Opsi B: Melalui REST API
Kirim request POST ke endpoint retry task:
```bash
curl -X POST \
  -H "Authorization: Bearer <OWNER_TOKEN>" \
  http://localhost:8080/api/v1/tasks/{task_id}/retry
```
**Contoh Respons:**
```json
{
  "task": {
    "id": "task_abc123",
    "status": "doing",
    "attempt_id": "att_def456",
    "attempt_status": "queued"
  },
  "attempt": {
    "id": "att_def456",
    "task_id": "task_abc123",
    "runner_id": "",
    "attempt_number": 2,
    "status": "queued",
    "started_at": null,
    "completed_at": null
  }
}
```

---

## 5. Penanganan Task Macet (Cancel Task & Process Tree Termination)

Jika sebuah task mengalami loop tak terhingga, timeout, atau hang pada script agent:
1. Klik tombol **Cancel Task** di UI atau kirim request:
   ```bash
   curl -X POST \
     -H "Authorization: Bearer <OWNER_TOKEN>" \
     http://localhost:8080/api/v1/tasks/{task_id}/cancel
   ```
2. Cuan Server akan menandai flag `cancel_requested: true` dan status attempt menjadi `canceled`.
3. Runner daemon mendeteksi status cancel saat eksekusi dan memanggil `KillProcessTree(process.Pid)`:
   - Mengirim sinyal `SIGKILL` ke seluruh process group (`-PID`).
   - Memastikan tidak ada proses zombie atau background subprocess (seperti `sleep`, child script, compiler) yang tertinggal mengonsumsi CPU/RAM.

---

## 6. Audit Trail & Verifikasi Integritas
- Seluruh riwayat attempt (`Attempt #1 [interrupted]`, `Attempt #2 [succeeded]`) tersimpan permanen pada tabel `task_attempts`.
- Seluruh histori transisi dan catatan audit tersimpan pada tabel `task_activities`.
- Operator dan PO dapat meninjau histori tersebut kapan saja pada timeline detail task untuk evaluasi Sprint Review.
