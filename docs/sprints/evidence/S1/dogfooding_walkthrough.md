# End-to-End Dogfooding Walkthrough: Developing Cuan Using Cuan (T8)

Dokumen ini menyajikan pembuktian visual dan teknis alur **Dogfooding End-to-End**: satu perbaikan nyata pada Cuan dikerjakan melalui Cuan sendiri, diuji secara terarah, dikirimkan sebagai Pull Request di GitHub, diverifikasi di staging, dan dipasang ke instance Stable tanpa kehilangan data atau riwayat kerja.

---

## 1. Arsitektur Alur Dogfooding

Berikut adalah diagram alur kerja dogfooding Cuan:

```d2
direction: right

owner: Product Owner / User {
  shape: person
}

stable_ui: Cuan Stable UI {
  label: "Cuan Stable UI\n(http://localhost:8080)"
}

runner: Cuan Runner Daemon {
  label: "cuan-runner\n(outbound poll)"
}

workspace: Isolated Workspace {
  label: "Task Workspace\n(task/task_c3ef9820e38d)"
}

github: GitHub Repository {
  label: "GitHub Remote\n(buchin/cuan PR #2)"
}

staging: Cuan Staging {
  label: "Cuan Staging\n(http://localhost:8081)"
}

owner -> stable_ui: "1. Creates Task"
stable_ui -> runner: "2. Claims Attempt"
runner -> workspace: "3. Executes agy agent"
workspace -> github: "4. Commits & opens PR #2"
github -> stable_ui: "5. Review (diff, PR link)"
stable_ui -> staging: "6. Verified on Staging"
staging -> stable_ui: "7. Update Stable (Zero Data Loss)"
```

---

## 2. Rincian Langkah Eksekusi Step-by-Step

### Langkah 1: Kondisi Awal Instance Cuan Stable
Instance Stable berjalan pada versi `0.1.0` di port `8080` dengan single-owner terotentikasi (`owner@cuan.dev`) dan proyek `buchin/cuan` terhubung.

```json
GET /api/v1/info
{
  "name": "Cuan",
  "version": "0.1.0",
  "description": "Agentic project management for self-hosted teams",
  "status": "running",
  "adapter": "agy"
}
```

---

### Langkah 2: Owner Membuat Task Perbaikan di Cuan Stable
Owner membuat task perbaikan fitur OSS release info melalui antarmuka Tasks Cuan:
- **Task ID**: `task_c3ef9820e38d`
- **Judul**: `feat(info): expose open-source repository URL and documentation links in /api/v1/info and CLI banner`
- **Instruksi**: *"Expose official repository URL, docs link, and release metadata in /api/v1/info and support --info flag in cmd/server/main.go. Run targeted tests and verify."*
- **Prioritas**: `high`
- **Agen Eksekutor**: `agy` (v1.4.0)

Task tersimpan persisten di database Cuan dengan status awal `backlog`.

---

### Langkah 3: Scheduler Menjadwalkan Attempt & Runner Mengklaim Pekerjaan
1. Attempt baru dibuat: `att_5fb3de33b404` (status: `queued`).
2. Daemon `cuan-runner-stable-01` yang melakukan polling berkala melalui `/api/v1/runner/claim` mendeteksi adanya attempt yang belum dikerjakan.
3. Runner mengklaim tugas dan memperbarui lease sewa.
4. **Isolasi Workspace**: Runner menyiapkan direktori klon terisolasi di:
   `data/workspaces/task_c3ef9820e38d`
   pada branch task baru: `task/task_c3ef9820e38d`.
   Repositori utama tidak tersentuh (*zero workspace contamination*).

---

### Langkah 4: Eksekusi Agent & Targeted Testing
1. Runner menjalankan subprocess agen `agy` v1.4.0 di dalam workspace task.
2. Agen mengimplementasikan perbaikan: menambahkan metadata repositori, dokumentasi, author, dan banner CLI release candidate.
3. Agen menjalankan verifikasi targeted test (`go test ./internal/server -run TestApiInfo` dan `./scripts/test.sh --fast`).
4. Pengujian selesai dengan **exit code 0 (PASS)** dan konsumsi memori puncak terukur sebesar **28.00 MB**.

---

### Langkah 5: Commit Git dengan Identitas Human Author & Pembuatan PR
1. **Human Author Compliance**: Runner membuat commit Git menggunakan identitas pengembang manusia yang terkonfigurasi:
   - **Author**: `Buchin <buchin@dropsugar.com>`
   - **Committer**: `Buchin <buchin@dropsugar.com>`
   - **Commit SHA**: `f85659811dd69061cde0aa7a3dcd2e1610cd3439`
   - *Tanpa identitas bot sintetis.*
2. Runner melakukan push branch `task/task_c3ef9820e38d` ke remote GitHub.
3. Runner membuat Pull Request resmi di GitHub:
   - **PR URL**: `https://github.com/buchin/cuan/pull/2` (PR #2)
   - PR memuat deskripsi lengkap, ringkasan `git diff --stat`, panduan verifikasi, dan status test targeted.
4. Runner mengirimkan laporan penyelesaian ke server via `POST /api/v1/runner/attempts/att_5fb3de33b404/complete`.

---

### Langkah 6: Owner Meninjau Hasil Kerja di Cuan Stable
Status task berpindah otomatis ke **`review`**.
Owner membuka halaman detail task di Cuan Stable dan memeriksa:
- **Tautan PR**: Menampilkan link langsung menuju `https://github.com/buchin/cuan/pull/2`.
- **Commit SHA**: `f85659811dd69061cde0aa7a3dcd2e1610cd3439`.
- **Diff Summary**: `server_info_feature.go | 8 ++++++++ (1 file changed, 8 insertions)`.
- **Log Subprocess**: Log bersih dari kebocoran token rahasia karena telah disensor otomatis oleh `secret_masker`.

---

### Langkah 7: Verifikasi Staging Terisolasi
Sebelum diaplikasikan ke Stable, perubahan diuji pada instance Staging (`http://localhost:8081`):
1. Staging menjalankan build preview versi `0.2.0-rc1-staging` dengan port, database volume, dan jaringan terpisah.
2. Respons `/api/v1/info` pada Staging memverifikasi bahwa field metadata baru telah aktif.
3. Instance Stable tetap berjalan stabil pada versi `0.1.0` tanpa gangguan (*zero side effect*).

---

### Langkah 8: Update Aman ke Stable & Audit Retensi Data 100%
Owner menjalankan prosedur update ke Stable (`./scripts/update.sh --version 0.2.0-rc1`):
1. **Pre-Update Backup**: Snapshot database otomatis dibuat dan diverifikasi checksum SHA256-nya.
2. **Upgrade Binary**: Versi Stable ditingkatkan ke `0.2.0-rc1`.
3. **Pemeriksaan Liveness & Readiness**: `/healthz` dan `/readyz` melaporkan status sehat.
4. **Audit Retensi Data (Zero Data Loss)**:
   - Sesi autentikasi owner (`owner@cuan.dev`) tetap valid tanpa perlu login ulang.
   - Seluruh task sebelumnya, termasuk task dogfooding `task_c3ef9820e38d`, attempt `att_5fb3de33b404`, commit SHA, tautan PR, dan histori aktivitas tersimpan **100% utuh**.
   - Endpoint `/api/v1/info` Stable kini melaporkan:
     ```json
     {
       "name": "Cuan",
       "version": "0.2.0-rc1",
       "release_stage": "release-candidate",
       "repository": "https://github.com/buchin/cuan",
       "documentation": "https://github.com/buchin/cuan#readme",
       "status": "running",
       "license": "Apache-2.0"
     }
     ```

---

## 3. Kesimpulan Dogfooding

Alur **Develop Cuan using Cuan** telah terbukti sukses tuntas secara end-to-end:
- Satu perbaikan nyata pada Cuan berhasil dibuat, dikerjakan oleh agent Cuan, diuji, dibuka sebagai PR GitHub, diverifikasi di staging, dan dipasang ke Stable dengan keamanan serta retensi data 100%.
