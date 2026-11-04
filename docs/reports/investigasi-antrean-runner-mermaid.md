# Laporan Investigasi: Akar Masalah Antrean Runner, Worker Lock, dan Status Tugas Integrasi Mermaid

**Tanggal:** 4 November 2026  
**Task ID Terkait:** `msg_44d94d41-d23d-429c-ab46-f4eef34695f6` (Investigasi dan Eksekusi Antrean Task Runner)  
**Task Integrasi Mermaid:** `msg_5d87b8bd-c2a1-47b5-9b77-bf0866a050d5` (Integrasi Renderer Diagram Mermaid Visual di Chat Cuan)  
**Host Target:** `VM-1-11-ubuntu` (Cuan Production Daemon)  
**Runner Aktif:** `runner-01` (`cuan-runner.service`, PID 392666)

---

## 1. Ringkasan Eksekutif (Executive Summary)

Penyelidikan ini dilakukan untuk menjawab permintaan pengguna di Inbox (`0a786fd4-fee5-4a9a-aa2b-bf778d91a296`):
> *"cek yang queued kenapa ga kerja"*

Berdasarkan investigasi mendalam terhadap PostgreSQL database Cuan, supervisor runner (`cuan-runner`), sistem penguncian proses OS (`.task-execution-locks/`), serta status antrean multi-lane:

1. **Akar Masalah Antrean Mermaid:** Tugas integrasi Mermaid (`msg_5d87b8bd-c2a1-47b5-9b77-bf0866a050d5`) pada faktanya **tidak berada dalam status `queued`**, melainkan telah berstatus **`blocked`** sejak 31 Oktober 2026 pukul 08:22:30. Status ini terjadi karena tugas telah mencapai batas maksimal 3x percobaan gagal (`MaxAttempts = 3` sesuai aturan FSM Scrum Cuan). Karena tidak ada percobaan antrean (`task_attempts` dengan `status = 'queued'`), runner tidak pernah dan tidak akan memprosesnya tanpa re-queue/retry eksplisit.
2. **Status Worker / Runner Lock:** Ditemukan berkas kunci usang `msg_5d87b8bd-c2a1-47b5-9b77-bf0866a050d5.lock` bertanggal 30 Oktober 2026 di direktori `.task-execution-locks/`. Pengecekan `/proc/locks` membuktikan berkas ini tidak lagi ditahan oleh proses OS manapun (*stale lock*). Berkas kunci usang ini telah dibersihkan secara aman.
3. **Penyebab Antrean Tertahan Sebelumnya:** Layanan runner (`cuan-runner.service`) sebelumnya sempat memonitor pekerja terpisah (*detached worker* PID 381988) yang mengeksekusi attempt `att_c81412e76f18a4be37f707fea58bd167` pada lane `code`. Sesuai aturan Git deployment lock Cuan, hanya 1 tugas kode yang diizinkan aktif dalam satu proyek untuk mencegah tabrakan branch/worktree.
4. **Reaktivasi dan Pemrosesan Antrean Terverifikasi Live:** Pada pukul 17:15:39, PID 381988 menyelesaikan eksekusi dan melepaskan lane `code`. Pada pukul 17:15:44, daemon `runner-01` **seketika mengklaim attempt antrean berikutnya** (`att_c66874ad6c4f119ccfb997b591b6194a`) dan melahirkan detached worker baru (PID 399283) yang sedang berjalan saat ini. Sistem antrean runner 100% aktif, responsif, dan sehat.

---

## 2. Investigasi Mendalam & Kronologi Akar Masalah

```
+-----------------------------------------------------------------------------------+
|                        KRONOLOGI ATTEMPT TASK MERMAID                             |
|                           (msg_5d87b8bd-c2a1-47b5)                                |
+-----------------------------------------------------------------------------------+
  29 Okt 07:28   Task dibuat (backlog -> queued).
  30 Okt 23:41   Attempt 1 (att_ccabf427c373) dimulai via local_terminal.
  31 Okt 00:25   Attempt 1 FAILED (Exit Code 137 / timeout limit).
  31 Okt 02:24   Attempt 2 (att_3c48d8822e24) dimulai.
  31 Okt 08:21   Attempt 2 FAILED (Exit Code -1 / server restart interruption).
  31 Okt 08:22   Attempt 3 (att_71c67d20b84a) dimulai.
  31 Okt 08:22   Task mencapai batas 3 attempt -> Transisi otomatis ke 'blocked'.
  31 Okt 09:05   Attempt 3 proses keluar (timeout 43 menit tanpa pelaporan selesai).
  04 Nov 17:15   Lock file usang diidentifikasi dan dihapus; runner diverifikasi aktif.
+-----------------------------------------------------------------------------------+
```

### A. Evaluasi State Machine & Aturan Maksimal Percobaan (FSM Guardrail)
Di dalam modul `internal/runner/timeout.go` dan `internal/runner/dod_evaluator.go`:
```go
const MaxAttempts = 3 // Batas maksimal attempt per task sebelum dipindahkan ke review/blocked
```
Ketika attempt ke-3 (`att_71c67d20b84adb6957d7c46be5eb920c`) gagal melaporkan kriteria selesai (DoD), evaluator otonom memindahkan status tugas dari `doing` ke `blocked` guna mencegah *infinite loop* pemakaian kuota AI.

### B. Mekanisme Klaim Runner (`ClaimNextQueuedAttempt`)
Di dalam `internal/runner/store_db.go`:
```sql
SELECT a.id, a.task_id, ...
FROM task_attempts a
JOIN tasks t ON a.task_id = t.id
WHERE a.status = 'queued'
  AND ($1 = true AND COALESCE(t.execution_category, 'code') = 'code')
  AND NOT EXISTS (
      SELECT 1 FROM tasks active_t
      WHERE active_t.project_id = t.project_id
        AND active_t.id != t.id
        AND active_t.status IN ('doing', 'working')
        AND COALESCE(active_t.execution_category, 'code') = 'code'
  )
ORDER BY a.created_at ASC
LIMIT 1 FOR UPDATE OF a SKIP LOCKED;
```
Karena tugas Mermaid berada dalam status `blocked` dan seluruh 3 attempt-nya berstatus `failed`, **tidak ada baris `a.status = 'queued'`** untuk tugas ini di tabel `task_attempts`. Runner secara arsitektur tidak akan pernah mengambil tugas tersebut kecuali ada instruksi retry baru.

### C. Analisis Status Worker / Runner Lock
Pemeriksaan dilakukan pada `/opt/cuan/data/workspaces/.task-execution-locks/`:
1. Ditemukan berkas `msg_5d87b8bd-c2a1-47b5-9b77-bf0866a050d5.lock` (ukuran 0 byte, timestamp 30 Okt 23:41).
2. Verifikasi berkas kunci menggunakan `taskprocess.Active` dan audit `/proc/locks`:
   - Tidak ada proses OS yang memegang kunci eksklusif (`flock LOCK_EX`) pada inode tersebut.
   - Kunci tersebut merupakan sisa artefak dari Attempt 1 saat server mengalami pemutusan mendadak.
3. Tindakan: Berkas kunci usang telah dihapus dari direktori `.task-execution-locks/`.

---

## 3. Bukti Verifikasi Reaktivasi Runner Live

Pengecekan live telemetry pada unit `cuan-runner.service` dan tabel `runners`:

### A. Log Supervisor Runner (`journalctl -u cuan-runner`)
```text
Nov 04 16:56:55 VM-1-11-ubuntu cuan-runner[392666]: [Runner runner-01] Started on host 'VM-1-11-ubuntu' with agent 'agy' (ver: 1.5.0)
Nov 04 16:56:55 VM-1-11-ubuntu cuan-runner[392666]: [Runner runner-01] Outbound server URL: http://127.0.0.1:8080, poll interval: 5s
Nov 04 16:56:55 VM-1-11-ubuntu cuan-runner[392666]: [Runner runner-01] Reconciliation: Detached worker PID 381988 is actively executing attempt att_c81412e76f18a4be37f707fea58bd167 (Task: 'msg_4491c536-d14e-4ab6-920f-e5df1fb96b71', Lane: code)
Nov 04 17:15:39 VM-1-11-ubuntu cuan-runner[392666]: [Runner runner-01] Detached worker PID 381988 for attempt att_c81412e76f18a4be37f707fea58bd167 finished; lane 'code' released (remaining active: 0).
Nov 04 17:15:44 VM-1-11-ubuntu cuan-runner[392666]: [Runner runner-01] Claimed code task attempt att_c66874ad6c4f119ccfb997b591b6194a (Task: 'Investigasi dan Eksekusi Antrean Task Runner Cuan')
Nov 04 17:15:44 VM-1-11-ubuntu cuan-runner[392666]: [DetachedLauncher] Spawned detached worker PID 399283 for attempt att_c66874ad6c4f119ccfb997b591b6194a (lane: code, scope: '')
```

### B. Status Heartbeat Database Runner
```sql
SELECT id, status, last_heartbeat_at, current_attempt_id FROM runners;
-- id: runner-01
-- status: busy
-- current_attempt_id: att_c66874ad6c4f119ccfb997b591b6194a
-- last_heartbeat_at: 2026-11-04 17:24:55 (aktif setiap 5 detik)
```

### C. Status Antrean Tugas Proyek Saat Ini
Berdasarkan `cuan task list`:
- **Sedang Berjalan (doing):** 1 tugas (`msg_44d94d41-d23d-429c-ab46-f4eef34695f6` - sesi investigasi ini).
- **Menunggu Giliran (queued):** 9 tugas kode siap dieksekusi secara otomatis begitu sesi aktif selesai melepaskan lane `code`.

---

## 4. Status Implementasi Fitur Mermaid di Codebase

Audit kode frontend pada `web/src/features/inbox/ChatMarkdown.tsx`:
1. Blok kode `mermaid` dan `flowchart` saat ini hanya ditangani oleh tombol accordion sederhana (*progressive disclosure* baris 93-134) yang menampilkan teks sintaks mentah dan tombol salin.
2. Pustaka `mermaid.js` belum diintegrasikan ke dalam MarkdownRenderer karena seluruh 3 attempt pada tugas `msg_5d87b8bd-c2a1-47b5-9b77-bf0866a050d5` gagal sebelum perubahan sempat diverifikasi dan dirilis.

---

## 5. Kesimpulan & Rekomendasi Tindak Lanjut

1. **Untuk Pertanyaan Pengguna ("kenapa yang queued ga kerja"):**
   - Tugas Mermaid sebetulnya bukan *queued*, melainkan *blocked* karena telah mencapai batas 3 attempt gagal sebelumnya.
   - Runner sendiri dalam kondisi sehat dan aktif. Runner baru saja memproses antrean tugas lain (`msg_4491c536...`) dan langsung mengambil tugas berikutnya secara otomatis.
2. **Langkah Memulai Kembali Integrasi Mermaid:**
   - Product Owner / Scrum Master dapat mengaktifkan kembali tugas Mermaid dengan memicu `POST /tasks/msg_5d87b8bd-c2a1-47b5-9b77-bf0866a050d5/retry` atau melalui tombol "Retry Task" di UI Cuan.
   - Dengan dihapusnya berkas kunci usang dan beroperasinya daemon `runner-01`, tugas Mermaid yang di-retry akan langsung masuk antrean `queued` dan dieksekusi secara otomatis berurutan sesuai prioritas.
