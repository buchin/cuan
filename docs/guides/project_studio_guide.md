# Panduan Operasional Project Studio: Discussions, Tasks Engine & Production Launch

Dokumen ini merupakan panduan operasional komprehensif bagi pemilik instance (*Single-Owner*), pengembang mandiri (*Solo Developer*), agensi digital, tim rekayasa perangkat lunak berkolaborasi dengan AI, dan operator multi-proyek mengenai arsitektur, konfigurasi, alur kerja ruang diskusi (*Project Discussion Rooms*), percakapan berutas multi-aktor (*Threaded Chat*), mesin penyulingan diskusi ke tugas (*Discussion-to-Task Distiller*), papan Kanban interaktif dengan kendali kriteria *Definition of Done* (DoD), manajemen dekomposisi tugas hierarkis (*Fan-Out Subtasks*), orkestrasi batch multi-proyek (*Batch ID*), penandaan fleksibel (*Tags Selector*), integrasi eksekusi agen otomatis (*Agent Execution Hook*), serta prosedur peluncuran produksi (*Production Launch*) di domain kanonik `cuan.dojo.cc` dengan Caddy *On-Demand TLS*.

---

## 1. Filosofi & Gambaran Umum

Fitur **Project Studio** pada Cuan mengusung filosofi **Project-Centric & Self-Contained Collaboration**:
1. **Zero-Friction Project Context:** Seluruh komunikasi (diskusi, instruksi, pertanyaan arsitektur) dan backlog tugas tersimpan melekat pada entitas proyek (`projects`). Tim tidak perlu beralih konteks antara aplikasi obrolan eksternal (seperti Discord atau Slack) dan pelacak isu (seperti Jira atau Trello).
2. **Multi-Actor Threaded Dialogue:** Ruang obrolan mendukung identitas multi-peran yang jelas (`user` untuk manusia/operator, `agent` untuk Scrum Master atau coding worker AI, dan `system` untuk audit log / notifikasi otomasi) serta utas balasan bersarang (*threaded replies* via `reply_to_id`).
3. **Discussion-to-Task Distiller with Guaranteed DoD:** Kesepakatan dalam diskusi dapat disuling seketika menjadi proposal tugas terstruktur. Mesin penyuling secara otomatis mengekstrak atau menginferensikan minimal 3 butir kriteria penerimaan (*Definition of Done* / DoD) yang terukur sebelum tugas disimpan ke backlog.
4. **Interactive Kanban & Real-Time SSE:** Papan kerja menyajikan 4 status alur kerja (*Backlog*, *Doing*, *Review*, *Done*) dengan pembaruan instan melalui *Server-Sent Events* (SSE) dan *heartbeat* 15 detik, serta fitur centang interaktif kriteria DoD langsung pada kartu tugas.
5. **Future-Proof Fan-Out & Batch Architecture:** Tabel `project_tasks` telah dirancang sejak awal (Opsi A yang disetujui PO) dengan dukungan kolom hierarki subtask (`parent_task_id`), pengelompokan batch multi-proyek (`batch_id`), dan array tag dinamis (`tags` JSONB terindeks GIN) untuk skalabilitas masa depan tanpa migrasi skema ulang.
6. **Automated Agent Hook & Dispatching:** Transisi tugas dari `backlog` ke `doing` secara otomatis memicu `AgentExecutionHook` dan mendispatch tugas ke antrean runner untuk dieksekusi oleh agen otonom.
7. **100% Data Retention & Backward Compatibility:** Skema migrasi database PostgreSQL `000012_project_discussions_tasks.up.sql` bersifat murni aditif tanpa mengubah struktur kolom Sprint 1–6, menjamin 100% integritas data historis pengguna, proyek, tugas lawas, sponsor, deployment lokal, server BYOS, custom domain, dan observabilitas.

---

## 2. Diagram Arsitektur E2E Project Studio

Berikut adalah diagram alur kerja siklus hidup interaksi pengguna, obrolan real-time, penyulingan tugas, papan Kanban, dan perutean produksi dengan Caddy:

```d2
direction: right

user: "Pengguna / AI Agent\n(Web UI Studio)" {
  shape: person
}

cuan_control_plane: "Cuan Control Plane (Go Backend)" {
  router: "Chi HTTP Router\n(/api/projects/{id}/rooms,\n/api/rooms/{id}/messages,\n/api/projects/{id}/tasks,\n/api/rooms/{id}/events)"
  studio_service: "Project Studio Repository Layer\n(Rooms, Messages, Tasks CRUD & Fan-Out)"
  distiller: "Discussion-to-Task Distiller\n(Transcript Parser & DoD Checklist Inferer)"
  agent_hook: "Agent Execution Hook\n(Task Status Transition Dispatcher)"
  sse_broker: "Room Event Broker\n(SSE Real-Time Broadcaster & Heartbeat)"
}

caddy_proxy: "Caddy Reverse Proxy\n(cuan.dojo.cc)" {
  tls_ondemand: "On-Demand TLS Engine\n(ACME Ask Gatekeeper)"
  reverse_proxy: "Dynamic Reverse Proxy\n(:8080 Stable / :8081 Staging)"
}

database: "PostgreSQL Database (000012)" {
  project_rooms: "project_rooms\n(id, project_id, title, topic)"
  project_messages: "project_messages\n(id, room_id, author_type, content, reply_to_id)"
  project_tasks: "project_tasks\n(id, project_id, status, dod_checklist, parent_task_id, batch_id, tags)"
  historical_data: "Sprint 1-6 Historical Tables\n(owners, projects, tasks, sponsors, servers, domains, observability)"
}

user -> caddy_proxy.reverse_proxy: "1. Request HTTPS https://cuan.dojo.cc"
caddy_proxy.tls_ondemand -> cuan_control_plane.router: "2. Query /api/caddy/ask?domain=cuan.dojo.cc"
caddy_proxy.reverse_proxy -> cuan_control_plane.router: "3. Proxy HTTP Traffic ke Backend"
cuan_control_plane.router -> cuan_control_plane.studio_service: "4. CRUD Rooms, Messages & Tasks"
cuan_control_plane.router -> cuan_control_plane.sse_broker: "5. Subscribe SSE Stream (/api/rooms/{id}/events)"
cuan_control_plane.router -> cuan_control_plane.distiller: "6. Distill Chat to Task (/distill)"
cuan_control_plane.distiller -> cuan_control_plane.studio_service: "7. Persist Distilled Task with >= 3 DoD"
cuan_control_plane.studio_service -> cuan_control_plane.agent_hook: "8. Status -> 'doing' triggers hook"
cuan_control_plane.studio_service -> database.project_rooms: "9a. Read/Write Rooms"
cuan_control_plane.studio_service -> database.project_messages: "9b. Read/Write Threaded Messages"
cuan_control_plane.studio_service -> database.project_tasks: "9c. Read/Write Tasks & Fan-Out"
cuan_control_plane.sse_broker -> user: "10. Real-time push events (room, message, task)"
```

---

## 3. Ruang Diskusi Proyek & Threaded Chat

### 3.1 Manajemen Room Diskusi (`project_rooms`)
Setiap proyek dapat memiliki banyak ruang diskusi terisolasi berdasarkan topik (misal: *Sprint Planning*, *Architecture Review*, *Bug Triage*).

- **Membuat Room Baru:**
  ```bash
  curl -X POST https://cuan.dojo.cc/api/projects/{project_id}/rooms \
    -H "Content-Type: application/json" \
    -d '{
      "title": "Sprint Planning Room",
      "topic": "Perencanaan Sprint 7 Project Studio & Production Launch"
    }'
  ```
  Respons HTTP `201 Created` mengembalikan objek room lengkap dengan UUID RFC 4122.

- **Melihat Daftar Room per Proyek:**
  ```bash
  curl -s https://cuan.dojo.cc/api/projects/{project_id}/rooms
  ```

### 3.2 Percakapan Multi-Aktor & Threaded Replies (`project_messages`)
Pesan chat dikelompokkan dalam room dengan metadata pengirim yang jelas:
- `author_type`: `user` (manusia), `agent` (bot/AI Scrum Master), `system` (sistem/audit).
- `reply_to_id`: UUID pesan induk untuk membuat balasan terstruktur (*nested thread*).

- **Mengirim Pesan Awal (User):**
  ```bash
  curl -X POST https://cuan.dojo.cc/api/rooms/{room_id}/messages \
    -H "Content-Type: application/json" \
    -d '{
      "author_type": "user",
      "author_id": "usr_buchin",
      "author_name": "Buchin",
      "content": "Mohon susun rencana peluncuran Studio dan checklist DoD untuk rilis produksi."
    }'
  ```

- **Mengirim Balasan Berutas (Agent Threaded Reply):**
  ```bash
  curl -X POST https://cuan.dojo.cc/api/rooms/{room_id}/messages \
    -H "Content-Type: application/json" \
    -d '{
      "author_type": "agent",
      "author_id": "agt_scrum_master",
      "author_name": "Scrum Master Agent",
      "reply_to_id": "UUID_PESAN_USER_DIATAS",
      "content": "Rencana rilis siap:\n- [ ] Uji E2E drill Staging port 8081\n- [ ] Migrasi database 000012 dan audit retensi data 100%\n- [ ] Deploy Cuan produksi via BYOS VPS di cuan.dojo.cc\n- [ ] Dokumentasi panduan operasional di docs/guides/project_studio_guide.md"
    }'
  ```

### 3.3 Real-Time Streaming via Server-Sent Events (SSE)
Klien terhubung ke endpoint `/api/rooms/{room_id}/events` untuk mendengarkan perubahan secara real-time:
```bash
curl -N -H "Accept: text/event-stream" https://cuan.dojo.cc/api/rooms/{room_id}/events
```
Format payload event:
- Handshake: `event: connected` dengan data `{"status":"connected","room_id":"..."}`.
- Heartbeat: komentar `: heartbeat` dan ping event setiap 15 detik untuk menjaga koneksi tetap hidup melalui reverse proxy dan NAT firewall.
- Domain Events: `room.updated`, `message.created`, `message.deleted`, `task.created`, `task.status_changed`, `task.updated`, `task.distilled`.

---

## 4. Discussion-to-Task Distiller Engine

Mesin penyuling (*distiller*) menganalisis histori obrolan dalam room dan menghasilkan draf tugas terstruktur tanpa memerlukan entri manual yang berulang.

### 4.1 Ekstraksi Judul & Deskripsi Konteks
- **Judul Tugas:** Diekstrak dengan prioritas: judul eksplisit payload $\rightarrow$ pola judul pesan (misal `Task: ...`, `Judul: ...`) $\rightarrow$ judul room $\rightarrow$ topik room $\rightarrow$ cuplikan pesan pertama.
- **Deskripsi:** Merangkum transkrip obrolan dengan atribusi peran `[author_type - author_name]` sehingga konteks kesepakatan tetap transparan.

### 4.2 Jaminan Minimal 3 Butir Kriteria DoD (*Definition of Done*)
Mesin penyuling menjamin proposal tugas selalu memiliki minimal 3 butir kriteria terverifikasi:
1. **Ekstraksi Eksplisit:** Mengidentifikasi pola checklist markdown seperti `- [ ]`, `- [x]`, `* [ ]`, atau baris berawalan `DoD:`.
2. **Inferensi Cerdas Otomatis:** Apabila kriteria eksplisit kurang dari 3 butir, mesin inferensi melengkapi dengan kriteria berbasis konteks obrolan (misalnya verifikasi API endpoints, targeted automated tests, dokumentasi operasional, atau release gate).

- **Memanggil Endpoint Distill:**
  ```bash
  curl -X POST https://cuan.dojo.cc/api/projects/rooms/{room_id}/distill \
    -H "Content-Type: application/json" \
    -d '{
      "title": "S07.5 — Production Launch di cuan.dojo.cc"
    }'
  ```
  Respons HTTP `201 Created` mengembalikan draf tugas yang telah tersimpan di backlog beserta rincian jumlah kriteria DoD yang diekstrak.

---

## 5. Papan Kanban & Definition of Done (DoD) Checklist

### 5.1 Siklus 4 Kolom Kanban
Papan tugas menyajikan 4 kolom status standar:
- **`backlog`:** Tugas baru hasil penyusunan rencana atau distilasi obrolan.
- **`doing`:** Tugas sedang dikerjakan secara aktif oleh tim manusia atau agen otonom.
- **`review`:** Implementasi telah selesai dan menunggu verifikasi bukti audit serta evaluasi kriteria DoD.
- **`done`:** Seluruh kriteria DoD terpenuhi 100% dan terverifikasi tanpa regresi.

### 5.2 Transisi Status & Pemicu Agen Otomatis (*Agent Execution Hook*)
Ketika status tugas diperbarui menjadi `doing` via endpoint PATCH:
```bash
curl -X PATCH https://cuan.dojo.cc/api/tasks/{task_id}/status \
    -H "Content-Type: application/json" \
    -d '{"status": "doing"}'
```
Backend secara otomatis mengaktifkan `AgentExecutionHook` yang meneruskan tugas ke `RunnerTaskBridgeDispatcher` untuk segera dijalankan di antrean runner tanpa intervensi manual.

### 5.3 Kendali Interaktif Kriteria DoD Langsung di Kartu
Pengguna dan agen dapat menandai kriteria DoD yang telah selesai secara langsung melalui endpoint update:
```bash
curl -X PUT https://cuan.dojo.cc/api/tasks/{task_id} \
    -H "Content-Type: application/json" \
    -d '{
      "dod_checklist": [
        {"id": "dod-1", "criterion": "Automated E2E Drill di Staging Port 8081", "completed": true},
        {"id": "dod-2", "criterion": "Migrasi skema 000012 & 100% data retention", "completed": true},
        {"id": "dod-3", "criterion": "Deploy Cuan di cuan.dojo.cc via BYOS VPS", "completed": true},
        {"id": "dod-4", "criterion": "Panduan operasional di docs/guides/", "completed": true}
      ]
    }'
```
Setiap perubahan memicu pembaruan indikator persentase (*progress bar*) di UI Studio dan menyiarkan event SSE `task.updated`.

---

## 6. Hierarki Fan-Out, Multi-Project Batch ID & Tag Selector

Skema tabel `project_tasks` mengakomodasi kebutuhan dekomposisi tugas tingkat lanjut dan pengelompokan lintas-proyek:

| Atribut | Tipe Data | Kegunaan & Contoh Kasus |
| :--- | :--- | :--- |
| **`parent_task_id`** | `UUID` (self-referencing FK) | **Dekomposisi Fan-Out:** Memecah *Epic* besar menjadi tugas-tugas anak (*Subtasks*). |
| **`batch_id`** | `VARCHAR(255)` | **Orkestrasi Batch Multi-Proyek:** Mengelompokkan tugas yang berjalan paralel pada banyak proyek (misal `batch-multisite-agency-q4`). |
| **`tags`** | `JSONB array` (GIN index) | **Tag Selector Fleksibel:** Mengelompokkan tugas berbasis kategori fungsional (misal `["wordpress", "seo", "adsense"]`). |

### 6.1 Query Filtering Lanjut via REST API
Backend menyediakan filter pencarian multi-dimensi pada endpoint GET:
- Filter Subtask: `GET /api/projects/{project_id}/tasks?parent_task_id={parent_uuid}`
- Filter Batch: `GET /api/projects/{project_id}/tasks?batch_id=batch-multisite-agency-q4`
- Filter Tag: `GET /api/projects/{project_id}/tasks?tag=wordpress`
- Kombinasi Filter: `GET /api/projects/{project_id}/tasks?status=doing&tag=wordpress`

---

## 7. Panduan Deployment Produksi & Konfigurasi Caddy On-Demand TLS

Platform Cuan dioperasikan pada server kanonik `cuan.dojo.cc` menggunakan Caddy sebagai gerbang *reverse proxy* otomatis dengan *On-Demand TLS*.

### 7.1 Port & Isolasi Lingkungan
- **Produksi (Stable):** Port `8080` (Domain `cuan.dojo.cc`).
- **Staging:** Port `8081` (Environment terisolasi untuk E2E drill).

### 7.2 Gatekeeper Caddy On-Demand TLS (`/api/caddy/ask`)
Untuk mencegah eksploitasi kuota sertifikat Let's Encrypt / ZeroSSL oleh domain jahat (*Denial-of-Wallet* / *Rate-Limit Exhaustion*), Caddy dikonfigurasikan memverifikasi otorisasi domain sebelum sertifikat diterbitkan:
```
# Konfigurasi Caddy On-Demand TLS
{
    on_demand_tls {
        ask http://127.0.0.1:8080/api/caddy/ask
        interval 2m
        burst 5
    }
}
```
Ketika request TLS masuk untuk `cuan.dojo.cc`, Caddy memanggil `GET /api/caddy/ask?domain=cuan.dojo.cc`:
- Jika domain terdaftar sebagai `BaseDomain` kanonik atau terverifikasi di `DomainRepository` $\rightarrow$ merespons `200 OK` (`"status":"authorized"`).
- Jika domain liar, belum diverifikasi, atau malformed $\rightarrow$ merespons `403 Forbidden` (`"status":"forbidden"`).

### 7.3 Prosedur Smoke Test Produksi
Setelah deployment selesai, lakukan uji kesehatan langsung:
```bash
# 1. Health probe
curl -I -sS https://cuan.dojo.cc/healthz
# Ekspektasi: HTTP/2 200 OK

# 2. Readiness check
curl -sS https://cuan.dojo.cc/readyz
# Ekspektasi: {"status":"ready","database":"connected","migrations":"applied"}

# 3. System metadata & mode check
curl -sS https://cuan.dojo.cc/api/info
curl -sS https://cuan.dojo.cc/api/mode
# Ekspektasi: {"app_mode":"studio","version":"...","base_domain":"cuan.dojo.cc"}
```

---

## 8. Panduan Triage, Troubleshooting & FAQ

### T1: Koneksi SSE terputus secara berkala
- **Penyebab:** Reverse proxy pihak ketiga atau load balancer mematikan koneksi HTTP yang diam (*idle timeout*).
- **Solusi:** Pastikan reverse proxy mengaktifkan unbuffered streaming (`X-Accel-Buffering: no` di Nginx atau `flush_interval -1` di Caddy). `RoomEventBroker` pada Cuan telah dilengkapi heartbeat ping berkala setiap 15 detik untuk menjaga soket tetap aktif.

### T2: Tugas gagal disuling dari obrolan (*Distill Error*)
- **Penyebab:** Room belum memiliki riwayat pesan teks yang memadai atau format JSON tidak valid.
- **Solusi:** Kirimkan minimal satu pesan dalam room yang memuat rencana kerja atau kriteria bertanda `- [ ]`. Mesin penyuling secara otomatis menginferensikan sisa kriteria agar memenuhi syarat minimum 3 DoD.

### T3: Sertifikat TLS tidak terbit untuk domain baru
- **Penyebab:** Domain belum menyelesaikan verifikasi DNS (CNAME / TXT) atau belum tercatat berstatus `verified` di sistem.
- **Solusi:** Periksa status domain via `GET /api/domains/{id}`. Gatekeeper `/api/caddy/ask` menolak penerbitan sertifikat SSL jika DNS belum terbukti mengarah ke server Cuan.

---

*Dokumen ini merupakan bagian dari Audit Evidence Pack Cuan Sprint 7 dan menjadi Single Source of Truth (SSOT) operasional Project Studio.*
