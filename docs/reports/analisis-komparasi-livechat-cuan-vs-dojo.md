# Analisis Komparasi Fitur Live Chat: Cuan vs Dojo

**Tanggal:** 22 Oktober 2026  
**Penyusun:** Tim Scrum Master & Technical Auditor Cuan  
**Subjek:** Evaluasi Kesiapan Cuan sebagai Pengganti Penuh (*Drop-In Replacement*) Live Chat Dojo  
**Repositori Terkait:**  
- Cuan: `https://github.com/buchin/cuan.git` (Go Backend, React/TypeScript Frontend)
- Dojo: `https://github.com/imdojo/dojo.git` (Laravel Backend, React Operator UI)

---

## 1. Eksekutif Ringkasan (*Executive Summary*)

Audit komparasi menyeluruh telah dilakukan antara kapabilitas live chat pada repositori **Cuan** dan baseline acuan pada repositori **Dojo**. Evaluasi ini mencakup 6 pilar utama: arsitektur sistem, mekanisme transport & real-time, state management & telemetri, antarmuka pengguna (Widget Pengunjung & Operator UI), sistem penanganan lampiran (*attachment*), serta sistem notifikasi.

### Kesimpulan Kesiapan (*Readiness Verdict*):
> **STATUS: BELUM SIAP (NOT READY) UNTUK PENGGANTIAN PENUH 100% SAAT INI.**
>
> Cuan memiliki arsitektur modern berbasis Go dan Server-Sent Events (SSE) dengan integrasi agen kecerdasan buatan (*Autonomous AI Agents*) dan kartu tugas 1-klik (*1-Click Agent Action Card*) yang **jauh lebih unggul** daripada Dojo. Namun, untuk fungsi layanan pelanggan murni (*customer support live chat*), Cuan masih memiliki **4 kesenjangan kritis (*critical blockers*)**:
> 1. Belum adanya upload lampiran pengunjung (*visitor file/photo/camera upload*).
> 2. Larangan pengiriman attachment dari operator ke customer pada backend Cuan (HTTP 400).
> 3. Telemetri navigasi halaman pengunjung hanya tersimpan sementara di memori (*in-memory*) dan hilang saat restart.
> 4. Belum tersedianya *canned responses* (template balasan cepat) dan notifikasi email fallback saat operator luring (*offline*).

---

## 2. Audit Kapabilitas Live Chat Cuan

### 2.1 Arsitektur & Komponen
- **Backend**: Golang menggunakan router `go-chi/chi/v5` pada service `cuan-server`.
  - Handler live chat terpusat di `internal/server/chat_handlers.go`.
  - Penyimpanan data utama menggunakan PostgreSQL melalui tabel `projects`, `project_rooms`, `project_messages`, dan `crm_contacts` (`migrations/000022_contacts_crm.up.sql`).
- **Frontend Operator**: Single Page Application (SPA) berbasis React 18, TypeScript, Vite, Tailwind CSS, dan komponen shadcn/ui zinc/slate (`web/src/features/inbox/InboxView.tsx`, `InboxSubPanel.tsx`, `Attachments.tsx`, `EscalateTaskDialog.tsx`).
- **Visitor Widget**: Skrip Vanilla JavaScript mandiri (`web/public/chat/widget.js`, 721 baris) yang disuntikkan ke dalam **Shadow DOM** terbuka (`attachShadow({ mode: 'open' })`) guna memastikan isolasi CSS menyeluruh dari situs induk (*host website*).
- **Protokol Kompatibilitas (*Shims*)**:
  - `window.$dojoChat`: Menyediakan method `open()`, `close()`, `toggle()`, `setMessage()`, `startTopic()`, dan `identify()`.
  - `window.$crisp`: Mengadopsi format antrean array push (`do: chat:open`, `chat:close`, `chat:toggle`, `message:send`, `set: user:email`, `user:nickname`, `message:text`).
- **CORS Policy**: Middleware `chatCORSMiddleware` menerapkan open CORS (`Access-Control-Allow-Origin: *`, `GET, POST, PUT, PATCH, DELETE, OPTIONS`) untuk semua endpoint di bawah `/api/chat/*`.

### 2.2 Transport & Real-Time
- **Visitor Widget**: Menggunakan interval polling HTTP GET berkala (`GET /api/chat/messages?session_uuid=...&after_id=...`) setiap 3.500 ms (3,5 detik).
- **Internal Operator**: Memanfaatkan **Server-Sent Events (SSE)** melalui endpoint `GET /api/chat/conversations/{id}/events` (`handleInboxStream`), memungkinkan streaming pembaruan pesan dan output agen AI secara real-time dan reaktif tanpa jeda polling.

### 2.3 State Management & Telemetri
- **Widget**: Mengelola state di memori lokal serta persistensi `localStorage` (`dojo_chat_guest_uuid` dan `dojo_chat_session_v2:...`).
- **Backend Registry**: `globalChatManager` (`*ChatSessionManager` dengan `sync.RWMutex`) menyimpan map sesi aktif (`sessions` dan `sessionByGuest`).
- **Keterbatasan Utama**: Endpoint telemetri `POST /api/chat/sessions/{uuid}/page` (`handleVisitorPageTelemetry`) hanya memperbarui struct di memori. Tidak ada kolom penyimpanan URL, judul halaman, atau referrer di tabel `crm_contacts` atau `project_rooms`, sehingga data telemetri pengunjung lenyap jika service Cuan dimulai ulang (*restart*).

### 2.4 Antarmuka Pengguna (UI/UX)
- **Direct Messenger Hub**: Mengadopsi pola WhatsApp/Telegram neutral style dengan sub-panel kontak (`InboxSubPanel`) yang membagi filter:
  - `All`: Seluruh kontak aktif.
  - `Agents`: Asisten kecerdasan buatan (Scrum Master, Antigravity Flash).
  - `Customers`: Pengunjung web aktif dari ekosistem Dojo.
- **Tampilan Percakapan**: Bubble stream dengan timestamp lokal, double checkmark receipt, auto-resizing composer (Enter = Kirim, Shift+Enter = Baris baru).
- **Fitur Unggulan Khusus Cuan**:
  - **1-Click Agent Action Card**: Kartu rekomendasi tugas terstruktur lengkap dengan checklist Definition of Done (DoD) dan tombol eksekusi 1-klik (`tasksApi.runAgent`).
  - **Task Escalation**: Modal dialog `EscalateTaskDialog.tsx` untuk mengubah keluhan atau masukan pengunjung menjadi project task secara langsung.

### 2.5 Penanganan Attachment & Media
- Operator dapat mengunggah file untuk percakapan AI Agent via endpoint `/api/v1/chat/attachments`.
- **Kekurangan Kritis**:
  - Tidak ada endpoint publik bagi pengunjung untuk mengunggah berkas.
  - Widget pengunjung (`widget.js`) tidak memiliki UI attachment picker / upload.
  - Backend Cuan secara eksplisit menolak pengiriman attachment oleh operator ke pelanggan dengan pesan error HTTP 400: `"Attachments are currently supported in agent conversations"` (`internal/server/chat_handlers.go#L1055-L1056`).

### 2.6 Notifikasi & Receipts
- Endpoint `POST /api/chat/receipts` hanya mengembalikan payload stub statis `{"ok": true, "receipts": []}`.
- Belum ada integrasi pengiriman email darurat (*delayed email digest*) saat operator tidak merespons pesan pengunjung.
- Belum ada pemberitahuan suara (*sound chime ping*) pada widget pengunjung.

---

## 3. Audit Kapabilitas Live Chat Dojo (Baseline Acuan)

### 3.1 Arsitektur & Komponen
- **Backend**: Laravel (PHP 8.2+), Eloquent ORM, MySQL/PostgreSQL, Redis.
- **Model & Database Schema**:
  - `chat_sessions`: Menyimpan `uuid`, `user_id`, `guest_uuid`, `name`, `email`, `status` (`active`, `closed`, `resolved`), `current_url`, `page_title`, `referrer_url`, `last_page_updated_at`.
  - `chat_messages`: Mendukung kolom relasional lampiran (`attachment_url`, `attachment_type`, `attachment_name`, `attachment_size`) dan status baca (`read_at`, `delivered_at`).
  - `chat_escalations`: Relasi eskalasi tugas ke issue tracker eksternal.
  - `chat_email_deliveries`: Pencatatan log email notifikasi pengunjung dan operator.
- **Operator Dashboard**: `LiveChatOperator.tsx` (5.696 baris kode) berbasis React UI komprehensif.
- **Visitor Widget**: `public/chat/widget.js` (1.922 baris kode), memiliki fitur file upload, audio chimes, markdown formatting, lightbox, dan deteksi offline.

### 3.2 Transport & Real-Time
- Polling adaptif dengan cursor ID (`after_id`) dan interval dinamis (1,5 detik saat aktif hingga 5 detik saat idle).
- Integrasi event dispatcher Laravel `NewChatMessage` dengan listener webhook dan queue jobs.
- Layanan read receipt terverifikasi (`ChatReceiptService`).

### 3.3 State Management & Visitor Intelligence
- Persistensi telemetri penuh di database: Setiap perpindahan rute URL pengunjung dicatat di database bersama timestamp.
- **Visitor Intelligence Engine** (`AdminChatController@intelligence`): Mengompilasi profil komprehensif pengunjung:
  - Profil akun terdaftar (nama, email, saldo, tier).
  - Riwayat transaksi & status pembayaran terakhir (`Payment` model).
  - Metadata perangkat: Browser, Sistem Operasi, Resolusi Layar, IP Address, Geolocation.
  - Riwayat sesi chat sebelumnya dan durasi waktu di halaman saat ini.

### 3.4 Antarmuka Pengguna & Fitur Operator
- **3-Panel Operator Layout**: Panel daftar sesi dengan tab `Chats`, `Online Visitors`, dan `Closed/Resolved`, panel percakapan utama, serta panel intelijen pengunjung (*sidebar intelligence drawer*).
- **Canned Responses (Library Balasan Cepat)**:
  - Koleksi puluhan template jawaban terstruktur berdasarkan kategori (*Payment, Troubleshooting, General, Onboarding*).
  - Pencarian instan template jawaban via modal, bottom sheet, atau popover.
  - Dukungan shortcut keyboard slash `/` (`/reply`, `/resolve`, `/escalate`) di composer.
- **Manajemen Sesi**: Kemampuan menutup chat (*close*), menyelesaikan chat (*resolve*), dan membuka kembali (*reopen*) secara formal.

### 3.5 Sistem Penanganan Attachment
- Endpoint publik pengunjung `POST /api/chat/attachments` (`ChatAttachmentController@uploadVisitor`).
- Endpoint admin operator `POST /api/admin/chat/attachments` (`ChatAttachmentController@uploadAdmin`).
- Penyimpanan aman ke Cloudflare R2 (`Storage::disk('dojo')`) dengan nama hash 40 karakter acak dan proteksi URL streaming terisolasi (`ChatAttachmentController@show` dengan header `X-Content-Type-Options: nosniff`).
- Validasi keamanan berlapis:
  - Whitelist ekstensi yang diizinkan: `.jpg`, `.jpeg`, `.png`, `.webp`, `.gif`, `.pdf`, `.zip`.
  - Blacklist ketat pencegahan *double-extension* file berbahaya (menolak `.php`, `.sh`, `.exe`, `.py`, dll.).
  - Batas ukuran berkas maksimal 5 MB (5.120 KB).

### 3.6 Notifikasi & Email Fallback
- **Queue Jobs Otomatis**:
  - `SendDelayedChatEmailDigestJob`: Mengirim email digest kepada tim operator jika pesan pengunjung tidak direspons dalam jangka waktu tertentu.
  - `SendChatTeamAlertJob` & `SyncChatTeamBadgeJob`: Sinkronisasi notifikasi badge dan alert ke platform tim.
- **Email Transkrip**: `ChatTranscriptMail` mengirimkan riwayat percakapan secara rapi ke email pengunjung saat sesi ditandai selesai (*resolved*).
- **Pemberitahuan Suara**: Web Audio API Chime pada widget pengunjung saat pesan baru tiba.

---

## 4. Pemetaan Feature Gap (Matriks Komparasi)

Berikut adalah rekapitulasi perbandingan kapabilitas antara kedua sistem:

| Domain | Fitur / Kapabilitas | Dojo (Baseline Acuan) | Cuan (Kondisi Saat Ini) | Status Evaluasi |
| :--- | :--- | :--- | :--- | :--- |
| **Arsitektur** | Bahasa & Runtime Backend | PHP 8.2 (Laravel) | Go 1.27 (High Performance) | **Cuan Lebih Cepat** |
| | Isolasi Shadow DOM Widget | Ya (`attachShadow`) | Ya (`attachShadow`) | **Setara** |
| | Dukungan Drop-in Shim `$crisp` | Ya (Array Push Queue) | Ya (Array Push Queue) | **Setara** |
| | Dukungan Shim API `$dojoChat` | Ya (Open, Close, Toggle, dll.)| Ya (Open, Close, Toggle, dll.)| **Setara** |
| | Open CORS Lintas Domain | Ya (`*`) | Ya (`*`) | **Setara** |
| **Transport** | Real-Time Visitor Stream | Polling berkala (1.5s - 5s) | Polling berkala (3.5s) | **Setara** |
| | Real-Time Operator Stream | Polling HTTP | **Server-Sent Events (SSE)** | **Cuan Unggul** |
| **Lampiran (Attachment)** | Upload Berkas Pengunjung | **Tersedia** (R2, Drag-drop, Paste) | **Belum Ada** (Tidak ada endpoint) | **Gap Kritis (P-0)** |
| | Operator Kirim Berkas ke Pengunjung | **Tersedia** | **Ditolak** (HTTP 400 di backend) | **Gap Kritis (P-0)** |
| | Validasi MIME & Ekstensi Aman | Whitelist + Blacklist Eksekusi | Belum ada pipeline publik | **Gap Kritis (P-0)** |
| | Lightbox Preview Gambar di Widget | Tersedia | Belum Ada | **Gap Minor (P-3)** |
| **Telemetri & Data** | Pelacakan Navigasi Halaman | Persisten di database PostgreSQL | Hanya in-memory (hilang saat restart)| **Gap Mayor (P-1)** |
| | Visitor Intelligence (Billing/Orders)| Tersedia via integrasi DB | Belum Ada | **Gap Mayor (P-1)** |
| | Read Receipts Terverifikasi | Persisten (`read_at`, `delivered_at`)| Stub statis kosong | **Gap Minor (P-2)** |
| **Operator Experience** | Canned Responses (Balasan Cepat) | Library lengkap, search, `/` shortcut | Belum Ada | **Gap Mayor (P-1)** |
| | Filter Kontak & Pencarian | Chats, Online, Closed, Search | All, Agents, Customers, Search | **Setara** |
| | Status Percakapan (Resolve/Close) | Ya (Active, Closed, Resolved) | Belum ada kontrol status di UI | **Gap Mayor (P-1)** |
| **Notifikasi** | Email Fallback / Unread Digest | Otomatis via Delayed Queue Job | Belum Ada | **Gap Mayor (P-1)** |
| | Email Transkrip Percakapan | Tersedia (`ChatTranscriptMail`) | Belum Ada | **Gap Minor (P-2)** |
| | Audio Chime Ping di Widget | Tersedia (Web Audio API) | Belum Ada | **Gap Minor (P-3)** |
| **AI & Workflow** | Saran Balasan AI | Tersedia (`ChatAiSuggestionService`)| Tersedia via interaksi Scrum Master | **Setara** |
| | **1-Click Agent Action Card & DoD** | Tidak Ada | **Tersedia Penuh** | **Cuan Unggul Signifikan** |
| | **Eskalasi Task & Isolasi Branch** | Terbatas (External Issue Task) | **Tersedia Penuh** (Workspace & Git)| **Cuan Unggul Signifikan** |

---

## 5. Rekomendasi & Rencana Tindak Lanjut (*Actionable Roadmap*)

Sebelum Cuan dapat menggantikan live chat Dojo secara penuh pada lingkungan produksi, tim pengembang harus menyelesaikan 4 tahap perbaikan berurutan:

### Fase 1: Implementasi Pipeline Lampiran Publik (Prioritas P-0, Critical Blocker)
1. **Endpoint Backend Publik**:
   - Tambahkan route `POST /api/chat/attachments` pada router publik Cuan (`internal/server/server.go`).
   - Implementasikan validasi ketat: ukuran file maksimal 5 MB, whitelist ekstensi gambar/dokumen aman (`jpg`, `jpeg`, `png`, `webp`, `gif`, `pdf`, `zip`), dan pencegahan nama berkas ganda (*double-extension*).
   - Simpan berkas ke direktori lampiran Cuan (`~/.local/share/cuan/attachments/`) atau S3/R2 storage driver.
2. **Koreksi Logika Pengiriman Operator**:
   - Hapus batasan di `internal/server/chat_handlers.go#L1055-L1056` yang memblokir lampiran untuk percakapan selain agen. Izinkan operator mengirim berkas ke sesi *customer*.
3. **Peningkatan Visitor Widget (`web/public/chat/widget.js`)**:
   - Tambahkan tombol ikon klip kertas / kamera pada baris composer.
   - Tambahkan listener input berkas dan progress bar indikator pengunggahan.

### Fase 2: Persistensi Telemetri Halaman & Manajemen Status Sesi (Prioritas P-1, High)
1. **Skema Database**:
   - Tambahkan kolom `current_url TEXT`, `page_title TEXT`, `referrer_url TEXT`, dan `status VARCHAR(32) DEFAULT 'active'` pada tabel `crm_contacts`.
2. **Handler Backend**:
   - Perbarui `handleVisitorPageTelemetry` untuk melakukan `UPDATE crm_contacts SET current_url = ..., page_title = ..., updated_at = NOW() WHERE session_uuid = ...`.
3. **Manajemen Status di Operator UI**:
   - Sediakan tombol "Selesaikan Percakapan / Resolve" di `InboxView.tsx` yang mengubah status sesi menjadi `closed` atau `resolved`.

### Fase 3: Template Balasan Cepat (*Canned Responses*) (Prioritas P-1, High)
1. **Operator Experience**:
   - Sediakan daftar balasan standar (*canned responses*) untuk pertanyaan seputar status server, panduan pembayaran, dan bantuan aktivasi token.
   - Sediakan popover pemilih balasan cepat atau trigger tombol pintas `/` pada composer operator.

### Fase 4: Notifikasi Email & Polish Audio (Prioritas P-2, Medium)
1. **Fallback Email**:
   - Implementasikan scheduler/worker ringan di Cuan untuk memeriksa sesi pengunjung yang memiliki pesan belum dibalas selama lebih dari 10 menit, lalu kirimkan email rangkuman (*digest*) ke alamat pengelola (*owner*).
2. **Widget Audio Chime**:
   - Tambahkan sintesis nada pendek (Web Audio oscillator) pada `widget.js` yang berbunyi ketika pesan operator berhasil diterima pengunjung.

---

## 6. Kesimpulan Akhir

Arsitektur Cuan Direct Messenger Hub membuktikan keunggulan besar dalam integrasi AI Agent dan otomatisasi siklus delegasi tugas (*Scrum Master & Action Cards*). Namun, untuk interaksi langsung dengan konsumen luar, fungsi utilitas dasar customer support (khususnya penanganan berkas dan telemetri persisten) harus dilengkapi terlebih dahulu sesuai roadmap di atas sebelum migrasi total dari Dojo live chat dieksekusi.
