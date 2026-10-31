# Panduan Operasional: Agentic Scrum Command Center & Conversational Inbox

Dokumen ini adalah panduan arsitektur dan operasional resmi untuk navigasi **shadcn `sidebar-09`** dan **Conversational Inbox** pada Cuan (Sprint 17).

---

## 1. Ikhtisar & Mental Model Produk

Cuan mentransformasi cara kerja harian tim rekayasa perangkat lunak dan solopreneur melalui filosofi **Agentic Scrum**:
- **Zero Cognitive Overload:** Menggantikan struktur kanban board yang kaku menjadi antarmuka percakapan natural. Pengguna tidak perlu merumuskan kartu task manual satu per satu.
- **Dual-Tier Workplace Shell (shadcn `sidebar-09`):** Navigasi dua tingkat yang sangat responsif, membagi antarmuka menjadi **Primary Rail** (48px), **Contextual Sub-Panel** (240px), dan **Main Viewport**.
- **4 Pilar Esensial:**
  1. **Inbox:** Pintu masuk utama pekerjaan dan diskusi dengan Scrum Master.
  2. **Projects:** Repositori proyek terhubung dengan metrik task teragregasi.
  3. **Agents:** Visibilitas pool worker agent (Antigravity accounts, kuota token, dan status).
  4. **Team:** Hierarki peran transparan (Product Owner, Scrum Master, Development Team).

---

## 2. Diagram Alur Kerja Pengguna (D2 TALA)

```d2
direction: right

po: "Product Owner (Human)" {
  shape: person
}

shell: "shadcn sidebar-09 Workplace Shell" {
  rail: "Primary Icon Rail (48px)" {
    nav: "Inbox | Projects | Agents | Team"
    theme: "Theme Toggle (Tokyo Night / Light)"
    user: "NavUser Avatar Popover"
  }
  subpanel: "Contextual Sub-Panel (240px)" {
    threads: "Thread List & Status Badges"
    filter: "Status Filter (All, Unread, Running, Done)"
  }
  main_view: "Main Viewport" {
    chat: "Interactive Discussion & DoD Cards"
    composer: "Textarea Composer (Enter to send)"
    action: "1-Click 'Setujui & Jalankan Sprint'"
  }
}

engine: "Agentic Scrum Engine" {
  sm: "Scrum Master (AI Manager)"
  triage: "Autonomous DoD Distiller"
  pool: "Worker Agent Pool (Antigravity)"
  db: "PostgreSQL Database (19 Migrations)"
}

po -> shell.rail: "1. Buka Tab Inbox"
shell.rail -> shell.subpanel: "2. Render Thread List"
po -> shell.subpanel: "3. Buat Thread Baru (Ide / Bug)"
shell.subpanel -> shell.main_view: "4. Tulis Deskripsi Masalah"
shell.main_view -> engine.sm: "5. Kirim Pesan Pembuka"
engine.sm -> engine.triage: "6. Analisis Intent & Rancang DoD"
engine.triage -> engine.pool: "7. Pilih Worker dengan Kuota Optimal"
engine.triage -> shell.main_view: "8. Balas Thread dengan Rencana & DoD"
po -> shell.main_view: "9. 1-Click Approve"
engine.sm -> engine.db: "10. Spawn Task & Eksekusi Otomatis"
engine.db -> shell.main_view: "11. Stream Status 'Running' & Progress"
```

---

## 3. Komponen Navigasi Dua Tingkat (`sidebar-09`)

### 3.1 Primary Rail (Tier 1 - 48px)
- **Logo Cuan:** Penanda identitas aplikasi di bagian paling atas.
- **Menu Ikon Global:**
  - `Inbox` (`Inbox` icon): Menampilkan thread percakapan aktif.
  - `Projects` (`FolderGit2` icon): Menampilkan repositori kode terhubung.
  - `Agents` (`Bot` icon): Menampilkan kapasitas pool agent Antigravity.
  - `Team` (`Users` icon): Menampilkan pembagian peran organisasi.
- **Quick Add (`+` icon):** Jalan pintas membuat project baru dari mana saja.
- **Theme Toggle:** Tombol satu klik untuk beralih antara tema **Tokyo Night** (dark) dan mode terang.
- **Sub-Panel Toggle (`PanelLeft` icon):** Menutup atau membuka sub-panel konteks.
- **NavUser Avatar:** Popover profil pengguna, indikator role OWNER, dan akses ganti password / logout.

### 3.2 Contextual Sub-Panel (Tier 2 - 240px)
- **Inbox Context:** Menampilkan daftar seluruh percakapan beserta badge status (`Inbox/Amber`, `Running/Blue`, `Done/Emerald`), cuplikan pesan terakhir, dan stempel waktu relatif.
- **Projects Context:** Menampilkan daftar proyek aktif dengan tombol cepat `New Project`.
- **Agents Context:** Menampilkan daftar worker pool beserta persentase sisa kuota token.
- **Team Context:** Filter cepat berdasarkan peran (Product Owner, Scrum Master, Dev Team).

---

## 4. Alur Kerja Conversational Inbox & Autonomous SM Triage

1. **Intake Kebutuhan:**
   Product Owner menekan tombol **"New Thread"** di sub-panel Inbox, memasukkan judul, memilih proyek terkait (opsional), dan menjelaskan masalah atau ide secara bebas.
2. **Autonomous Triage:**
   Secara instan, Scrum Master (`@sm`) memproses input dan membalas dengan struktur standar Agentic Scrum:
   - **Problem Statement & Scope:** Ringkasan inti masalah yang teridentifikasi.
   - **Definition of Done (DoD) Checklist:** Kriteria keberhasilan konkret yang wajib dipenuhi sebelum task dinyatakan selesai.
   - **Rekomendasi Worker Agent:** Memilih model/akun Antigravity yang memiliki kuota token paling memadai (misal `antigravity-flash`).
3. **1-Click Approval:**
   Product Owner meninjau rencana. Cukup dengan menekan tombol **"Setujui & Jalankan Sprint"**, task otomatis:
   - Didaftarkan ke database `project_tasks` terikat ke `room_id`.
   - Status task berubah menjadi `doing`.
   - Worker agent ditugaskan secara otomatis.
   - Banner status eksekusi tampil di bagian atas composer.

---

## 5. Agents Hub & Team Directory

### 5.1 Agents Hub (Antigravity Pool)
- **Token Capacity Meters:** Bar indikator visual memperlihatkan persentase kuota token terpakai vs batas limit.
- **Preferred Routing:** Menandai worker agent berkecepatan tinggi (`antigravity-flash`) sebagai worker utama untuk pekerjaan harian.
- **Sandbox Isolation:** Setiap worker beroperasi di direktori terisolasi dengan akses git aman.

### 5.2 Team Directory
- **Product Owner (Buchin):** Pemilik produk manusia. Bertanggung jawab atas outcome dan approval.
- **Scrum Master (@sm):** AI Manager yang mengawal proses, triage pesan, dan verifikasi kriteria selesai.
- **Development Team (@pin, @pin2):** Worker agents yang menulis kode, menjalankan test terarah, dan melakukan handoff otomatis saat limit tercapai.

---

## 6. Keamanan & Retensi Data (PostgreSQL 19 Migrasi)

Seluruh data percakapan Inbox dan task terhubung aman di PostgreSQL:
- `project_rooms`: Menyimpan metadata thread percakapan.
- `project_messages`: Menyimpan histori obrolan lengkap dengan tipe author (`user`, `agent`, `system`).
- `project_tasks`: Terikat relasional dengan `room_id` dan mempertahankan 100% data migrasi dari Sprint 1 hingga Sprint 17 tanpa regresi schema.

---

## 7. Real-Time Task Progress Banner & Actionable Completion Card (Sprint 57)

Mulai Sprint 57, antarmuka obrolan Conversational Inbox dilengkapi dengan umpan balik visual instan (*real-time task feedback*) yang ramah pengguna awam dan non-developer:

### 7.1 Floating Live Progress Chip (`TaskProgressBanner`)
- **Deteksi Reaktif:** Terhubung langsung dengan selector `activeConversationTasks`, mendeteksi tugas aktif yang berstatus `queued` atau `running`.
- **Live Elapsed Timer & Agent:** Menampilkan indikator denyut (*pulse*), status antrean atau pemrosesan, nama agen pelaksana (`cuan-runner`), dan durasi berjalan (*live elapsed time*) yang bertambah per detik.
- **Polling Adaptif:** Mengaktifkan interval polling kilat (3–5 detik) saat tugas sedang berjalan (*in-flight*), dan otomatis kembali ke interval santai (*idle backoff* 20 detik) saat tidak ada tugas aktif, memastikan zero re-render storm dan server hemat sumber daya.
- **Non-Intrusive Floating:** Mengambang secara elegan di bagian atas aliran obrolan tanpa mengganggu alur membaca maupun interaksi formulir composer.

### 7.2 Actionable Completion Card (`TaskCompletionCard`)
- **Bahasa Bisnis Bebas Jargon:** Menampilkan ringkasan hasil kerja ramah UMKM (*business summary*), bukan sekadar tumpukan log teknis mentah.
- **Kriteria Selesai (DoD):** Menampilkan rekap pemenuhan kriteria Definition of Done dengan visual lencana hijau yang menenangkan (*zero guesswork & total peace of mind*).
- **Tombol Aksi Cepat:** Menyediakan tombol pintas langsung menuju halaman detail tugas serta tombol "Lanjutkan Diskusi" yang memicu instruksi lanjutan tanpa perlu mengetik ulang referensi tugas.

### 7.3 Background Audio Chime & Document Title Notification Badge
- **Harmonic Web Audio Synthesizer:** Menggunakan Web Audio API peramban murni (tanpa binary audio eksternal `.mp3`/`.wav`) dengan dua gelombang sinus harmonis C6 (1046.5 Hz) dan E6 (1318.5 Hz) serta exponential decay 0.4s yang lembut dan ramah pendengaran.
- **Zero-Friction Autoplay Fallback:** Menangani pembatasan izin autoplay browser secara senyap (*silent catch*) tanpa error ke konsol pengguna.
- **Visual Title Notification Badge:** Saat browser tab tidak aktif atau berada di latar belakang (*document.hidden*), judul tab otomatis menampilkan lencana dinamis `(1) Tugas Selesai! - Cuan` dan segera mengembalikan judul asli saat jendela difokuskan kembali (`window.onfocus`).

### 7.4 Worker Observability & Queue Health di Menu Pengaturan
- **Visual Indicator Ramah Bisnis:** Menyajikan widget status kesehatan daemon `cuan-runner` di halaman Pengaturan (`WorkerHealthCard`), dilengkapi lampu indikator visual (Hijau = Aktif/Siap, Kuning = Sedang Memproses, Merah/Abu-abu = Perhatian/Tidak Aktif).
- **Kedalaman Antrean:** Menampilkan metrik antrean tugas (*queued*, *running*, dan *queue depth*) serta detak jantung (*heartbeat*) terakhir dari worker secara seketika via endpoint `GET /api/system/worker-health`.


