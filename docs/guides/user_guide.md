# Cuan User & Operational Guide

Panduan lengkap penggunaan Cuan bagi Product Owner dan tim pengembang. Dokumen ini menjelaskan alur kerja harian, siklus hidup task, peninjauan Pull Request, mekanisme revisi terpadu, dan manajemen runner agent.

---

## 1. Konsep Inti (*Core Concepts*)

```d2
direction: right

project: Project Repository {
  label: "Connected GitHub Repo\n(buchin/cuan)"
}

task: Task Item {
  label: "Task\n(Title & Instructions)"
}

runner: Runner Daemon {
  label: "cuan-runner\n(Claims attempt)"
}

workspace: Isolated Workspace {
  label: "Task Workspace\n(task/<id> branch)"
}

pr: GitHub Pull Request {
  label: "Pull Request\n(Formatted Diff & Tests)"
}

project -> task: "has many"
task -> runner: "assigned & claimed"
runner -> workspace: "executes agent"
workspace -> pr: "commits & pushes"
```

1. **Project**: Entitas repositori GitHub yang dihubungkan ke Cuan. Menyimpan konfigurasi branch default (`main`) dan kredensial akses terenkripsi.
2. **Task**: Satuan pekerjaan atau fitur yang ditentukan oleh owner. Memiliki instruksi teknis, prioritas, status (`backlog`, `doing`, `review`, `done`), serta branch task terisolasi.
3. **Attempt**: Siklus eksekusi fisik dari sebuah task oleh runner agent. Memiliki status tersendiri (`queued`, `running`, `succeeded`, `failed`, `canceled`, `interrupted`) sehingga kegagalan satu attempt tidak merusak riwayat task.
4. **Task Workspace**: Direktori klon terisolasi per task (`data/workspaces/<task_id>`). Mencegah kontaminasi antar-task dan melindungi repositori utama / instalasi server dari modifikasi sembarangan.
5. **Human Author Identity**: Seluruh commit Git yang dihasilkan oleh agent menggunakan identitas pengembang manusia yang terkonfigurasi (`Buchin <buchin@dropsugar.com>`), tanpa menggunakan identitas bot sintetis.

---

## 2. Siklus Hidup Task (*Task State Transitions*)

Cuan memisahkan status konseptual task dari status operasional attempt:

### A. Status Task:
- **`backlog`**: Task baru dibuat, belum ada runner yang mengeksekusi.
- **`doing`**: Sedang dalam proses eksekusi aktif oleh runner agent.
- **`review`**: Agent telah menyelesaikan pekerjaan, targeted test lulus (exit code 0), commit telah dibuat, dan PR di GitHub telah terbuka. Menunggu review owner.
- **`done`**: Owner menerima hasil kerja agent dan memverifikasi penggabungan ke branch utama.

### B. Status Attempt (Operasional):
- **`queued`**: Menunggu giliran runner yang cocok dan tidak sibuk.
- **`running`**: Runner sedang menjalankan subprocess agent dalam workspace task.
- **`succeeded`**: Agent selesai dengan exit code 0 dan verifikasi targeted test lulus.
- **`failed`**: Agent menghasilkan error atau exit code bukan nol. Detail error tercatat di log attempt.
- **`canceled`**: Owner menekan tombol batalkan; runner menghentikan process tree agent seketika melalui sinyal `SIGTERM`/`SIGKILL`.
- **`interrupted`**: Runner kehilangan koneksi jaringan atau mati mendadak saat tugas berjalan. Attempt ditandai terputus secara aman tanpa auto-retry liar yang berisiko duplikasi commit.

---

## 3. Meninjau Hasil Kerja Agent (Review Phase)

Ketika task berpindah ke status **`review`**, antarmuka Cuan menampilkan panel observabilitas lengkap pada Task Detail:
1. **GitHub Pull Request URL**: Tautan langsung menuju PR resmi di repositori GitHub (contoh: `https://github.com/buchin/cuan/pull/1`).
2. **Commit SHA & Author**: Checkpoint commit Git yang dihasilkan beserta identitas human committer.
3. **Branch Task**: Nama branch kerja (`task/<task_id>`).
4. **Diff Summary**: Ringkasan perubahan berkas (`git diff --stat`) yang menunjukkan berkas baru, berkas terubah, dan baris kode.
5. **Targeted Test Status**: Status pengujian otomatis beserta exit code (contoh: `Targeted validation executed (exit 0)`).
6. **Execution Logs**: Log streaming terstruktur dari stdout dan stderr agent dengan sensor rahasia otomatis (`[REDACTED]`).

---

## 4. Siklus Revisi Terpadu (*Revision Cycle: Review -> Doing*)

Jika hasil kerja agent memerlukan perbaikan tambahan atau ada instruksi review dari owner:

1. Buka task yang berstatus `review`.
2. Klik tombol **Minta Revisi** (atau ubah status kembali ke **`doing`**).
3. Masukkan arahan revisi konkret pada catatan perbaikan (contoh: *"Perbaiki validasi email dan tambahkan unit test untuk domain invalid"*).
4. **Apa yang Terjadi di Balik Layar?**
   - Task kembali ke status `doing` dan attempt baru (Attempt #2) dijadwalkan.
   - Runner mengklaim attempt revisi dan **melanjutkan workspace yang sama** (`data/workspaces/<task_id>`) pada branch yang sama.
   - Perubahan sebelumnya dipertahankan (*zero work loss*).
   - Agent menerapkan perbaikan revisi dan membuat commit baru.
   - Runner melakukan push ke branch task yang sama.
   - **Zero Duplicate PR**: Alih-alih membuat PR baru, Cuan memperbarui PR GitHub yang sudah ada dengan checkpoint revisi terbaru.
   - Task kembali ke status `review` dengan histori attempt yang lengkap dan dapat dilacak.

---

## 5. Pemantauan Server & Resource Agent

Pada tab **Servers & Runners**:
- **Host Metrics**: Menampilkan utilisasi CPU, persentase RAM, dan sisa kapasitas SSD.
- **Agent Leases**: Runner memperbarui sewa (*lease*) aktif setiap 3 detik. Jika runner terputus selama lebih dari 30 detik, lease kedaluwarsa dan status attempt dicatat secara transparan.
- **Concurrency Guard**: Cuan menerapkan batas default 1 task aktif per runner untuk mencegah kehabisan memori (*OOM Killer*) pada VPS kecil.
