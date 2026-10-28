# Laporan Investigasi: Akar Masalah Task Mengantri di Prod dan Remediasi

**Tanggal:** 28 Oktober 2026  
**Status Produksi:** Healthy (`1ff8134a7e61acaa02a66b7c86e3f5ef2659d705`)  
**Target Host:** `https://cuan.dojo.cc`

---

## 1. Pertanyaan / Isu Utama
> *"Kenapa di prod task tidak dimulai padahal tidak ada yang aktif. Malah statusnya mengantri?"*

---

## 2. Investigasi & Akar Masalah (Root Cause)

Berdasarkan penelusuran mendalam terhadap database PostgreSQL produksi (`cuan`), log layanan server (`cuan-server`), dan alur eksekusi runner (`internal/task/launch.go` & `internal/server/terminal_handlers.go`), ditemukan **dua faktor utama** yang menyebabkan task berstatus mengantri (*queued*):

```d2
direction: right

client: "Browser / User" {
  shape: rectangle
}

server: "Cuan Server (cuan.dojo.cc)" {
  concurrency_check: "isProjectActive() Check"
  launch_queue: "Task Launch Engine"
}

db: "PostgreSQL Database" {
  task_launches: "table: task_launches"
  tasks: "table: tasks"
}

client -> server.concurrency_check: "Trigger Run Task"
server.concurrency_check -> db.task_launches: "Cek active launches (state=started)"
db.task_launches -> server.concurrency_check: "Orphan Lock Terdeteksi!"
server.concurrency_check -> db.tasks: "Set status = 'queued'"
server.concurrency_check -> client: "409 Project Concurrency Busy"
```

### A. Orphan Task Launch Lock (Backend Concurrency Bug)
1. **Mekanisme Kunci Concurrency Project:** Untuk mencegah konflik Git worktree, Cuan memberlakukan batas konkurensi 1 proses per project melalui fungsi `isProjectActive()` dan `ClaimLaunch()`.
2. **Kondisi Orphan Lock:** Ketika sebuah sesi task sebelumnya terputus tidak normal (misalnya lease attempt kedaluwarsa, crash runner, atau status task diubah secara eksternal ke `done`/`review`), baris pada tabel `task_launches` tertinggal dalam `state = 'started'` atau `state = 'claimed'` dengan `lease_token` aktif.
3. **Dampak:** Meskipun runner fisik sudah berhenti dan tidak ada agent yang berjalan, query `ClaimLaunch()` dan `isProjectActive()` mendeteksi masih ada proses `started`. Akibatnya, setiap task baru yang masuk ke project tersebut ditolak konkurensinya dan dibiarkan berstatus `queued` (*mengantri*) secara permanen.

### B. Interactive Terminal Task vs Background Task
1. Pada fitur Terminal (`ProjectsView.tsx` & `ProjectsWorkspace.tsx`), mengklik tombol Terminal membuat task interaktif berjudul `Terminal · <project>`.
2. Jika saat tombol diklik sedang ada task lain yang berjalan, backend mengembalikan `409 Conflict (project_concurrency_busy)` dan menandai task tersebut sebagai `queued`.
3. Namun, interactive terminal session membutuhkan koneksi WebSocket browser pengguna secara langsung (bukan background worker). Ketika task yang menghalangi selesai, terminal task yang ditinggalkan pengguna tetap berada di database dengan status `queued`. Di database produksi ditemukan 3 task terminal yang tertinggal dalam status ini (`task_24a5d7eabbba`, `task_b24d700a6ef7`, dan `task_65d407d6a301`).

### C. Penolakan Tool `search_web` & MCP pada Chat Scrum Master
Pada pengujian ruang diskusi, chat agent (`@sm`) mengalami penolakan eksekusi saat memanggil `search_web` dan tool MCP inspeksi platform karena belum terdaftar dalam whitelist permission CLI (`antigravity_settings.go`) dan validator `isSafeInboxTool` (`inbox_runtime.go`).

---

## 3. Langkah Remediasi & Implementasi Kode

### A. Perbaikan Kode Permanen
1. **Pencegahan & Rekonsiliasi Orphan Launch (`3dc8444`):**
   - Menambahkan pembersihan otomatis tabel `task_launches` (`state = 'stopped'`) saat transisi status task ke `done`, `review`, `blocked`, `cancelled`, atau `failed`.
   - Mengimplementasikan `ReconcileStartupLaunches` yang otomatis merapikan semua orphan launch yang tertinggal saat server booting.
   - Memodifikasi `ClaimLaunch()` agar memverifikasi apakah task yang mengunci sudah selesai; jika sudah, launch usang langsung direkonsiliasi sehingga task antrean baru dapat langsung diklaim.
2. **Optimasi Workspace Retention & Resource Guard (`8e55169`, `d68ebac`):**
   - Mendukung immediate retirement untuk task yang dibatalkan dan retensi instan untuk task selesai guna mencegah penumpukan worktree.
3. **Penyatuan Card UI & Event Isolation (`29f1b8a`, `0de78f4a5`):**
   - Mengintegrasikan touch target WCAG (min 44px) dan penanganan status Queued yang akurat pada `TaskCard`.
4. **Whitelist Tool `search_web` & Platform MCP (`58ed5ff`, `1ff8134`):**
   - Mendaftarkan `search_web`, `call_mcp_tool`, `inspect_database_query`, `inspect_database_schema`, dan `inspect_app_logs` ke whitelist read-only runtime.
   - Memperbaiki parsing regex durasi cooldown kuota provider (`resetsInRegex`).

### B. Remediasi Database & Sistem Produksi
1. **Unblock Orphan Launches:** Menjalankan hotfix SQL pada database produksi untuk membersihkan baris `task_launches` yang tertinggal dari sesi sebelumnya.
2. **Pembersihan Terminal Task Usang:** Mengubah status 3 terminal task antrean usang yang ditinggalkan (`task_65d407d6a301`, `task_24a5d7eabbba`, `task_b24d700a6ef7`) menjadi `cancelled`.
3. **Disk Pruning:** Melakukan safe pruning pada 20 worktree usang di VPS, menghemat 2.5GB disk sehingga kapasitas aman di 73% (14GB tersedia).

---

## 4. Bukti Verifikasi Produksi

1. **Deployment Live:**
   - Commit `1ff8134a7e61acaa02a66b7c86e3f5ef2659d705` aktif pada `/opt/cuan/current`.
   - Endpoint `https://cuan.dojo.cc/healthz` merespons `200 OK` dengan status `healthy`.
2. **Pengecekan Antrean Task Database:**
   ```sql
   SELECT id, title, status FROM tasks WHERE status = 'queued';
   -- Result: 0 rows (tidak ada task mengantri yang macet)
   ```
3. **Pengecekan Active Launches Database:**
   ```sql
   SELECT id, task_id, state FROM task_launches WHERE state IN ('queued', 'claimed', 'started');
   -- Result: 0 rows (tidak ada kunci orphan tersisa)
   ```
4. **Verifikasi Chat Agent Scrum Master:**
   - Chat agent di room `2fd24c9d-d0fe-4f80-a7b7-521c2d9c0a85` berhasil menjawab prompt pengguna ("Apakah tool websearch sudah dienable untuk ente?" dan "Cek repo apakah ada fitur yang belum kita deploy ke prod") dengan memanggil tool inspeksi internal secara mulus tanpa penolakan runtime.
