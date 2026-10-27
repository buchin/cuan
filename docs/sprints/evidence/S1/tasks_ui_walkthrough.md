# Walkthrough UI Task Board & Histori Pekerjaan (Sprint 1 — T4)

Dokumen ini mendokumentasikan fitur antarmuka (UI) Task Board, pemisahan status task dan status attempt eksekusi, serta histori audit transisi yang diimplementasikan pada task T4 sesuai [S1.md](../../../docs/sprints/S1.md).

---

## 1. Navigasi & Tampilan Task Board

Pada tab navigasi utama Cuan, menu **Tasks Board** menyediakan dua mode visualisasi:
1. **Mode Kanban Board**:
   - Kolom berurutan: **Backlog**, **Doing**, **Review**, **Done**.
   - Setiap kartu menampilkan: ID task, judul, badge project GitHub, prioritas (`low`, `normal`, `high`, `urgent`), agent yang ditugaskan (`🤖 agy`), serta badge status attempt eksekusi terbaru (`queued`, `running`, `succeeded`, `failed`).
   - Tombol aksi cepat untuk memindahkan status antar kolom secara instan (contoh: dari *Backlog* ke *Doing*, atau dari *Review* ke *Done*).
2. **Mode List Table**:
   - Menampilkan tabel komprehensif berisi ID, Judul, Project, Status Task, Status Attempt, Prioritas, Agent, dan tombol aksi **Detail**.
3. **Filter & Pencarian Real-Time**:
   - Filter dropdown per project GitHub yang terhubung.
   - Kolom pencarian instan (search) berdasarkan kata kunci judul atau instruksi task.
4. **Tombol "Buat Task Baru"**:
   - Membuka modal dialog interaktif untuk mendefinisikan task baru.

---

## 2. Modal Dialog: Pembuatan Task Baru

Owner dapat membuat task baru dengan atribut:
- **Judul Task** (Wajib): Nama ringkas pekerjaan yang akan dikerjakan agent.
- **Project Terkait** (Opsional): Menghubungkan task dengan repository GitHub yang telah dihubungkan pada T3 (contoh: `buchin/cuan`).
- **Instruksi / Scope**: Detail teks instruksi atau scope pekerjaan yang akan dieksekusi oleh coding agent.
- **Prioritas**: Pilihan tingkat kepentingan (`low`, `normal`, `high`, `urgent`).
- **Assigned Agent**: Pilihan adapter agent (default: `agy` - Antigravity Adapter yang lolos spike T1).

Setelah disimpan:
- Task otomatis tersimpan secara persisten pada PostgreSQL.
- Histori awal `task_created` dicatat ke dalam tabel audit `task_activities`.
- Task langsung muncul pada kolom **Backlog**.

---

## 3. Tampilan Detail Task (Task Detail View)

Ketika owner mengklik kartu task pada Kanban atau tombol *Detail* pada tabel List:
1. **Header Detail Task**:
   - Menampilkan Judul Task, ID unik (`task_xxx`), Project GitHub terkait, dan tanggal pembuatan.
   - **Pemisahan Status Ganda (Dual Status Badges)**:
     - Badge **Status Task**: Menunjukkan status bisnis/sprint (`backlog`, `doing`, `review`, `done`, `blocked`).
     - Badge **Status Attempt**: Menunjukkan status proses eksekusi runner (`queued`, `running`, `succeeded`, `failed`, `canceled`, `interrupted`).
2. **Peringatan Kegagalan Attempt (Failure Guardrail Alert)**:
   - Apabila attempt terakhir mengalami kegagalan (`status == failed` atau `exit_code != 0`), banner merah mencolok akan muncul:
     > **Kegagalan Attempt Eksekusi Terdeteksi:**
     > Attempt `att_xxx` mengalami kegagalan dengan exit code `1`.
     > Sesuai kriteria DoD: Kegagalan attempt TIDAK otomatis dianggap sukses atau di-mark done. Task tetap bertahan dalam status kerja untuk perbaikan.
3. **Formulir Edit Atribut & Status**:
   - Owner dapat memperbarui Judul, Instruksi, Status, Prioritas, dan Assigned Agent.
   - Setiap perubahan status task akan langsung dicatat sebagai entri `status_change` pada audit log (merekam `old_status` dan `new_status`).
4. **Kontrol Runner & Attempt Eksekusi**:
   - Tombol **Trigger Attempt Baru**: Mendaftarkan attempt baru berstatus `queued` untuk runner agent.
   - Tombol simulasi status (`Set Running`, `Set Succeeded`, `Set Failed`) untuk verifikasi dan observasi transisi live.
   - Tabel riwayat semua attempt yang pernah dijalankan untuk task tersebut.
5. **Histori Transisi & Catatan Kerja (Audit Timeline)**:
   - Garis waktu kronologis mencatat:
     - Pembuatan task (`task_created`).
     - Perubahan status task (`status_change`) lengkap dengan indikator `old_status → new_status`.
     - Transisi attempt eksekusi (`attempt_transition`).
     - Catatan kerja dan komentar manual yang ditambahkan oleh owner (`comment`).
   - Kotak formulir penulisan catatan kerja / komentar baru di bagian bawah timeline.

---

## 4. Jaminan Ketahanan Restart (Server Persistence)

Sesuai kriteria DoD:
- Seluruh data task, relasi project, status attempt, dan entri histori aktivitas tersimpan di PostgreSQL volume.
- Restart server atau reboot container tidak menyebabkan data hilang.
- Bukti pengujian restart otomatis divalidasi oleh targeted suite `internal/server/verify_t4_test.go` (lulus 100%).

---

## 5. Ringkasan Endpoint API Terkait T4

| Method | Endpoint | Deskripsi |
|---|---|---|
| `GET` | `/api/v1/tasks` | Mengambil daftar task terproteksi (mendukung filter `project_id`, `status`, `search`) |
| `POST` | `/api/v1/tasks` | Membuat task baru dengan status awal `backlog` |
| `GET` | `/api/v1/tasks/{id}` | Mengambil detail lengkap task beserta histori aktivitas dan riwayat attempt |
| `PUT` | `/api/v1/tasks/{id}` | Memperbarui atribut dan/atau status task (otomatis merekam `status_change`) |
| `DELETE` | `/api/v1/tasks/{id}` | Menghapus task beserta riwayat attempt dan audit log secara bersih |
| `POST` | `/api/v1/tasks/{id}/comments` | Menambahkan catatan kerja / komentar pada task |
| `POST` | `/api/v1/tasks/{id}/attempts` | Mendaftarkan attempt eksekusi baru |
| `PATCH` | `/api/v1/tasks/{id}/attempts/{attemptId}` | Memperbarui status attempt (menegakkan aturan kegagalan attempt) |
