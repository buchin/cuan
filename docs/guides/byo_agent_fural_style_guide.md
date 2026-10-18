# Panduan Operasional: Fural-Style BYO Agent Connection & Dynamic Chat Roles

Dokumen ini merupakan panduan teknis dan operasional untuk arsitektur Fural-Style Bring Your Own (BYO) Agent dan Dynamic Contextual Chat Roles pada platform Cuan.

---

## 1. Arsitektur Fural-Style BYO Agent Connection

Arsitektur Fural-Style BYO Agent di Cuan mengeliminasi dependensi peran bisnis statis. Agen didefinisikan secara murni oleh identitas (@handle), Harness eksekusi (Antigravity, Claude, Codex, atau Custom Model), dan Sesi Autentikasi.

### Diagram Arsitektur Komponen (D2 TALA Engine)

```d2
direction: right

PoUser: "Product Owner / User\n(Buchin)" {
  shape: person
  style.fill: "#1e293b"
  style.font-color: "#f8fafc"
}

WebUI: "Cuan Web Interface\n(React 18 + Vite + Tailwind)" {
  AddModal: "Fural-Style Add Modal\n(@handle + Harness)"
  ChatHub: "Unified Chat Hub\n(Dynamic Context Persona)"
  TaskBoard: "Global Task Board\n(Agent Runner + xterm.js)"
}

Backend: "Cuan Control Plane (Go REST/WS Engine)" {
  AgentRegistry: "Agent Registry API\n(Format Fural: {id, name, harness})"
  ContextClassifier: "Dynamic Context Classifier\n(Scrum Master | Dev | Friendly)"
  PtyRunner: "PTY Process Manager\n(Task Runner Engine)"
}

HarnessRuntimes: "BYO Agent Harness Runtimes" {
  AGY: "Antigravity CLI (agy)\nAutonomous Loop"
  Claude: "Claude Code CLI\nAnthropic Runtime"
  Codex: "Codex CLI\nOpenAI / Custom Harness"
  Shell: "Interactive PTY Shell\nBash / Sh Fallback"
}

Storage: "Storage & State Layer" {
  PostgreSQL: "PostgreSQL DB\n(Strictly 19 Migrations)"
  Workspace: "Task Workspace Dir\n(/tmp/cuan/workspaces)"
}

PoUser -> WebUI.AddModal: "1. Daftarkan BYO Agent (@handle)"
PoUser -> WebUI.ChatHub: "2. Chat & Koordinasi (@mention)"
PoUser -> WebUI.TaskBoard: "3. Jalankan Task Runner"

WebUI.AddModal -> Backend.AgentRegistry: "POST /api/v1/agents"
WebUI.ChatHub -> Backend.ContextClassifier: "Kirim Pesan Obrolan"
WebUI.TaskBoard -> Backend.PtyRunner: "POST /api/v1/tasks/{id}/run-agent"

Backend.AgentRegistry -> Storage.PostgreSQL: "Simpan Registry Agen"
Backend.ContextClassifier -> Backend.PtyRunner: "Dispatch Persona Prompt"

Backend.PtyRunner -> HarnessRuntimes.AGY: "Spawn agy -p"
Backend.PtyRunner -> HarnessRuntimes.Claude: "Spawn claude -p"
Backend.PtyRunner -> HarnessRuntimes.Codex: "Spawn codex -p"
Backend.PtyRunner -> HarnessRuntimes.Shell: "Spawn interactive shell"

Backend.PtyRunner -> Storage.Workspace: "Binds PTY to Task Dir"
```

---

## 2. Eliminasi Peran Kaku & Standarisasi Registry Fural

Pada arsitektur lama, agen memiliki peran statis yang dikunci pada profil (seperti "CS Toko", "Admin", atau "Lead Architect"). Pada Sprint 23, konsep peran kaku dieliminasi sepenuhnya:

- **Format Entitas Fural Standar:**
  ```json
  {
    "id": "c053475c-0513-435b-9bb2-b548ab5b0068",
    "name": "@dev",
    "harness": "agy",
    "connection_id": "conn_dev_session_123",
    "enabled": true
  }
  ```
- **Endpoints REST API Standar:**
  - `GET /api/v1/agents`: Mengambil seluruh agen terdaftar.
  - `POST /api/v1/agents`: Mendaftarkan agen baru format Fural.
  - `GET /api/v1/agents/{id}`: Detail agen berdasarkan ID.
  - `PATCH /api/v1/agents/{id}`: Memperbarui parsial atribut agen (nama, harness, status enabled).
  - `DELETE /api/v1/agents/{id}`: Menghapus agen dari sistem.

---

## 3. Modal Tambah Agen & Pilihan Harness

Modal pendaftaran agen (`web/src/features/team/TeamView.tsx`) didesain ulang mengadopsi standar UX Fural:

1. **Pilihan Harness:**
   - **Antigravity (`agy`)**: Runner CLI otonom cerdas untuk pemecahan masalah end-to-end.
   - **Claude (`claude`)**: Runner Anthropic Claude Code untuk refactoring dan arsitektur kode.
   - **Codex (`codex`)**: Runner model Codex/OpenAI untuk script generation dan perbaikan cepat.
   - **Custom Model (`custom`)**: Konfigurasi endpoint & API key kustom untuk model mandiri.
2. **Mention Handle (`@handle`):**
   - Input handle otomatis diberi awalan `@`.
   - Sanitasi otomatis terhadap spasi dan karakter non-alphanumeric.
3. **Panel Autentikasi / Sesi Akun:**
   - Menampilkan status pairing akun CLI tanpa perlu mengekspos token mentah (zero plaintext leakage).

---

## 4. Dynamic Contextual Chat Persona

Setiap agen tidak lagi terikat pada satu peran tetap saat diajak berinteraksi. Sistem secara dinamis mengklasifikasikan intent dan konteks pesan ke dalam 3 persona utama:

```d2
direction: down

IncomingMsg: "Pesan Masuk dari Pengguna\n(Direct Room / @mention)" {
  shape: oval
  style.fill: "#334155"
  style.font-color: "#ffffff"
}

Classifier: "Context Classifier\n(internal/server/chat_handlers.go)" {
  shape: diamond
  style.fill: "#0284c7"
  style.font-color: "#ffffff"
}

RoleSM: "Persona: Scrum Master\n(Koordinasi Sprint, Backlog, Review, DoD)" {
  style.fill: "#10b981"
  style.font-color: "#ffffff"
}

RoleDev: "Persona: Developer\n(Teknis, Coding, Bugfix, Refactor, Unit Test)" {
  style.fill: "#6366f1"
  style.font-color: "#ffffff"
}

RoleFriend: "Persona: Friendly Coworker\n(Sapaan Natural, Status, Ramah)" {
  style.fill: "#f59e0b"
  style.font-color: "#ffffff"
}

IncomingMsg -> Classifier: "Analisis Keyword & Semantik"
Classifier -> RoleSM: "Sprint / Review / Evaluasi / DoD"
Classifier -> RoleDev: "Ngoding / Fix / Endpoint / Test"
Classifier -> RoleFriend: "Halo / Kabar / Sapaan Umum"
```

1. **Role Scrum Master**:
   - *Pemicu*: Keyword `sprint`, `planning`, `review`, `evaluasi`, `backlog`, `dod`, `velocity`, `standup`.
   - *Respon*: Membedah scope pekerjaan, mengevaluasi backlog, mengukur Definition of Done (DoD), dan memfasilitasi alur kerja tim.
2. **Role Developer**:
   - *Pemicu*: Keyword `teknis`, `ngoding`, `fix`, `bug`, `error`, `endpoint`, `api`, `database`, `query`, `refactor`, `unit test`.
   - *Respon*: Fokus pada implementasi kode, analisa root cause bug, clean code architecture, dan automated test pass.
3. **Role Friendly**:
   - *Pemicu*: Keyword `halo`, `hai`, `apa kabar`, `pagi`, `terima kasih`, `siap`.
   - *Respon*: Percakapan natural, hangat, ringkas, dan responsif sebagai rekan kerja AI.

---

## 5. Integrasi PTY Task Runner dengan BYO Agent

- Pemilihan agen penanggung jawab (Assignee) di Papan Kanban (`TaskBoardView.tsx`) dan panel inspeksi (`TaskSidePeek.tsx`) langsung terhubung dengan ID agen BYO yang tersimpan.
- Saat tombol **Jalankan Worker Agent** ditekan:
  - Backend memetakan harness agen (`agy`, `claude`, `codex`, atau `shell`).
  - Terminal PTY diinisialisasi dan dieksekusi di folder workspace task terkait (`/tmp/cuan/workspaces/task-xxx`).
  - Frontend menampilkan live streaming interaktif terminal xterm.js dengan indikator status animasi (pulsing green dot saat `running`, hijau saat `completed`, merah saat `failed`).

---

## 6. Jaminan Integritas Database (Zero DB Regression)

- Skema database dipertahankan secara non-destruktif dengan **strictly 19 PostgreSQL migrations** (`000001_initial_schema` s.d. `000019_studio_project_tasks`).
- Tidak ada file migrasi baru (`000020`) yang dibuat.
- Pemetaan entitas agen memanfaatkan kolom `provider` untuk harness, `model_id` untuk connection ID, dan `status` untuk status aktif/enabled.

---

## 7. Prosedur Operasional & Verifikasi di Host VPS cuan.dojo.cc

1. **Pengecekan Layanan Systemd:**
   ```bash
   sudo systemctl status cuan.service
   ```
2. **Restart Layanan Pasca Rilis:**
   ```bash
   sudo systemctl restart cuan.service
   ```
3. **Smoke Test Endpoint:**
   - Health Check: `curl -I https://cuan.dojo.cc/healthz` (Harus HTTP 200 OK)
   - Readyness Check: `curl -s https://cuan.dojo.cc/readyz` (Harus `status: ready`, migrations: 19)
   - Info Check: `curl -s https://cuan.dojo.cc/api/info`
   - Agent Registry: `curl -s https://cuan.dojo.cc/api/v1/agents`
