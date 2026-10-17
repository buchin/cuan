# Panduan Operasional Native Agent Hub & In-Browser Interactive Terminal (PTY Engine)

Dokumen ini merupakan panduan operasional mandiri bagi pemilik instance (*Single-Owner*), pengembang mandiri (*Solo Developer*), operator cloud studio, serta tim rekayasa perangkat lunak mengenai arsitektur, konfigurasi, alur kerja pendaftaran agen AI (*Native Agent Hub*), integrasi model Google Gemini Pro (BYOK - *Bring Your Own Key*), enkripsi kredensial di sisi server (*KeyVault AES-256-GCM*), pengoperasian terminal interaktif di browser (*In-Browser Interactive Terminal PTY Engine* berbasis `github.com/creack/pty` dan `@xterm/xterm`), konfigurasi *WebSocket Upgrade Forwarding* pada Nginx reverse proxy, serta pengoperasian produksi di domain `cuan.dojo.cc`.

---

## 1. Filosofi & Nilai Produk

Fitur **Native Agent Hub & In-Browser Interactive Terminal** pada Cuan mengusung filosofi **Zero-Local-Footprint Cloud Engineering**:
1. **Develop Cuan Pake Cuan 100% In-Browser:** Pengembang dapat membuat task, menugaskan agen AI, memantau pengerjaan kode secara visual, dan mengambil alih terminal langsung dari browser di perangkat apa saja (laptop, tablet, iPad, Chromebook) tanpa perlu memasang Git, Go compiler, Docker, SSH key, atau database lokal.
2. **Bring-Your-Own-Key (BYOK) Google Gemini Pro:** Pengguna menghubungkan akun atau API key Google Gemini Pro milik sendiri secara mandiri. Cuan tidak melakukan markup biaya token atau mewajibkan sentralisasi billing.
3. **Zero Credential Leakage:** Kredensial API key LLM dienkripsi secara simetris menggunakan algoritma AES-256-GCM via `internal/crypto/vault.go` sebelum disimpan di tabel `agent_connections`. DTO publik hanya mengembalikan flag `has_credentials: true` tanpa mengekspos token rahasia ke frontend atau berkas log.
4. **Dual Operating Terminal Modes:**
   - **Watch Mode:** Terminal menampilkan stream stdout/stderr real-time saat agen AI sedang menyusun atau menguji kode (*read-only ANSI stream*).
   - **Interactive Take-Over Mode:** Pengembang dapat langsung mengintervensi sesi terminal menggunakan keyboard browser, menjalankan perintah shell bebas (`ls -la`, `go test ./...`, `git status`), atau mengedit file langsung dengan editor terminal seperti nano/vim.
5. **Fluid PTY Session with Dynamic Window Resizing:** Backend PTY engine berbasis `creack/pty` merespons pesan JSON resize dari xterm.js (`{"type": "resize", "cols": 80, "rows": 24}`) sehingga output terminal menyesuaikan dimensi viewport browser pengguna tanpa glitch wrapping ANSI.
6. **100% Data Retention Guarantee:** Skema database migrasi PostgreSQL `000014_agent_hub_terminal.up.sql` bersifat murni aditif menambahkan tabel `agents` dan `agent_connections` tanpa mengubah atau merusak seluruh data historis Sprint 1 hingga Sprint 8 (pemilik, proyek, tugas lawas, sponsor, deployment lokal, server remote BYOS, custom domain, observabilitas, room diskusi, dan batch campaign).

---

## 2. Diagram Arsitektur E2E Agent Hub & Terminal Engine

Berikut adalah diagram alur kerja siklus hidup interaksi pengguna, studio web UI, Nginx reverse proxy dengan WebSocket upgrade, Cuan control plane, KeyVault, PTY shell session terikat task worktree, dan Google Gemini Cloud API:

```d2
direction: right

user: "Developer / Operator (Web Browser)" {
  ui_studio: "Cuan Studio UI" {
    agent_hub: "Agent Hub View (/studio/agents)"
    discussion_rooms: "Discussion Rooms (@mention)"
    task_terminal: "Task Terminal (Xterm.js)"
  }
}

reverse_proxy: "Reverse Proxy & TLS (cuan.dojo.cc)" {
  nginx_proxy: "Nginx Gateway (Port 443 SSL)"
  ws_forwarder: "WebSocket Upgrade Forwarder (/api/tasks/*/terminal)"
}

cuan_control: "Cuan Control Plane (Go Backend Port 8080)" {
  router: "Chi Router (/api/v1/*)"
  agent_service: "Agent & Connection Service"
  ws_pty_engine: "WebSocket PTY Engine (creack/pty)"
  vault: "KeyVault (AES-256-GCM Encryption)"
}

cloud_ai: "AI Providers (Cloud)" {
  gemini_api: "Google Gemini Pro API"
}

database: "PostgreSQL Database (000014)" {
  agents_table: "agents"
  conn_table: "agent_connections"
  historical_tables: "Sprint 1-8 Tables (100% Data Retention)"
}

task_workspace: "Task Workspace (Git Worktree)" {
  shell_session: "PTY Shell Session (bash / sh)"
}

user.ui_studio.agent_hub -> reverse_proxy.nginx_proxy: "1. Manage Agents & Set Gemini API Key"
reverse_proxy.nginx_proxy -> cuan_control.router: "2. Forward REST API Calls"
cuan_control.router -> cuan_control.agent_service: "3. Route Agent Request"
cuan_control.agent_service -> cuan_control.vault: "4. Encrypt API Key (AES-256-GCM)"
cuan_control.agent_service -> database.conn_table: "5. Store Encrypted Connection"
cuan_control.agent_service -> database.agents_table: "6. Store Agent Personas"
cuan_control.agent_service -> cloud_ai.gemini_api: "7. Verify Quota & Model Status"

user.ui_studio.task_terminal -> reverse_proxy.ws_forwarder: "8. Connect WebSocket (WSS /api/tasks/:id/terminal)"
reverse_proxy.ws_forwarder -> cuan_control.ws_pty_engine: "9. Upgrade & Forward Connection"
cuan_control.ws_pty_engine -> task_workspace.shell_session: "10. Spawn & Bind creack/pty"
task_workspace.shell_session -> cuan_control.ws_pty_engine: "11. Stream Output (ANSI Escape Codes)"
cuan_control.ws_pty_engine -> user.ui_studio.task_terminal: "12. Render Interactive Terminal (Watch / Take-Over)"
```

---

## 3. Struktur Database & Model Data (000014)

Skema database migrasi `migrations/000014_agent_hub_terminal.up.sql` mendefinisikan tabel `agents` dan `agent_connections`:

```sql
CREATE TABLE IF NOT EXISTS agents (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    avatar VARCHAR(512) NOT NULL DEFAULT '',
    role VARCHAR(128) NOT NULL DEFAULT '',
    system_prompt TEXT NOT NULL DEFAULT '',
    model_id VARCHAR(128) NOT NULL DEFAULT 'gemini-1.5-pro',
    provider VARCHAR(64) NOT NULL DEFAULT 'google',
    status VARCHAR(64) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);
CREATE INDEX IF NOT EXISTS idx_agents_provider ON agents(provider);

CREATE TABLE IF NOT EXISTS agent_connections (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    provider VARCHAR(64) NOT NULL DEFAULT 'google',
    credentials_encrypted TEXT NOT NULL DEFAULT '',
    plan_hint VARCHAR(128) NOT NULL DEFAULT '',
    status VARCHAR(64) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_agent_connections_user_id ON agent_connections(user_id);
CREATE INDEX IF NOT EXISTS idx_agent_connections_provider ON agent_connections(provider);
```

### Penjelasan Kolom Kunci:
- **`agents`**:
  - `model_id`: Model AI yang digunakan (misal `gemini-1.5-pro`, `gemini-2.0-flash`).
  - `system_prompt`: Arahan sistem (*system instructions*) untuk membentuk persona agen (contoh: "Kamu adalah Senior Go Developer yang teliti dalam penulisan unit test").
  - `status`: Status aktif persona (`active`, `inactive`, `paused`).
- **`agent_connections`**:
  - `credentials_encrypted`: Ciphertext JSON terenkripsi AES-256-GCM berisi API key Google Gemini.
  - `plan_hint`: Indikator tier kuota pengguna (misal: `pay-as-you-go`, `free-tier`, `pro`).
  - `status`: Status konektivitas (`active`, `invalid`, `quota_exceeded`).

---

## 4. Panduan API & Eksekusi Operasional

### 4.1 Registrasi & Manajemen Persona Agen (`/api/v1/agents`)

#### Membuat Agen Baru (`POST /api/v1/agents` atau `POST /api/agents`):
```bash
curl -X POST https://cuan.dojo.cc/api/v1/agents \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Gemini Architect",
    "avatar": "https://api.dicebear.com/7.x/bottts/svg?seed=architect",
    "role": "Cloud Architecture & Testing Specialist",
    "system_prompt": "Kamu adalah staf ahli arsitektur cloud dan unit testing Go.",
    "model_id": "gemini-1.5-pro",
    "provider": "google"
  }'
```

Respons (201 Created):
```json
{
  "agent": {
    "id": "agent_9a8b7c6d5e",
    "name": "Gemini Architect",
    "avatar": "https://api.dicebear.com/7.x/bottts/svg?seed=architect",
    "role": "Cloud Architecture & Testing Specialist",
    "system_prompt": "Kamu adalah staf ahli arsitektur cloud dan unit testing Go.",
    "model_id": "gemini-1.5-pro",
    "provider": "google",
    "status": "active",
    "created_at": "2026-10-17T15:30:00Z",
    "updated_at": "2026-10-17T15:30:00Z"
  }
}
```

#### Menampilkan Daftar Agen (`GET /api/v1/agents` atau `GET /api/agents`):
```bash
curl -s https://cuan.dojo.cc/api/v1/agents
```

---

### 4.2 Menghubungkan Akun Google Gemini Pro (`/api/v1/agent-connections`)

#### Membuat Koneksi Akun Baru (`POST /api/v1/agent-connections`):
```bash
curl -X POST https://cuan.dojo.cc/api/v1/agent-connections \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "usr_master_admin",
    "provider": "google",
    "api_key": "AIzaSyFakePlaceholderKeyForVerification_NoPlaintextLeakage",
    "plan_hint": "google-ai-studio-payg"
  }'
```

Respons Sanitasi DTO (201 Created):
```json
{
  "connection": {
    "id": "conn_1a2b3c4d5e",
    "user_id": "usr_master_admin",
    "provider": "google",
    "has_credentials": true,
    "plan_hint": "google-ai-studio-payg",
    "status": "active",
    "created_at": "2026-10-17T15:31:00Z",
    "updated_at": "2026-10-17T15:31:00Z"
  }
}
```
*(Perhatikan bahwa `api_key` tidak pernah dimunculkan kembali pada respons JSON!)*

#### Verifikasi / Probe Status Koneksi (`POST /api/v1/agent-connections/{id}/verify`):
```bash
curl -X POST https://cuan.dojo.cc/api/v1/agent-connections/conn_1a2b3c4d5e/verify
```

Respons (200 OK):
```json
{
  "status": "active",
  "provider": "google",
  "verified_at": "2026-10-17T15:32:00Z",
  "models_available": ["gemini-1.5-pro", "gemini-2.0-flash"],
  "quota_healthy": true
}
```

---

### 4.3 WebSocket Pseudo-Terminal (PTY) Engine (`/api/tasks/{id}/terminal`)

#### Endpoint Protokol:
- **URL WebSocket:** `wss://cuan.dojo.cc/api/tasks/{task_id}/terminal`
- **Fallback URL:** `wss://cuan.dojo.cc/api/v1/tasks/{task_id}/terminal`
- **Subprotocol:** Standard WebSocket Binary/Text Frame.

#### Format Pesan Input Pengguna (Client -> Server):
1. **Raw Stdin Stream (Keystrokes):**
   Karakter atau byte input keyboard yang diketik pengguna dikirimkan sebagai text/binary frame langsung ke stdin PTY:
   ```text
   ls -la\n
   ```
2. **Terminal Window Resize Event:**
   Saat window browser atau ukuran container xterm.js berubah, client mengirimkan payload JSON:
   ```json
   {
     "type": "resize",
     "cols": 120,
     "rows": 36
   }
   ```

#### Format Output Terminal (Server -> Client):
Server mengalirkan bytes output ANSI sequence langsung dari proses PTY ke WebSocket client. xterm.js me-render teks dengan warna, styling ANSI, dan kursor interaktif.

#### Menutup Sesi Terminal Secara Bersih (`DELETE /api/tasks/{id}/terminal`):
```bash
curl -X DELETE https://cuan.dojo.cc/api/tasks/task-001/terminal
```
Respons (200 OK):
```json
{
  "status": "closed",
  "task_id": "task-001"
}
```

---

## 5. Konfigurasi Nginx Reverse Proxy (WebSocket Upgrade)

Agar stream WebSocket PTY dapat melewati reverse proxy Nginx pada host `DOJO_TERMAPI_VPS` (`cuan.dojo.cc`) tanpa terputus, konfigurasi reverse proxy dilengkapi direktif `Upgrade` dan `Connection "upgrade"` dengan batas timeout 24 jam:

```nginx
# /etc/nginx/sites-available/cuan.dojo.cc.conf

server {
    server_name cuan.dojo.cc;
    client_max_body_size 100M;

    # Dedicated WebSocket Upgrade location block for PTY Terminal
    location ~* ^/api/(v1/)?(projects/)?tasks/[^/]+/terminal {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Disable buffering and set 24h timeouts for persistent interactive sessions
        proxy_buffering off;
        proxy_cache off;
        proxy_read_timeout 86400s;
        proxy_send_timeout 86400s;
    }

    # Default application routing
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        proxy_buffering off;
        proxy_cache off;
        proxy_read_timeout 86400s;
        proxy_send_timeout 86400s;
    }

    listen [::]:443 ssl;
    listen 443 ssl;
    ssl_certificate /etc/letsencrypt/live/cuan.dojo.cc/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/cuan.dojo.cc/privkey.pem;
    include /etc/letsencrypt/options-ssl-nginx.conf;
    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;
}

server {
    listen 80;
    listen [::]:80;
    server_name cuan.dojo.cc;
    return 301 https://$host$request_uri;
}
```

Uji dan reload Nginx:
```bash
sudo nginx -t
sudo systemctl reload nginx
```

---

## 6. Prosedur Rilis & Verifikasi Kesehatan Sistem

### 6.1 Pengecekan Kesehatan (*Liveness & Readiness*)
```bash
# Liveness probe
curl -s https://cuan.dojo.cc/healthz
# {"app":"cuan","status":"healthy","version":"0.1.0"}

# Readiness probe post-migration 000014
curl -s https://cuan.dojo.cc/readyz
# {"database":"connected","migrations":"applied","migrations_count":14,"owner_configured":false,"status":"ready"}
```

### 6.2 Verifikasi Sesi Terminal Aktif
```bash
curl -s https://cuan.dojo.cc/api/terminal/sessions
# {"active_sessions":0,"total_sessions_spawned":1,"sessions":[]}
```

---

## 7. Troubleshooting & FAQ Operasional

| Gejala | Penyebab Umum | Tindakan Mitigasi |
| --- | --- | --- |
| Terminal menampilkan `WebSocket connection failed: 400 Bad Request` | Reverse proxy tidak meneruskan header `Upgrade` | Pastikan blok `location ~* ^/api/(v1/)?(projects/)?tasks/[^/]+/terminal` memuat `proxy_set_header Upgrade $http_upgrade;` dan `proxy_set_header Connection "upgrade";` |
| Sesi terminal terputus setiap 60 detik secara tiba-tiba | Timeout proxy Nginx default (60s) | Tambahkan `proxy_read_timeout 86400s;` dan `proxy_send_timeout 86400s;` pada konfigurasi Nginx |
| Teks output berantakan saat resize window browser | Paket JSON resize tidak terkirim atau gagal diproses | Pastikan xterm.js menggunakan `@xterm/addon-fit` dan mengirim event resize saat window berubah ukuran |
| API key Gemini gagal diverifikasi | Kuota API key habis atau key dibatasi oleh Google Cloud Console | Masuk ke Google AI Studio, periksa apakah API key memiliki akses ke `gemini-1.5-pro` atau `gemini-2.0-flash` |
| Database migration gagal | Koneksi PostgreSQL terputus | Jalankan manual `./bin/cuan-migrate --direction up` atau periksa `systemctl status cuan` |
