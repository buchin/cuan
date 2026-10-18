# Panduan Operasional Mandiri: Self-Hosted Agent CLI Runner & Live Web Terminal (xterm.js)

Dokumen ini merupakan panduan operasional mandiri (*self-contained operational guide*) berstandar §11 bagi pengguna mandiri (*Solo Developer*), operator agensi digital, pengelola multi-brand studio, dan administrator sistem mengenai arsitektur sistem Sprint 22 Cuan:
1. **Agent CLI Account Onboarding & Session Store:** Mekanisme onboarding agen CLI host (`agy`, `claude`, `shell`), deteksi otomatis binary di sistem host, pairing akun lokal via CLI/API, dan penyimpanan token sesi terenkripsi/terisolasi di `~/.cuan/agent_sessions.json`.
2. **PTY Task Runner Engine with Agent Dispatcher:** Engine proses PTY berbasis `github.com/creack/pty` yang membungkus pemanggilan agen CLI dengan parameter prompt task, streaming bidirectional I/O melalui WebSocket, pencatatan kode keluar (*exit code*), dan pembaruan otomatis status eksekusi task (`idle`, `running`, `completed`, `error`).
3. **Interactive Web Terminal (xterm.js) di Task SidePeek:** Terminal interaktif berbasis `@xterm/xterm`, `@xterm/addon-fit`, dan `@xterm/addon-web-links` yang responsif, mendukung tema Tokyo Night / Dark Slate, auto-fit resize, tombol aksi cepat (*Run Agent*, *Clear*, *Reconnect*), dan intervensi keyboard langsung.
4. **Agent Assignment & Visual Status Indicators:** Pemilihan agen penanggung jawab di Task SidePeek serta sinkronisasi visual indikator status (Running hijau berkedip, Completed biru muda, Error merah, Idle abu-abu) pada papan Kanban Global Task Board.
5. **Zero DB Migration Regression & Production Launch:** Menjaga keutuhan 19 migrasi database PostgreSQL tanpa penambahan skema baru (memanfaatkan kolom `assignee_id`, `execution_status`, dan `agent_output` dari migrasi 000012, 000014, 000015), deployment bundle frontend ke `/opt/cuan/web/dist`, binary `cuan-server` ke VPS `cuan.dojo.cc`, dan verifikasi Nginx WebSocket reverse proxy.

---

## 1. Arsitektur Komponen & Alur Bidirectional Terminal (D2 TALA)

Diagram berikut menjelaskan siklus hidup aliran data antara peramban web pengguna, Nginx reverse proxy dengan WebSocket upgrade, Cuan Go Server, PTY Manager, dan Agent CLI runner pada host:

```d2
direction: right

browser: "Browser Klien (Frontend Web)" {
  task_peek: "TaskSidePeek.tsx (Tab Runner)" {
    xterm_ui: "Xterm.js Terminal Instance"
    addons: "FitAddon & WebLinksAddon"
    agent_selector: "Agent Assignment Dropdown"
    action_toolbar: "Run Agent / Clear / Reconnect"
  }
}

reverse_proxy: "Nginx Gateway (cuan.dojo.cc:443)" {
  ws_filter: "WebSocket Upgrade Forwarder (/api/v1/tasks/:id/terminal)"
  http_api: "HTTP REST API Forwarder (/api/v1/*)"
}

cuan_backend: "Cuan Backend Server (Go Port 8080)" {
  api_router: "Chi Router & Auth Gatekeeper"
  agent_session_store: "Agent Session Store (~/.cuan/agent_sessions.json)"
  terminal_manager: "PTY Terminal Manager (internal/terminal)"
  pty_process: "PTY Subprocess (creack/pty)"
}

host_cli: "Host Environment & CLI Agents" {
  agy_cli: "Antigravity CLI (agy)"
  claude_cli: "Claude Code CLI (claude)"
  shell_cli: "Interactive Shell (bash / sh)"
  task_worktree: "Project Workspace Directory"
}

browser.task_peek.action_toolbar -> cuan_backend.api_router: "1. POST /api/v1/tasks/{id}/run-agent"
cuan_backend.api_router -> cuan_backend.terminal_manager: "2. Dispatch Agent Runner (Prompt & Env)"
cuan_backend.terminal_manager -> cuan_backend.pty_process: "3. Spawn creack/pty session"
cuan_backend.pty_process -> host_cli.agy_cli: "4. Exec agy / fallback bash"
browser.task_peek.xterm_ui -> reverse_proxy.ws_filter: "5. WebSocket Connect (/api/v1/tasks/{id}/terminal)"
reverse_proxy.ws_filter -> cuan_backend.terminal_manager: "6. Bidirectional Raw PTY Stream (WS Upgrade)"
cuan_backend.terminal_manager -> browser.task_peek.xterm_ui: "7. Live ANSI Output Chunks"
```

---

## 2. Setup & Pairing Instructions Local Agent CLI Runner

Cuan menyediakan dukungan bawaan untuk mendeteksi binary AI agent CLI yang terpasang pada host server atau workstation pengembang.

### 2.1. Deteksi Binary Agent via CLI
Jalankan binary `cuan-server` dengan argumen `--detect-agents` untuk memindai ketersediaan binary di sistem lokal (`$PATH`):

```bash
./bin/cuan-server --detect-agents
```

Contoh keluaran JSON yang dihasilkan:
```json
[
  {
    "type": "agy",
    "name": "Antigravity CLI",
    "path": "/home/dev/.local/bin/agy",
    "version": "1.4.2",
    "installed": true
  },
  {
    "type": "claude",
    "name": "Claude Code CLI",
    "path": "/home/dev/.local/bin/claude",
    "version": "2.1.302",
    "installed": true
  },
  {
    "type": "shell",
    "name": "System Shell (bash/sh)",
    "path": "/bin/bash",
    "version": "5.2.21(1)-release",
    "installed": true
  }
]
```

### 2.2. Deteksi Binary Agent via REST API
Klien frontend atau skrip otomasi dapat meminta daftar binary terpasang melalui endpoint publik:
```bash
curl -sSL https://cuan.dojo.cc/api/v1/agents/binaries
```

Response format:
```json
{
  "binaries": [
    {
      "type": "agy",
      "name": "Antigravity CLI",
      "path": "/home/dev/.local/bin/agy",
      "version": "1.4.2",
      "installed": true
    },
    {
      "type": "claude",
      "name": "Claude Code CLI",
      "path": "/home/dev/.local/bin/claude",
      "version": "2.1.302",
      "installed": true
    },
    {
      "type": "shell",
      "name": "System Shell (bash/sh)",
      "path": "/bin/bash",
      "version": "5.2.21(1)-release",
      "installed": true
    }
  ]
}
```

### 2.3. Melakukan Pairing Akun Agent CLI
Untuk menghubungkan agen CLI dengan token autentikasi atau label perangkat tertentu:

```bash
# Melalui CLI flag
./bin/cuan-server --pair-agent agy --agent-name "Dev Workstation Agent" --agent-token "agt_sample_token_xyz"
```

Atau melalui REST API:
```bash
curl -X POST https://cuan.dojo.cc/api/v1/agents/pair \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <AUTH_TOKEN>" \
  -d '{
    "agent_type": "agy",
    "device_name": "Production VPS Runner",
    "token": "agy_device_token_xyz123"
  }'
```

Sesi tersimpan secara persisten pada `~/.cuan/agent_sessions.json` dengan izin berkas terisolasi (`0600`).

---

## 3. PTY Task Runner Engine & Agent Dispatcher (S22.2)

### 3.1. Alur Kerja Eksekusi Task
Saat tombol **Run Agent** ditekan pada antarmuka web, frontend mengirimkan request:

```http
POST /api/v1/tasks/{taskId}/run-agent
Content-Type: application/json
Authorization: Bearer <JWT_TOKEN>

{
  "agent_type": "agy",
  "prompt": "Implement unit tests for session store"
}
```

Backend Cuan memproses permintaan ini melalui langkah-langkah berikut:
1. **Task Retrieval:** Memeriksa keberadaan task berdasarkan `taskId`.
2. **Assignee & Status Update:** Memperbarui kolom `execution_status = 'running'` dan mencatat waktu mulai.
3. **PTY Session Spawning:** Memanggil `manager.RunTaskAgent(taskID, agentType, prompt, workDir)`.
4. **Startup ANSI Banner:** Menginjeksikan banner informatif ke dalam stream PTY:
   ```
   ============================================================
      CUAN SELF-HOSTED AGENT RUNNER — SPRINT 22
      Task ID:    <task-id>
      Task Title: <task-title>
      Agent Type: agy
   ============================================================
   ```
5. **CLI Invocation with Intelligent Fallback:**
   - Jika binary `agy` tersedia: menjalankan `agy --auto` dengan instruksi task.
   - Jika `claude` dipilih dan tersedia: menjalankan `claude -p "<prompt>"`.
   - Jika binary tidak terpasang di host: secara anggun beralih ke sesi interaktif `/bin/bash` dengan instruksi tercetak di terminal.
6. **Exit Code & History Tracking:** Saat proses selesai, exit code dicatat. Jika `code == 0`, status task diperbarui menjadi `completed`; jika tidak, menjadi `error`. Seluruh ringkasan disimpan dan dapat diakses via `GET /api/v1/tasks/{taskId}/run-agent/history`.

---

## 4. Interactive Web Terminal (xterm.js) di Task SidePeek (S22.3)

### 4.1. Fitur Utama Terminal Web
- **Bidirectional WebSocket:** Menggunakan protokol WebSocket untuk pertukaran data dua arah berlatensi rendah antara browser dan pseudoterminal Linux (`creack/pty`).
- **Auto-Fit Resize:** Menggunakan `FitAddon` untuk menghitung ukuran karakter (kolom x baris) berdasarkan dimensi elemen DOM pembungkus, mengirimkan pesan resize JSON `{"type": "resize", "cols": C, "rows": R}` ke backend saat tab dibuka atau ukuran jendela berubah.
- **Link Clickability:** Menggunakan `WebLinksAddon` untuk mendeteksi URL di output terminal secara otomatis sehingga dapat diklik langsung.
- **ANSI Color Support:** Mendukung penuh kode warna ANSI 16 dan 256 warna, sesuai dengan palette Tokyo Night Dark (`#1a1b26`).
- **Quick Controls:**
  - **Run Agent Button:** Memulai eksekusi task agen dalam satu klik.
  - **Clear Screen Button:** Menghapus buffer layar terminal.
  - **Reconnect Button:** Menghubungkan ulang sesi WebSocket jika terjadi pemutusan koneksi jaringan.

---

## 5. Agent Assignment & Visual Status Indicators (S22.4)

### 5.1. Status Indikator di Papan Kanban
Pada [web/src/features/tasks/TaskBoardView.tsx](file:///home/dev/repos/cuan/web/src/features/tasks/TaskBoardView.tsx), setiap kartu tugas menampilkan:
- **Running:** Lingkaran hijau berkedip (*pulsing animation* `animate-ping`) dengan teks "Running", mengindikasikan proses agen sedang aktif di terminal.
- **Completed:** Badge hijau/teal mengindikasikan agen telah selesai mengeksekusi instruksi dengan exit code 0.
- **Failed / Error:** Badge merah mengindikasikan proses agen terhenti atau menghasilkan exit code bukan nol.
- **Idle:** Badge abu-abu mengindikasikan task belum dijalankan oleh runner.

### 5.2. Dropdown Penugasan di Task SidePeek
Di [web/src/features/tasks/TaskSidePeek.tsx](file:///home/dev/repos/cuan/web/src/features/tasks/TaskSidePeek.tsx), pengguna dapat memilih agen:
- **Antigravity Flash (Autonomous Worker)**
- **Antigravity Pro (Complex Refactor)**
- **Scrum Master (@sm)**
- **Human Owner (Buchin)**

Pilihan ini langsung memperbarui penanggung jawab (`assignee_id`) pada tugas tanpa perlu me-reload halaman.

---

## 6. Zero DB Migration Regression Guarantee (S22.5)

Sprint 22 mengadopsi prinsip ketat **Zero DB Migration Regression**:
- **Jumlah Migrasi:** Tepat 19 berkas migrasi PostgreSQL (`000001` hingga `000019`).
- **Pemanfaatan Kolom yang Ada:**
  - `assignee_id` (VARCHAR 255) — ditambahkan pada migrasi `000012_tasks_byos.up.sql`.
  - `execution_status` (VARCHAR 50) — ditambahkan pada migrasi `000014_agent_hub_terminal.up.sql`.
  - `agent_output` (TEXT) — ditambahkan pada migrasi `000015_project_tasks_agent_output.up.sql`.
- **Integritas Data:** Seluruh data operasional, tugas, proyek, dan riwayat pesan chat yang ada di database produksi tetap utuh tanpa modifikasi destruktif.

---

## 7. Panduan Pemecahan Masalah (Troubleshooting Guide)

### 7.1. Terminal Menampilkan "Connection closed" Sesaat Setelah Dibuka
- **Penyebab:** Konfigurasi Nginx memutus koneksi WebSocket karena timeout default 60 detik atau header upgrade tidak diteruskan.
- **Solusi:** Pastikan blok Nginx untuk endpoint terminal memuat konfigurasi WebSocket upgrade berikut:
  ```nginx
  location ~* ^/api/(v1/)?(projects/)?tasks/[^/]+/terminal {
      proxy_pass http://127.0.0.1:8080;
      proxy_http_version 1.1;
      proxy_set_header Upgrade $http_upgrade;
      proxy_set_header Connection "upgrade";
      proxy_set_header Host $host;
      proxy_read_timeout 86400s;
      proxy_send_timeout 86400s;
  }
  ```

### 7.2. Agen Tidak Ditemukan (`binary not found in PATH`)
- **Penyebab:** Binary `agy` atau `claude` terpasang di folder lokal user (seperti `~/.local/bin` atau `~/.cargo/bin`) yang tidak ada di `$PATH` saat `cuan.service` dijalankan oleh systemd.
- **Solusi:**
  1. Periksa path binary: `which agy` atau `which claude`.
  2. Buka konfigurasi systemd: `sudo nano /etc/systemd/system/cuan.service`.
  3. Tambahkan path ke `Environment="PATH=...:/home/dev/.local/bin"`.
  4. Jalankan `sudo systemctl daemon-reload && sudo systemctl restart cuan`.

### 7.3. Izin Akses PTY Bermasalah (`permission denied /dev/ptmx`)
- **Penyebab:** Pengguna yang menjalankan service tidak memiliki izin mengakses pseudoterminal devfs.
- **Solusi:** Pastikan service berjalan di bawah user yang tergabung dalam grup `tty`:
  ```bash
  sudo usermod -aG tty cuan
  ```

---

## 8. Prosedur Rilis & Verifikasi Produksi di VPS

1. **Kompilasi Binary Go:**
   ```bash
   go build -o bin/cuan-server ./cmd/server
   ```
2. **Kompilasi Frontend Web:**
   ```bash
   npm --prefix web run build
   ```
3. **Sinkronisasi Aset ke VPS (`cuan.dojo.cc`):**
   ```bash
   # Salin binary backend
   scp bin/cuan-server user@cuan.dojo.cc:/opt/cuan/bin/cuan-server
   
   # Salin bundle frontend dist
   rsync -avz --delete web/dist/ user@cuan.dojo.cc:/opt/cuan/web/dist/
   ```
4. **Restart Layanan:**
   ```bash
   ssh user@cuan.dojo.cc "sudo systemctl restart cuan"
   ```
5. **Smoke Test Produksi:**
   ```bash
   curl -sSL https://cuan.dojo.cc/healthz
   curl -sSL https://cuan.dojo.cc/api/v1/agents/binaries
   ```
   Keduanya harus mengembalikan status `200 OK` dengan payload JSON valid.
