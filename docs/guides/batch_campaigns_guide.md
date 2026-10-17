# Panduan Operasional Batch Campaigns Engine & Multi-Project Fan-Out Execution

Dokumen ini merupakan panduan operasional mandiri bagi pemilik instance (*Single-Owner*), pengembang mandiri (*Solo Developer*), agensi digital pengelola puluhan situs web, serta tim rekayasa perangkat lunak mengenai arsitektur, konfigurasi, alur kerja pembuatan kampanye massal (*Batch Campaigns*), pemecahan tugas otomatis (*Fan-Out Decompiler Engine*), eksekusi paralel terisolasi (*Multi-Project Isolated Runner*), pemantauan agregat terpusat (*Centralized Rollup Aggregator*), siaran langsung kemajuan (*SSE Streaming Engine*), serta pengoperasian produksi di domain kanonik `cuan.dojo.cc`.

---

## 1. Filosofi & Nilai Produk

Fitur **Batch Campaigns Engine** pada Cuan mengusung filosofi **Zero-Friction Mass Operations with Isolated Blast Radius**:
1. **One-Click Multi-Site Rollout:** Cukup 1 konfigurasi kampanye untuk mendistribusikan perubahan kode, injeksi skrip analitik/iklan (seperti Google AdSense Auto-Ads), atau pembaruan konfigurasi ke puluhan proyek/repositori sekaligus.
2. **Dynamic Target Resolution:** Target proyek dapat didefinisikan secara fleksibel menggunakan pencocokan kriteria tags (contoh: `tag:wordpress`, `tag:adsense`, `tag:agency-portfolio`) atau daftar ID proyek eksplisit.
3. **Guaranteed DoD Quality Gate:** Setiap subtask anak yang di-generate oleh decompiler otomatis dilengkapi minimal 3 butir kriteria penerimaan (*Definition of Done* / DoD) yang terukur dan dapat diverifikasi sebelum dinyatakan selesai.
4. **Controlled Concurrency & Fault Isolation:** Worker pool membatasi beban CPU dan I/O dengan kuota konkurensi terkontrol (default: 5 worker paralel). Kegagalan pada 1 situs tidak merusak, memblokir, atau membatalkan eksekusi pada proyek lain (*isolated blast radius*).
5. **Real-Time Visibility without Polling:** Agregator rollup menghitung metrik kemajuan secara live dan menyiarkannya via *Server-Sent Events* (SSE) lengkap dengan *heartbeat* 15 detik, sehingga UI menampilkan status real-time tanpa membebani server dengan polling HTTP berulang.
6. **100% Data Retention & Backward Compatibility:** Skema migrasi database PostgreSQL `000013_batch_campaigns.up.sql` bersifat murni aditif tanpa mengubah struktur tabel Sprint 1–7, menjamin 100% integritas data historis pengguna, proyek, tugas lawas, sponsor, deployment lokal, server BYOS, custom domain, dan observabilitas.

---

## 2. Diagram Arsitektur E2E Batch Campaigns Engine

Berikut adalah diagram alur kerja siklus hidup interaksi pengguna, decompiler fan-out, worker pool runner terisolasi, rollup aggregator, SSE broadcaster, dan reverse proxy produksi:

```d2
direction: right

operator: "Operator / Solo Dev\n(Campaign Studio UI)" {
  shape: person
}

cuan_control: "Cuan Control Plane (Go Backend)" {
  router: "Chi Router\n(/api/campaigns/*)"
  decompiler: "Fan-Out Decompiler Engine\n(Target Resolver & Task Generator)"
  runner_pool: "Multi-Project Isolated Runner\n(Worker Pool Concurrency Limit: 5)"
  aggregator: "Rollup Aggregator\n(Centralized Progress Metrics)"
  sse_stream: "SSE Streaming Engine\n(Real-Time Heartbeat & Broadcast)"
}

caddy_proxy: "Reverse Proxy & TLS\n(cuan.dojo.cc)" {
  ssl_gatekeeper: "Let's Encrypt / On-Demand TLS\n(/api/caddy/ask)"
  proxy_pass: "Nginx Reverse Proxy\n(Port 8080 Production)"
}

database: "PostgreSQL Database (000013)" {
  campaigns: "batch_campaigns\n(id, title, payload, criteria, status)"
  project_tasks: "project_tasks\n(parent_task_id, batch_id, dod_checklist)"
  projects: "projects\n(10 WordPress Dummy Sites)"
  historical: "Sprint 1-7 Tables\n(100% Data Retention)"
}

operator -> caddy_proxy.proxy_pass: "1. Create Batch Campaign (/api/campaigns)"
caddy_proxy.ssl_gatekeeper -> cuan_control.router: "2. Verify Authorized Domain (/api/caddy/ask)"
caddy_proxy.proxy_pass -> cuan_control.router: "3. Forward Request to Backend"
cuan_control.router -> database.campaigns: "4. Store Campaign Draft"
operator -> cuan_control.router: "5. Execute Campaign (/api/campaigns/{id}/execute)"
cuan_control.router -> cuan_control.decompiler: "6. Trigger Fan-Out Decompile"
cuan_control.decompiler -> database.projects: "7. Query Matching Sites (tag:wordpress)"
cuan_control.decompiler -> database.project_tasks: "8. Generate 10 Subtasks (DoD >= 3)"
cuan_control.decompiler -> cuan_control.runner_pool: "9. Enqueue Subtasks into Worker Pool"
cuan_control.runner_pool -> database.project_tasks: "10. Isolated Execution per Site (Doing -> Done)"
cuan_control.runner_pool -> cuan_control.aggregator: "11. Report Task Lifecycle Events"
cuan_control.aggregator -> cuan_control.sse_stream: "12. Emit Progress & Rollup Metrics"
cuan_control.sse_stream -> operator: "13. Push Live Progress Bar (10/10 100%)"
```

---

## 3. Struktur Database & Model Data (000013)

Tabel `batch_campaigns` didefinisikan secara aditif pada file migrasi `migrations/000013_batch_campaigns.up.sql`:

```sql
CREATE TABLE IF NOT EXISTS batch_campaigns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    template_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    target_criteria JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(50) NOT NULL DEFAULT 'draft',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_batch_campaigns_status ON batch_campaigns(status);
CREATE INDEX IF NOT EXISTS idx_batch_campaigns_created_at ON batch_campaigns(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_batch_campaigns_title ON batch_campaigns(title);
CREATE INDEX IF NOT EXISTS idx_batch_campaigns_template_payload ON batch_campaigns USING gin(template_payload);
CREATE INDEX IF NOT EXISTS idx_batch_campaigns_target_criteria ON batch_campaigns USING gin(target_criteria);
```

### Kolom Penting:
- `template_payload`: Menyimpan skrip/kode yang akan didistribusikan (`script_body`, `script_type`, `environment`, `dod_template`).
- `target_criteria`: Kriteria pemilihan situs/proyek (`tags: ["wordpress", "adsense"]`, `project_ids: [...]`, `exclude_project_ids: [...]`).
- `status`: Siklus status kampanye (`draft`, `running`, `completed`, `failed`, `cancelled`).

---

## 4. Panduan API & Eksekusi Operasional

### 4.1 Membuat Kampanye Batch Baru (`POST /api/campaigns`)

Gunakan endpoint ini untuk mendaftarkan draf kampanye:

```bash
curl -X POST https://cuan.dojo.cc/api/campaigns \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Mass Rollout AdSense Auto-Ads 2026 Script Update",
    "description": "Pembaruan script tag Google AdSense Auto-Ads ca-pub-9876543210 pada seluruh situs WordPress klien",
    "target_criteria": {
      "tags": ["wordpress"]
    },
    "template_payload": {
      "script_type": "inline_html",
      "placement": "head_prepend",
      "script_body": "<script async src=\"https://pagead2.googlesyndication.com/pagead/js/adsbygoogle.js?client=ca-pub-9876543210\" crossorigin=\"anonymous\"></script>",
      "dod_template": [
        "Backup file header.php dan functions.php sebelum injeksi skrip",
        "Injeksikan snippet AdSense ca-pub-9876543210 sebelum tag penutup </head>",
        "Verifikasi HTTP 200 OK dan validasi tag iklan ter-render pada DOM",
        "Audit Google Core Web Vitals memastikan zero layout shift (CLS < 0.1)"
      ]
    }
  }'
```

**Respons HTTP `201 Created`:**
```json
{
  "status": "success",
  "campaign": {
    "id": "c1f7a092-23c4-4b5f-8d2a-456789abcdef",
    "title": "Mass Rollout AdSense Auto-Ads 2026 Script Update",
    "status": "draft",
    "target_criteria": { "tags": ["wordpress"] },
    "created_at": "2026-10-17T07:15:00Z"
  }
}
```

### 4.2 Menjalankan Eksekusi Kampanye (`POST /api/campaigns/{id}/execute`)

Memicu decompiler untuk mengurai target proyek dan menjalankan worker pool secara paralel:

```bash
curl -X POST https://cuan.dojo.cc/api/campaigns/c1f7a092-23c4-4b5f-8d2a-456789abcdef/execute
```

**Respons HTTP `200 OK`:**
```json
{
  "status": "success",
  "campaign_id": "c1f7a092-23c4-4b5f-8d2a-456789abcdef",
  "total_tasks": 10,
  "completed_tasks": 10,
  "failed_tasks": 0,
  "peak_concurrency": 5,
  "duration_ms": 142,
  "campaign": {
    "status": "completed"
  }
}
```

### 4.3 Mengambil Metrik Agregat Rollup (`GET /api/campaigns/{id}/rollup`)

Mengambil ringkasan live penghitungan subtask anak:

```bash
curl -s https://cuan.dojo.cc/api/campaigns/c1f7a092-23c4-4b5f-8d2a-456789abcdef/rollup
```

**Respons JSON:**
```json
{
  "campaign_id": "c1f7a092-23c4-4b5f-8d2a-456789abcdef",
  "rollup": {
    "total_targets": 10,
    "completed": 10,
    "failed": 0,
    "running": 0,
    "pending": 0,
    "success_percentage": 100.0,
    "status": "completed"
  }
}
```

### 4.4 Berlangganan Live SSE Stream (`GET /api/campaigns/{id}/events`)

Membuka koneksi HTTP stream real-time dengan heartbeat 15 detik:

```bash
curl -N -H "Accept: text/event-stream" https://cuan.dojo.cc/api/campaigns/c1f7a092-23c4-4b5f-8d2a-456789abcdef/events
```

**Aliran Event:**
```text
event: connected
data: {"status":"connected","campaign_id":"c1f7a092-23c4-4b5f-8d2a-456789abcdef"}

event: campaign.progress
data: {"campaign_id":"c1f7a092-23c4-4b5f-8d2a-456789abcdef","total_targets":10,"completed":1,"failed":0,"running":5,"success_percentage":10.0,"status":"running"}

event: task.status_changed
data: {"task_id":"t-site-01","project_id":"wp-site-01","status":"done"}

event: campaign.completed
data: {"campaign_id":"c1f7a092-23c4-4b5f-8d2a-456789abcdef","total_tasks":10,"completed_tasks":10,"failed_tasks":0,"success":true}
```

---

## 5. Konfigurasi Produksi & Keamanan TLS

### 5.1 Systemd Service (`/etc/systemd/system/cuan.service`)
Layanan dijalankan sebagai daemon sistem mandiri:
```ini
[Unit]
Description=Cuan Production Server
After=network.target postgresql.service

[Service]
Type=simple
User=ubuntu
WorkingDirectory=/opt/cuan
EnvironmentFile=/opt/cuan/.env
ExecStart=/opt/cuan/bin/cuan-server
Restart=always
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

### 5.2 Nginx Reverse Proxy & Let's Encrypt SSL
File konfigurasi `/etc/nginx/sites-available/cuan.dojo.cc.conf` mengamankan lalu lintas web dengan SSL/TLS Let's Encrypt serta mematikan buffer proxy khusus untuk streaming SSE:

```nginx
server {
    server_name cuan.dojo.cc;
    client_max_body_size 100M;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # SSE streaming support (disable buffering)
        proxy_buffering off;
        proxy_cache off;
        proxy_read_timeout 86400s;
        proxy_send_timeout 86400s;
    }

    listen 443 ssl;
    ssl_certificate /etc/letsencrypt/live/cuan.dojo.cc/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/cuan.dojo.cc/privkey.pem;
    include /etc/letsencrypt/options-ssl-nginx.conf;
    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;
}

server {
    listen 80;
    server_name cuan.dojo.cc;
    return 301 https://$host$request_uri;
}
```

### 5.3 Gatekeeper Caddy On-Demand TLS (`GET /api/caddy/ask`)
Cuan menyediakan endpoint pengesahan sertifikat otomatis:
- `https://cuan.dojo.cc/api/caddy/ask?domain=cuan.dojo.cc` -> `200 OK` (`{"status":"authorized"}`).
- Domain asing/ilegal langsung ditolak dengan `403 Forbidden` guna melindungi kuota sertifikat dari *ACME rate-limit exhaustion*.

---

## 6. Prosedur Pemeliharaan & Troubleshooting

1. **Memeriksa Status Layanan:**
   ```bash
   sudo systemctl status cuan
   ```
2. **Melihat Log Real-Time:**
   ```bash
   sudo journalctl -u cuan -f
   ```
3. **Pemeriksaan Health Check & Readiness:**
   ```bash
   curl -s https://cuan.dojo.cc/healthz
   curl -s https://cuan.dojo.cc/readyz
   ```
4. **Restart Layanan:**
   ```bash
   sudo systemctl restart cuan
   ```
