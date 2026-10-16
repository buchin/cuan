# Task Detail UI Walkthrough — Perubahan Git & Pull Request Observability (T6)

## 1. Ringkasan Fitur UI T6

Pada implementasi T6, antarmuka Task Detail dilengkapi dengan modul observabilitas Git dan GitHub Pull Request yang dirancang untuk memberikan transparansi penuh kepada Owner tanpa mengorbankan kesederhanaan penggunaan (user-centric).

Komponen utama:
1. **Header Action Controls untuk Status Review:**
   - Tombol **"Minta Revisi"** (berwarna amber dengan ikon RotateCcw): Membuka dialog cepat bagi PO/Owner untuk memasukkan arahan revisi. Mengembalikan status task ke `doing` dan secara otomatis menjadwalkan attempt revisi baru di antrean runner.
   - Tombol **"Approve & Done"** (berwarna emerald dengan ikon Check): Memungkinkan Owner menyetujui hasil kerja dan PR setelah diverifikasi, menandai status task menjadi `done`.
2. **Card Perubahan Git & GitHub Pull Request (T6):**
   - **Task Branch:** Menampilkan nama branch kerja terisolasi (misal `task/{task_id}`) disertai label proteksi branch default `✓ Default branch terlindungi`.
   - **GitHub PR Link:** Tombol/tautan langsung menuju GitHub Pull Request nyata (misal `PR #42`) dengan ikon eksternal yang membuka PR di tab baru.
   - **Commit SHA:** Menampilkan 7 karakter pertama dari commit hash terbaru yang dihasilkan oleh agent, dengan catatan author terverifikasi `Author: Buchin`.
   - **Status Review:** Menampilkan status terkini (`Menunggu Review PO`) dengan indikator kesiapan tindakan.
   - **Targeted Test Validation:** Menampilkan badge hasil validasi test targeted beserta exit status (contoh: `Targeted validation executed (exit 0)`).
   - **Ringkasan Diff Perubahan:** Blok tampilan kode (`pre`) dengan tema dark mode yang menampilkan keluaran `git diff --stat`, merinci file yang berubah, jumlah baris penambahan (`+`), dan pengurangan (`-`).

---

## 2. Struktur Visual Antarmuka

```
+---------------------------------------------------------------------------------------+
|  <- Kembali ke Task Board               [Minta Revisi]  [Approve & Done] [Hapus Task] |
+---------------------------------------------------------------------------------------+
|  Judul Task: S01.6 — Hasilkan perubahan Git dan PR        [doing / review / done]     |
|  Project: cuan | Dibuat: 2026-10-16                       Attempt: att_xxx (succeeded)|
+---------------------------------------------------------------------------------------+
|                                                                                       |
|  [ICON] Perubahan Git & GitHub Pull Request (T6)                                      |
|  Branch terisolasi per task, proteksi branch default, diff summary, dan PR GitHub.    |
|                                                                                       |
|  +--------------------+ +--------------------+ +--------------------+ +---------------+
|  | TASK BRANCH        | | GITHUB PR          | | COMMIT SHA         | | STATUS REVIEW |
|  | task/lzzofiyzsc... | | PR #42 [LINK]      | | a83f2e1            | | Menunggu      |
|  | ✓ Default terlindung| | Auto-update revisi | | Author: Buchin     | | Review PO     |
|  +--------------------+ +--------------------+ +--------------------+ +---------------+
|                                                                                       |
|  [✓] Targeted automated tests passed (exit code 0)                                    |
|                                                                                       |
|  Ringkasan Diff Perubahan:                                                            |
|  +---------------------------------------------------------------------------------+  |
|  | internal/git/workspace.go  | 250 ++++++++++++++++++++++++++++++                 |  |
|  | internal/git/pr.go         | 230 +++++++++++++++++++++++++                      |  |
|  | web/src/App.tsx            | 140 ++++++++++++++                                 |  |
|  | 3 files changed, 620 insertions(+)                                              |  |
|  +---------------------------------------------------------------------------------+  |
+---------------------------------------------------------------------------------------+
```

---

## 3. Alur Pengujian Siklus Revisi melalui UI

1. **Owner Menginspeksi Task dalam Status Review:**
   - Owner membuka Task Detail dan melihat ringkasan diff serta tautan PR GitHub.
2. **Owner Meminta Revisi:**
   - Mengklik tombol **"Minta Revisi"**.
   - Memasukkan arahan revisi: *"Perbaiki catatan verifikasi dan tambahkan helper."*
   - Sistem otomatis:
     - Mengubah status task menjadi `doing`.
     - Mencatat activity timeline `[status_change]` dengan pesan arahan revisi.
     - Menjadwalkan `TaskAttempt` baru dengan status `queued`.
3. **Runner Mengeksekusi Revisi:**
   - Runner mengambil attempt baru, membuka workspace yang sama di direktori terisolasi, menerapkan perubahan, membuat commit baru, dan mem-push ke branch yang sama.
   - PR generator memperbarui PR yang sudah ada di GitHub tanpa membuat PR duplikat.
4. **Owner Menerima Hasil Revisi & Melakukan Approval:**
   - Task kembali ke status `review` dengan diff dan commit terbaru.
   - Owner mengklik **"Approve & Done"**, menandai task selesai dengan tuntas.
