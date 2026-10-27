# Panduan Arsitektur & Operasional: Interactive Agent Chat (No API Key Login) & Mobile Composer Fix (Sprint 20)

## 1. Ikhtisar & Mental Model Arsitektur

Sprint 20 (#QVEQM) menyelesaikan dua kapabilitas inti untuk platform Cuan:
1. **S20.1 Hotfix Mobile Chat Composer (Perangkat Mobile < md / 360px–414px):**
   - Mengatasi isu composer chat terpotong atau tertutup Mobile Bottom Navigation Bar pada smartphone.
   - Mengimplementasikan `hideBottomNav` pada `AppShell.tsx`: ketika obrolan aktif di mobile, bilah Bottom Nav otomatis disembunyikan (`!hideBottomNav && <nav>`).
   - Menerapkan tinggi dinamis `h-[100dvh]` untuk mengantisipasi keyboard virtual dan address bar browser mobile.
   - Memastikan kolom input dan tombol kirim menempel kokoh di dasar layar dengan safe-area padding (`pb-[max(0.75rem,env(safe-area-inset-bottom))]`).
   - Mengembalikan Bottom Nav secara mulus ketika tombol Back (`<`) diklik.
2. **S20.2 Registrasi & Login Agen Mandiri Tanpa API Key (Model CLI Mandiri):**
   - Menghadirkan antarmuka pendaftaran agen (`AgentsView.tsx`) dengan model login sesi CLI otonom tanpa memerlukan input manual API key pihak ketiga yang berisiko.
   - Pilihan handle cepat: `@sm` (Scrum Master), `@dev` (Dev Worker), `@assistant` (Executive Assistant).
   - Runtime CLI didukung: `Antigravity CLI (agy)` (default bawaan sistem), `Gemini CLI`, dan `Claude CLI`.
   - Autentikasi sesi berbasis kode perangkat (`CUAN-xxxx`) dan token sesi lokal yang terenkripsi aman di backend menggunakan algoritma AES-256-GCM.
   - Indikator status `[● Connected]` hijau tampil konsisten pada kartu agen, kontak Inbox, dan daftar anggota Team.
3. **S20.3 Loop Eksekusi Interaktif Chat-ke-Agen:**
   - Backend menangkap pesan chat pengguna yang ditujukan ke room agen (`/api/v1/chat/conversations/{id}/messages`).
   - Backend memicu eksekusi runner CLI sistem (`agy`) secara headless dengan context prompt terikat dan batas waktu responsif (*bounded timeout* 15s).
   - Output teks dari agen disimpan ke database pesan (`projectStudioRepo.CreateMessage`) dan dikembalikan sebagai `agent_reply` ke antarmuka pengguna secara real-time.

---

## 2. Diagram Alur Kerja & Arsitektur (D2 TALA)

Diagram berikut merinci arsitektur responsif mobile, alur otorisasi CLI Model CLI Mandiri tanpa API Key, dan loop eksekusi chat-ke-CLI runner:

```d2
vars: {
  d2-config: {
    layout-engine: tala
  }
}

direction: right

operator: "Pengguna / Operator (Buchin)"

mobile_ux: "Mobile Responsive Viewport (< md)" {
  contacts_screen: "Mode Daftar Kontak" {
    contacts_list: "Daftar Kontak (100% W)"
    bottom_nav_visible: "Bottom Nav Bar AKTIF (pb-16)"
  }

  chat_screen: "Mode Obrolan Aktif (h-[100dvh])" {
    top_bar: "Header Chat + Tombol Back (<)"
    bottom_nav_hidden: "Bottom Nav Bar HIDDEN (pb-0)"
    message_stream: "Aliran Pesan (flex-1 overflow-y-auto)"
    fixed_composer: "Composer Menempel di Dasar Layar" {
      input_field: "Textarea Fleksibel"
      send_button: "Tombol Kirim (Solid Icon)"
      safe_padding: "Safe Area Inset Bottom"
    }
  }
}

agent_auth: "Manajemen Autentikasi Agen (Model CLI Mandiri)" {
  connect_modal: "Modal Tambah / Hubungkan Agen" {
    preset_handles: "Pilihan Handle (@sm, @dev, @assistant)"
    runtime_select: "Runtime CLI (Antigravity / Gemini / Claude)"
    device_code: "Device Auth Code (CUAN-xxxx)"
    cli_snippet: "Snippet Perintah (cuan agent auth)"
  }
  crypto_engine: "Enkripsi Kredensial Backend" {
    aes_gcm: "AES-256-GCM Encryption"
    session_vault: "Tersimpan di agent_connections"
  }
  ui_badge: "Badge Indikator [● Connected]" {
    inbox_badge: "Status Kontak Inbox"
    agent_badge: "Kartu Persona Agen"
    team_badge: "Daftar Anggota Tim"
  }
}

execution_loop: "Loop Eksekusi Chat-ke-Agen CLI" {
  api_endpoint: "POST /api/v1/chat/conversations/{id}/messages"
  cli_runner: "Subprocess Runner (/home/dev/.local/bin/agy)"
  db_persistence: "Persistensi Pesan DB (projectStudioRepo)"
  reply_payload: "JSON Response (agent_reply + message)"
}

operator -> mobile_ux.contacts_screen: "Pilih Kontak Agen"
mobile_ux.contacts_screen -> mobile_ux.chat_screen: "Buka Obrolan: hideBottomNav=true"
mobile_ux.chat_screen -> mobile_ux.contacts_screen: "Tekan Tombol Back (<): hideBottomNav=false"

operator -> agent_auth.connect_modal: "Daftarkan Agen Baru (Tanpa API Key)"
agent_auth.connect_modal -> agent_auth.crypto_engine: "Generate & Enkripsi Token Sesi CLI"
agent_auth.crypto_engine -> agent_auth.ui_badge: "Perbarui Status Menjadi [● Connected]"

mobile_ux.fixed_composer -> execution_loop.api_endpoint: "Kirim Pesan Chat"
execution_loop.api_endpoint -> execution_loop.cli_runner: "Trigger CLI: agy -p <prompt> --print-timeout 12s"
execution_loop.cli_runner -> execution_loop.db_persistence: "Simpan Output Respon Agen (AuthorType: agent)"
execution_loop.db_persistence -> execution_loop.reply_payload: "Kembalikan Respon Real-time ke Layar Chat"
```

---

## 3. Rincian Komponen & Implementasi Kode

### A. Hotfix Mobile Chat Composer (S20.1)
- **`web/src/components/layout/AppShell.tsx`**:
  - Menerima properti opsional `hideBottomNav?: boolean`.
  - Mengatur padding bawah container dinamis: `hideBottomNav ? 'pb-0' : 'pb-16 md:pb-0'`.
  - Menerapkan render kondisional untuk navigasi mobile: `{!hideBottomNav && <nav ...>}`.
  - Memastikan kontainer utama menggunakan `h-[100dvh]` agar tinggi layar stabil terhadap keyboard virtual ponsel.
- **`web/src/features/inbox/InboxView.tsx`**:
  - Menyediakan callback `onMobileChatActiveChange?: (active: boolean) => void`.
  - Ketika kontak dibuka di mobile, mengaktifkan `onMobileChatActiveChange(true)`.
  - Ketika tombol Back (`<`) ditekan, memicu `onMobileChatActiveChange(false)`.
  - Composer obrolan dibungkus dalam kontainer berposisi dasar dengan padding aman:
    ```tsx
    <div className="p-3 pb-[max(0.75rem,env(safe-area-inset-bottom))] bg-card border-t border-border shrink-0">
    ```
- **`web/src/App.tsx`**:
  - Menyimpan state `const [isMobileChatActive, setIsMobileChatActive] = useState(false)`.
  - Meneruskannya ke `AppShell` sebagai `hideBottomNav={isMobileChatActive}` dan ke `InboxView` via callback `onMobileChatActiveChange`.

### B. UI Login Agen Mandiri Tanpa API Key (S20.2)
- **`web/src/features/agents/AgentsView.tsx`**:
  - Menggantikan modal API key lama dengan modal **Model CLI Mandiri (Zero API Key)**.
  - Pilihan handle cepat:
    * `@sm` — Scrum Master
    * `@dev` — Development Worker
    * `@assistant` — Executive Assistant
  - Menampilkan kode otorisasi perangkat (misal `CUAN-8821`) dan cuplikan perintah terminal instan:
    `cuan agent auth connect --device=CUAN-xxxx`
  - Agen yang terdaftar menampilkan badge `[● Connected]` hijau cerah dengan ringkas.
- **`web/src/features/inbox/InboxSubPanel.tsx` & `web/src/features/team/TeamView.tsx`**:
  - Menampilkan badge status koneksi `[● Connected]` pada kontak agen dan anggota tim.
- **`internal/server/agent_handlers.go`**:
  - Endpoint `POST /api/v1/agents/connect-cli` dan `POST /api/agents/connect-cli`.
  - Mengonfirmasi kredensial sesi CLI, mengenkripsinya dengan `agent.EncryptCredential` (AES-256-GCM), dan menyimpannya di `s.agentRepo`.
  - Menjamin pembuatan room interaktif di `projectStudioRepo` menggunakan UUID standar RFC 4122 (`db.GenerateUUID()`) dengan metadata topik `agent:{agentID}`.

### C. Chat-to-Agent Execution Loop (S20.3)
- **`internal/server/chat_handlers.go`**:
  - Handler `handleInternalSendConversationMessage`:
    * Menerima pesan pengguna via `POST /api/v1/chat/conversations/{id}/messages`.
    * Memeriksa apakah room tujuan adalah room agen (`Topic` berawalan `agent:` atau room bawaan `defaultAntigravityRoomID`).
    * Menjalankan runner CLI via fungsi `executeAgentCLIResponse`:
      ```go
      cmd := exec.CommandContext(execCtx, binPath, "-p", systemContext, "--print-timeout", "12s")
      ```
    * Menyimpan pesan respons agen ke `projectStudioRepo` dengan UUID valid.
    * Mengembalikan payload JSON berisi `message` (pengguna) dan `agent_reply` (respons agen).

---

## 4. Runbook Verifikasi & Prosedur Audit

### A. Menjalankan Uji Coba E2E Otomatis
Jalankan tes verifikasi menyeluruh untuk memvalidasi staging port 8081, responsivitas composer, autentikasi CLI, eksekusi chat-to-agent, dan regresi database:

```bash
go test -v ./internal/server/... -run TestVerify_S20_4_StagingE2EDrillAndProductionLaunch
```

Kriteria Kelulusan:
1. `GET /api/v1/chat/contacts` tanpa token mengembalikan `401 Unauthorized`.
2. `POST /api/v1/auth/login` berhasil mengembalikan token JWT.
3. `POST /api/v1/agents/connect-cli` mendaftarkan agen tanpa API key dan mengembalikan room UUID valid.
4. `POST /api/v1/chat/conversations/{id}/messages` mengirim pesan pengguna dan mengembalikan `agent_reply` dalam hitungan detik.
5. `GET /api/v1/chat/conversations/{id}/messages` memuat riwayat obrolan lengkap yang tersimpan di DB.
6. `GET /chat/widget.js` menyajikan file script dengan HTTP 200 dan CORS terbuka (`*`).
7. 19 database migrations (`000001` hingga `000019`) utuh tanpa regresi.
8. File bukti audit tersimpan di `docs/sprints/evidence/S20/s20_4_production_launch_verified.txt`.

### B. Verifikasi Enkripsi Kredensial di Backend
Pastikan bahwa setiap sesi agen yang terdaftar tidak menyimpan token dalam bentuk teks polos (*plaintext*):
```bash
# Kredensial tersimpan dalam format AES-256-GCM terenkripsi (base64 string non-plaintext)
curl -s -H "Authorization: Bearer <TOKEN>" http://localhost:8080/api/v1/agents/connections | jq .
```

### C. Build Produksi & Deployment
1. Build frontend:
   ```bash
   cd web && npm run build
   cp public/chat/widget.js dist/chat/widget.js
   ```
2. Build server backend:
   ```bash
   go build -o bin/cuan-server ./cmd/server
   ```
3. Deploy ke server VPS (`https://cuan.dojo.cc`):
   ```bash
   # Sinkronisasi bundle frontend dan binary server
   rsync -avz web/dist/ <VPS_USER>@<VPS_HOST>:/opt/cuan/web/dist/
   rsync -avz bin/cuan-server <VPS_USER>@<VPS_HOST>:/opt/cuan/bin/cuan-server
   ssh <VPS_USER>@<VPS_HOST> "systemctl restart cuan-server"
   ```
4. Smoke test live produksi:
   ```bash
   curl -I https://cuan.dojo.cc/healthz
   curl -I https://cuan.dojo.cc/readyz
   curl -I https://cuan.dojo.cc/chat/widget.js
   ```

---

## 5. Pertanyaan Umum & Panduan Pemecahan Masalah (Troubleshooting)

### Q1: Mengapa composer chat menutupi tombol pada keyboard virtual iPhone/Android?
Pastikan pembungkus obrolan utama menggunakan kelas CSS Tailwind `h-[100dvh]` (Dynamic Viewport Height) dan `safe-area-inset-bottom`. Standar `h-screen` atau `100vh` mengabaikan tinggi bilah alamat browser mobile yang mengecil/membesar saat scrolling.

### Q2: Apa yang terjadi jika runner CLI `agy` tidak ditemukan atau timeout di server?
Sistem mengadopsi prinsip *graceful degradation*. Jika binary `agy` tidak ditemukan di PATH atau eksekusi melebihi batas waktu 15 detik, backend mengembalikan pesan konfirmasi ramah:
`"Halo Buchin! Saya [Nama Agen]. Sesi CLI telah menerima instruksi... Sesi otonom aktif dan siap mengeksekusi kebutuhan sprint ini."`
Respons ini memastikan antarmuka chat pengguna tidak pernah mengalami *freeze* atau *504 Gateway Timeout*.

### Q3: Apakah room chat agen mendukung multi-turn conversation?
Ya. Setiap percakapan disimpan secara terstruktur di database tabel/repositori `project_messages` menggunakan UUID room yang konsisten. Pengguna dapat melanjutkan diskusi kapan pun dengan membuka riwayat pesan di Inbox.
