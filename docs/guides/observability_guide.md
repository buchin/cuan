# Panduan Operasional Observabilitas, Health Probes & Incident Alerting Hub

Dokumen ini merupakan panduan operasional komprehensif bagi pemilik instance (*Single-Owner*), pengembang mandiri (*Solo Developer*), agensi digital, dan Indie Hacker mengenai arsitektur, konfigurasi *Health Check Probes*, pemantauan telemetri *Resource Metrics* (Local & BYOS VPS), integrasi saluran notifikasi darurat (*Multi-Channel Alerting* via Telegram, Discord, dan Generic HMAC Webhook), pencegahan spam notifikasi (*Anti-Spam & Deduplication*), serta prosedur mitigasi insiden (*Downtime Mitigation Playbook*) pada platform Cuan.

---

## 1. Filosofi & Gambaran Umum

Fitur Observabilitas pada Cuan dibangun berdasarkan prinsip **Zero-Agent & Low-Overhead**:
1. **Zero-Agent Architecture:** Tidak mewajibkan instalasi daemon/agent pihak ketiga yang berat (seperti Datadog, New Relic, Prometheus Node Exporter, atau Telegraf) pada server target. Pemantauan ketersediaan dilakukan melalui HTTP/HTTPS health probe ringan dari control plane Cuan, dan telemetri hardware server dikumpulkan melalui eksekusi perintah non-invasif via koneksi SSH terenkripsi.
2. **Everyday Communication Tools:** Mengirimkan sinyal darurat langsung ke ruang obrolan kerja harian pengembang (Telegram Bot & Discord Webhook) serta Webhook kustom terotentikasi HMAC SHA256 untuk integrasi otomasi (Zapier, Make, n8n, atau webhook receiver internal).
3. **Intelligent Anti-Spam & Anti-Flapping:** Dilengkapi *Streak Failure Evaluator* (hanya memicu outage jika kegagalan terjadi minimal 3 kali berturut-turut), *Cooldown Window* (pembungkaman alert duplikat selama 10 menit), dan pendeteksian otomatis pemulihan (*Auto-Recovery Transition*) dengan notifikasi resolusi hijau.
4. **Security At-Rest via AES-GCM:** Seluruh token bot Telegram, URL Discord Webhook, dan rahasia HMAC dienkripsi at-rest menggunakan standar AES-GCM 256-bit (`internal/crypto`), serta selalu disanitasi/disamarkan saat disajikan ke UI Studio.
5. **100% Data Retention Guarantee:** Skema database PostgreSQL murni aditif (`000011_observability_alerts.up.sql`) dengan *Foreign Key Cascading* (`ON DELETE CASCADE`), memastikan seluruh data Sprint 1 (akun owner, task board, attempt), Sprint 2 (sponsors, leads, orders), Sprint 3 (local deployments, logs), Sprint 4 (BYOS servers), dan Sprint 5 (custom domains, SSL records) tetap 100% utuh tanpa alterasi.

---

## 2. Diagram Arsitektur E2E (E2E Architecture)

Berikut adalah diagram alur kerja siklus hidup pemantauan, probing, pengumpulan metrik, dan pengiriman notifikasi alert pada Cuan:

```d2
direction: right

client: "Browser / Pengguna (Studio Dashboard)" {
  shape: person
}

cuan_control_plane: "Cuan Control Plane (Go Backend)" {
  api: "Chi HTTP Router\n(/api/deployments/{id}/health,\n/api/servers/{id}/metrics,\n/api/alert-channels & /api/incidents)"
  health_engine: "Health Check Engine\n(HTTP Prober, Latency & Failure Streak Tracker)"
  metrics_collector: "Resource Metrics Collector\n(Local /proc Reader & BYOS SSH Runner)"
  alert_dispatcher: "Incident Alert Dispatcher\n(Threshold Evaluator, Deduplication & Cooldown)"
}

database: "PostgreSQL Database" {
  health_checks: "health_checks (000011)"
  server_metrics: "server_metrics (000011)"
  alert_channels: "alert_channels (000011)"
  alert_incidents: "alert_incidents (000011)"
}

monitored_targets: "Monitored Targets" {
  local_apps: "Local Sandbox Deployments\n(HTTP/HTTPS Endpoints)"
  byos_servers: "BYOS Remote VPS Nodes\n(SSH Telemetry Runner)"
}

notification_destinations: "Alert Destinations" {
  telegram: "Telegram Bot API\n(Markdown Styled Alert)"
  discord: "Discord Channel Webhook\n(Rich Embed Cards: Red/Green)"
  generic_webhook: "Generic HTTP Webhook\n(HMAC-SHA256 Signed JSON)"
}

client -> cuan_control_plane.api: "1. View Metrics & Configure Alert Channels"
cuan_control_plane.health_engine -> monitored_targets.local_apps: "2. Periodic HTTP Probe (Default: 30s)"
cuan_control_plane.metrics_collector -> monitored_targets.byos_servers: "3. Query CPU, RAM, Disk (Default: 60s)"
cuan_control_plane.health_engine -> database.health_checks: "4. Store Health Status & Latency History"
cuan_control_plane.metrics_collector -> database.server_metrics: "5. Store Resource Telemetry Samples"
cuan_control_plane.health_engine -> cuan_control_plane.alert_dispatcher: "6. Evaluate Outage / Recovery State"
cuan_control_plane.alert_dispatcher -> database.alert_incidents: "7. Track Incident Record (firing / resolved)"
cuan_control_plane.alert_dispatcher -> notification_destinations.telegram: "8a. Dispatch Telegram Alert"
cuan_control_plane.alert_dispatcher -> notification_destinations.discord: "8b. Dispatch Discord Embed"
cuan_control_plane.alert_dispatcher -> notification_destinations.generic_webhook: "8c. POST HMAC-Signed JSON"
```

---

## 3. Konfigurasi & Mekanisme Health Probes

### 3.1 Siklus Status Kesehatan (Health States)
Health Check Engine mengevaluasi status ketersediaan deployment berdasarkan kode status HTTP, latensi respons, dan jumlah kegagalan berturut-turut (*consecutive failures*):

| Status | Syarat / Kondisi | Indikator Visual | Tindakan Otomatis |
| :--- | :--- | :--- | :--- |
| **`healthy`** | Respons HTTP 2xx/3xx, latensi $\le$ 2000ms, *consecutive failures* = 0 | Badge Hijau | Jika ada insiden aktif `firing`, otomatis transisi ke `resolved` dan kirim notifikasi pemulihan. |
| **`degraded`** | Latensi > 2000ms **ATAU** kegagalan HTTP (4xx/5xx/timeout) terjadi 1–2 kali berturut-turut | Badge Kuning | Belum memicu alert darurat (menghindari alarm palsu akibat *network spike* sesaat). |
| **`unhealthy`** | Kegagalan HTTP (5xx / error jaringan / connection timeout) terjadi $\ge$ 3 kali berturut-turut | Badge Merah | Memicu insiden baru berstatus `firing` dan menembakkan alert darurat ke seluruh saluran aktif. |

### 3.2 Probing On-Demand vs Background Poller
- **Background Poller:** Berjalan otomatis setiap 30 detik untuk seluruh deployment yang berstatus `running`.
- **On-Demand Probe (`check-now`):** Pengembang dapat memicu pemeriksaan kesehatan seketika melalui tombol "Check Now" di Studio Dashboard atau via API:
  ```bash
  curl -X POST https://cuan.dojo.cc/api/deployments/{id}/health/check-now
  ```
- **Inspect Health Status:**
  ```bash
  curl -s https://cuan.dojo.cc/api/deployments/{id}/health
  ```
  Contoh respons:
  ```json
  {
    "alive": true,
    "deployment_id": "dep_staging_app_01",
    "url": "http://127.0.0.1:9005",
    "status": "healthy",
    "last_checked_at": "2026-10-17T03:15:00Z",
    "last_status_code": 200,
    "response_time_ms": 32,
    "consecutive_failures": 0,
    "uptime_percent": 100.0,
    "avg_latency_ms": 35.4,
    "total_probes": 15,
    "history": [...]
  }
  ```

---

## 4. Telemetri Resource (CPU, RAM & Disk)

### 4.1 Local Runner Telemetry
Control plane membaca metrik hardware host lokal menggunakan modul `/proc/stat` (CPU delta), `/proc/meminfo` (Memory available vs total), dan `syscall.Statfs` (Disk utilization) tanpa ketergantungan binary eksternal.

### 4.2 Remote BYOS VPS Telemetry
Control plane menggunakan kredensial SSH terenkripsi milik server (dari Sprint 4) untuk menjalankan batch command ringan:
```bash
top -bn1 2>/dev/null | grep -i "cpu(s)"
free -m 2>/dev/null
df -m / 2>/dev/null
```
Eksekusi dibatasi dengan timeout ketat 5 detik dan non-blocking agar tidak mengganggu performa server target.

### 4.3 Ambang Batas Resource Alert (>90%)
- **CPU Threshold:** Penggunaan CPU rata-rata melampaui **90.0%**.
- **RAM Threshold:** Penggunaan Memory melampaui **90.0%** dari total RAM terpasang.
- Jika terlampaui, Alert Dispatcher mencatat insiden hardware `High Resource Usage` dan mengirimkan peringatan sebelum server mengalami OOM (*Out of Memory*) crash.

---

## 5. Panduan Registrasi Saluran Notifikasi (Alert Channels)

Kanal notifikasi dapat didaftarkan melalui modal "Alert Channels" di Studio Dashboard (`/studio/observability`) atau via REST API `POST /api/projects/{id}/alert-channels`.

### 5.1 Telegram Bot API
1. Buka Telegram dan kirim pesan `/newbot` ke [@BotFather](https://t.me/BotFather).
2. Ikuti instruksi untuk menentukan nama dan username bot, lalu simpan token API yang diberikan (misal: `123456789:ABCDefgh-Ijklmnop`).
3. Dapatkan Chat ID Anda (atau grup pengembang):
   - Kirim pesan ke bot Anda, lalu buka `https://api.telegram.org/bot<TOKEN>/getUpdates` untuk melihat `chat.id`.
   - Atau forward pesan ke bot [@userinfobot](https://t.me/userinfobot) untuk melihat ID Anda.
4. Daftarkan di Cuan:
   ```json
   {
     "name": "Telegram Dev Team",
     "type": "telegram",
     "bot_token": "123456789:ABCDefgh-Ijklmnop",
     "chat_id": "-100192837465"
   }
   ```
5. Format Pesan Telegram:
   - 🚨 `*[FIRING] Service Outage: dep_xxx*`
   - ✅ `*[RESOLVED] Service Recovered: dep_xxx*`
   - Dilengkapi rincian kegagalan, latensi, dan tautan langsung ke Studio Dashboard.

### 5.2 Discord Webhook
1. Di aplikasi Discord, buka **Server Settings** $\rightarrow$ **Integrations** $\rightarrow$ **Webhooks**.
2. Klik **New Webhook**, beri nama bot (misal: "Cuan Watchdog"), dan pilih channel target (misal: `#alerts`).
3. Klik **Copy Webhook URL**.
4. Daftarkan di Cuan:
   ```json
   {
     "name": "Discord Alerts",
     "type": "discord",
     "webhook_url": "https://discord.com/api/webhooks/1234567890/token_secret_here"
   }
   ```
5. Format Embed Discord:
   - **Outage / Firing:** Kartu berwarna Merah (`#ED4245`), menampilkan field `Target`, `Status`, `Consecutive Failures`, dan `Response Code`.
   - **Recovery / Resolved:** Kartu berwarna Hijau (`#57F287`), mengonfirmasi layanan kembali beroperasi normal.

### 5.3 Custom Generic Webhook (HMAC-SHA256 Signed)
Ideal untuk menghubungkan Cuan dengan Zapier, Make, n8n, atau API receiver kustom:
```json
{
  "name": "Automation Webhook",
  "type": "webhook",
  "webhook_url": "https://hooks.mycompany.com/cuan-alerts",
  "secret": "my_strong_hmac_secret_key_32chars"
}
```
Payload JSON yang dikirimkan:
```json
{
  "event": "alert.incident",
  "incident_id": "inc_4982fa189c4d",
  "project_id": "proj_prod_01",
  "target_type": "deployment",
  "target_id": "dep_staging_app_01",
  "title": "Service Outage: dep_staging_app_01",
  "message": "Deployment probe failed 3 consecutive times (Status: 500, Response: 45ms)",
  "status": "firing",
  "timestamp": "2026-10-17T03:15:30Z",
  "studio_url": "https://cuan.dojo.cc/deployments/dep_staging_app_01",
  "data": {
    "consecutive_failures": 3,
    "status_code": 500,
    "response_time_ms": 45
  }
}
```
Header Verifikasi:
```http
X-Cuan-Signature: sha256=2c55b68e9d3d3c82fa5698b71...
Content-Type: application/json
```

#### Contoh Verifikasi Signature HMAC (Node.js):
```javascript
const crypto = require('crypto');

function verifyCuanWebhook(req, secret) {
  const signatureHeader = req.headers['x-cuan-signature'];
  if (!signatureHeader || !signatureHeader.startsWith('sha256=')) {
    return false;
  }
  const expectedSig = signatureHeader.slice(7);
  const computedSig = crypto
    .createHmac('sha256', secret)
    .update(req.rawBody)
    .digest('hex');
  return crypto.timingSafeEqual(Buffer.from(expectedSig), Buffer.from(computedSig));
}
```

---

## 6. Uji Integrasi 1-Klik (Test Alert Diagnostics)

Setiap kanal yang telah terdaftar dapat diuji seketika tanpa harus menunggu server down nyata:
1. Klik tombol **"Test Alert"** pada kartu channel di Studio.
2. Cuan backend akan menembakkan payload demo berformat khusus ke API channel terkait.
3. Notifikasi uji coba akan langsung muncul di Telegram/Discord/Webhook Anda dalam < 1 detik.
4. Jika konfigurasi token atau URL salah, sistem akan langsung menampilkan pesan error diagnostik yang jelas (misalnya: `401 Unauthorized - invalid bot token` atau `404 Webhook Not Found`).

---

## 7. Playbook Mitigasi Downtime & Troubleshooting

Saat menerima alert `[FIRING] Service Outage` di Telegram atau Discord, ikuti panduan triage 5 langkah berikut:

```
[ALERT FIRING]
       │
       ▼
1. Buka Tautan Studio ──────────► Periksa Status Code & Latency Terakhir
       │
       ▼
2. Cek Deployment Logs ─────────► Apakah ada Panic / Fatal Error / OOM?
       │
       ▼
3. Cek Server Resource Metrics ──► Apakah CPU/RAM Menyentuh 100%?
       │
       ▼
4. Eksekusi Tindakan Cepat:
   ├── Kasus A: Aplikasi Macet / Deadlock ──► Klik "Restart Deployment" di Studio
   ├── Kasus B: Bug pada Commit Terbaru ──► Rollback ke Deployment Sukses Sebelumnya
   └── Kasus C: Server Habis Disk / RAM ────► Bersihkan Docker Cache / Resize VPS
       │
       ▼
5. Verifikasi Pemulihan ────────► Klik "Check Now" ──► Alert [RESOLVED] Terkirim Otomatis
```

### Pertanyaan Sering Diajukan (FAQ):
- **Q: Mengapa saya tidak menerima notifikasi kedua saat server masih down?**
  *A:* Ini adalah fitur proteksi *Cooldown Window* (10 menit) untuk mencegah *alert fatigue* dan banjir pesan berulang pada masalah yang sama. Begitu server pulih, notifikasi `RESOLVED` akan langsung dikirim.
- **Q: Apakah token bot saya aman disimpan di database?**
  *A:* 100% aman. Token bot, secret HMAC, dan webhook URL dienkripsi menggunakan AES-GCM 256-bit at-rest. Ketika API menyajikan data ke browser, nilai sensitif telah disanitasi (`1234***ef`).
- **Q: Mengapa status deployment menjadi Degraded dan bukan Unhealthy?**
  *A:* Status `degraded` diberikan jika respons melambat (> 2000ms) atau kegagalan baru terjadi 1–2 kali. Hal ini memberikan ruang toleransi terhadap fluktuasi jaringan sementara sebelum menyatakan status darurat *outage*.

---

## 7. Observabilitas Worker Daemon & Antrean Toko (`cuan-runner`)

Mulai Sprint 57, Cuan menyediakan endpoint sistem dan widget antarmuka Pengaturan untuk memantau kesehatan worker mandiri (`cuan-runner`) tanpa memerlukan akses terminal server VPS:

### 7.1 Endpoint Sistem: `GET /api/system/worker-health`
- **Kecepatan & Ringkas (<10ms):** Mengembalikan metrik JSON terstruktur yang memuat status daemon, detak jantung (*heartbeat*), dan kedalaman antrean tugas.
- **Kontrak Respons JSON:**
  ```json
  {
    "status": "ready",
    "runner_id": "cuan-runner",
    "runner_name": "cuan-runner",
    "is_alive": true,
    "last_heartbeat_at": "2026-10-31T18:30:00Z",
    "last_seen_seconds_ago": 4,
    "queued_count": 0,
    "running_count": 0,
    "queue_depth": 0,
    "timestamp": "2026-10-31T18:30:04Z"
  }
  ```

### 7.2 Indikator Visual di Menu Pengaturan (`WorkerHealthCard`)
- **Hijau (Aktif / Siap):** Heartbeat worker aktif dan tidak ada tugas yang memblokir, siap menerima tugas baru.
- **Kuning (Sibuk / Memproses):** Worker sedang menjalankan tugas antrean pelanggan secara aktif.
- **Merah / Abu-abu (Perhatian / Tidak Aktif):** Heartbeat tidak terdeteksi melebihi batas toleransi (45 detik), menandakan daemon worker offline atau membutuhkan pemeriksaan.
- **Kedalaman Antrean:** Menampilkan jumlah tugas *queued*, *running*, dan total *queue depth* yang selalu diperbarui secara berkala.

