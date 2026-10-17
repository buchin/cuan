# Panduan Operasional Creator Blueprints & In-Task Agentic Execution

Dokumen ini merupakan panduan operasional mandiri bagi pengembang mandiri (*Solo Developer*), kreator konten digital, pemilik instance Cuan (*Single-Owner*), dan tim rekayasa perangkat lunak mengenai arsitektur, konfigurasi, alur kerja instansiasi proyek kreator (*Creator Blueprints*), eksekusi tugas berbasis agen AI (*In-Task Agentic Execution Engine*), integrasi Google Gemini Pro (BYOK - *Bring Your Own Key*), persistensi output agen di kolom `agent_output`, sinkronisasi otomatis checklist *Definition of Done* (DoD), serta pengoperasian produksi di domain `cuan.dojo.cc`.

---

## 1. Filosofi & Nilai Produk: "Agentic Project Management for Creators"

Cuan mentransformasi sistem manajemen proyek dari sekadar daftar tugas statis (*dead to-do lists*) menjadi ruang kerja dinamis bertenaga agen otonom yang langsung mengeksekusi konten:

1. **Eliminasi To-Do List Mati:** Pada project management tradisional (seperti Trello, Notion, atau Jira), kartu task hanyalah catatan pasif. Di Cuan, setiap kartu task kreator dapat langsung dijalankan dengan menekan tombol **"Run with Agent"**, menghasilkan deliverable nyata (draf artikel SEO, skrip YouTube 10 menit, email nurture sequence, atau komparasi produk affiliasi) secara langsung di dalam kartu tugas.
2. **Zero-Friction Instantiation (Creator Project Wizard):** Kreator tidak perlu lagi merancang struktur folder, room diskusi, atau daftar task dari nol. Dengan satu klik pada katalog Creator Blueprint (Blog Publisher, YouTube Channel, Online Shop, Affiliate Site), Cuan secara atomik membuat seluruh ekosistem proyek: room diskusi terisolasi, pipeline tahapan kerja terstruktur, kriteria DoD terinci, serta agen AI persona yang telah dikonfigurasi.
3. **In-Task Content Generation & Markdown Drawer:** Hasil kerja agen AI disimpan langsung pada atribut `agent_output` dan disajikan dalam tab *Agent Output* pada Task Detail Drawer dengan perenderan format Markdown bersih dan tombol *One-Click Copy to Clipboard*.
4. **Auto-Updating Definition of Done (DoD):** Ketika agen selesai menyusun draf konten, engine memvalidasi deliverable terhadap butir-butir DoD yang relevan (misal: "Draft hook & title menarik", "Outline 5 subtopik utama", "Meta description & call-to-action") dan secara otomatis menandai checkbox item DoD sebagai terselesaikan.
5. **Zero Credential Leakage:** Eksekusi agen menggunakan kredensial Google Gemini Pro (BYOK) yang tersimpan secara terenkripsi menggunakan algoritma AES-256-GCM via `internal/crypto/vault.go`. Seluruh payload API dan respon publik tidak pernah membocorkan API key ke klien atau log server.
6. **100% Data Retention Guarantee:** Skema database migrasi PostgreSQL `000015_creator_blueprints_execution.up.sql` bersifat murni aditif non-destruktif. Kolom `project_type` dan `metadata` ditambahkan ke tabel `projects`, sementara kolom `agent_output` dan `execution_status` ditambahkan ke tabel `project_tasks`, menjaga 100% integritas seluruh data historis dari Sprint 1 hingga Sprint 9.

---

## 2. Diagram Arsitektur E2E Creator Engine (D2 TALA)

Diagram berikut mengilustrasikan interaksi alur kerja antara kreator di antarmuka Studio, Reverse Proxy Nginx pada `cuan.dojo.cc`, Cuan Control Plane, Creator Blueprint Engine, In-Task Agentic Executor, KeyVault, Google Gemini API, dan PostgreSQL:

```d2
direction: right

creator: "Creator (Blogger, YouTuber, Olshop)" {
  ui: "Cuan Studio (Web Browser)" {
    wizard: "Creator Project Wizard (/studio)"
    kanban: "Kanban Board & In-Task Button"
    drawer: "Task Detail & Agent Output Drawer"
  }
}

gateway: "Reverse Proxy Gateway (cuan.dojo.cc)" {
  nginx: "Nginx TLS Reverse Proxy (Port 443 SSL)"
  tls_gate: "Caddy On-Demand TLS Gatekeeper"
}

cuan_core: "Cuan Control Plane (Go Backend Port 8080)" {
  router: "Chi HTTP Router (/api/v1/*)"
  blueprint_engine: "Creator Blueprint Engine"
  agent_executor: "In-Task Agentic Executor"
  vault: "KeyVault (AES-256-GCM Encryption)"
}

ai_cloud: "AI Provider Cloud" {
  gemini_api: "Google Gemini Pro API (BYOK)"
}

database: "PostgreSQL Database (000015)" {
  projects_table: "projects (project_type, metadata)"
  tasks_table: "project_tasks (agent_output, execution_status)"
  rooms_table: "project_rooms & project_messages"
  historical_data: "Sprint 1-9 Tables (100% Retention)"
}

creator.ui.wizard -> gateway.nginx: "1. Select Blueprint (Blog / YouTube / Olshop)"
gateway.nginx -> cuan_core.router: "2. POST /api/v1/creator/projects/instantiate"
cuan_core.router -> cuan_core.blueprint_engine: "3. Build Blueprint Project Model"
cuan_core.blueprint_engine -> database.projects_table: "4. Atomic Insert Project, Rooms, Tasks & DoD"

creator.ui.kanban -> gateway.nginx: "5. Click 'Run with Agent'"
gateway.nginx -> cuan_core.router: "6. POST /api/v1/tasks/:id/execute-agent"
cuan_core.router -> cuan_core.agent_executor: "7. Fetch Task & DoD Context"
cuan_core.agent_executor -> cuan_core.vault: "8. Decrypt Gemini API Key"
cuan_core.agent_executor -> ai_cloud.gemini_api: "9. Generate Content via LLM Prompt"
ai_cloud.gemini_api -> cuan_core.agent_executor: "10. Return Generated Content Draft"
cuan_core.agent_executor -> database.tasks_table: "11. Save agent_output & Auto-Check DoD"
cuan_core.agent_executor -> database.rooms_table: "12. Broadcast Update to Discussion Room"
cuan_core.agent_executor -> creator.ui.drawer: "13. Render Markdown & One-Click Copy"
```

---

## 3. Struktur Database & Model Data (000015)

Skema database migrasi `migrations/000015_creator_blueprints_execution.up.sql` menambahkan ekstensi aditif ke tabel yang sudah ada:

```sql
-- Cuan Migration 000015: Creator Blueprints & In-Task Agent Execution
-- (000015_creator_blueprints_execution.up.sql)

ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS project_type VARCHAR(64) NOT NULL DEFAULT 'custom',
    ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX IF NOT EXISTS idx_projects_project_type ON projects(project_type);

ALTER TABLE project_tasks
    ADD COLUMN IF NOT EXISTS agent_output TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS execution_status VARCHAR(64) NOT NULL DEFAULT 'idle';

CREATE INDEX IF NOT EXISTS idx_project_tasks_execution_status ON project_tasks(execution_status);
```

### Penjelasan Kolom Kunci:
- **`projects.project_type`**: Menandai kategori proyek kreator (`blog_publisher`, `youtube_channel`, `online_shop`, `affiliate_site`, atau `custom`).
- **`projects.metadata`**: Objek JSONB untuk menyimpan preferensi blueprint, seperti niche blog, URL channel YouTube, atau konfigurasi target audiens.
- **`project_tasks.agent_output`**: Kolom teks tempat persistensi draf konten hasil generate agen AI (artikel lengkap, skrip video dengan timestamps dan visual cues, copy penjualan, atau tabel review komparatif).
- **`project_tasks.execution_status`**: Status eksekusi siklus hidup agen pada task tersebut (`idle`, `running`, `completed`, `failed`).

---

## 4. Katalog 4 Creator Blueprints Siap Pakai

Cuan menyediakan 4 template blueprint utama yang dapat diakses melalui endpoint `GET /api/v1/creator/templates`:

### 1. Blog Publisher Blueprint (`blog_publisher`)
- **Target Persona:** Blogger profesional, editor konten niche, web publisher AdSense & affiliate.
- **Dedicated Agent Persona:** *SEO Content Strategist* (Model: `gemini-1.5-pro`).
- **Pre-seeded Discussion Room:** *Content Strategy & SEO Research*.
- **Standard Pipeline Tasks & DoD:**
  1. *Keyword Research & Topical Map:* Riset kata kunci utama, volume pencarian, dan pemetaan intent pencari.
  2. *Article Outline & Headings Structure:* Pembuatan hierarki H1, H2, H3 yang ramah SEO dan mudah dibaca.
  3. *Comprehensive Pillar Post Drafting:* Penulisan artikel mendalam 1.500+ kata dengan meta description dan rekomendasi visual.

### 2. YouTube Channel Blueprint (`youtube_channel`)
- **Target Persona:** YouTuber, video creator, pembuat konten edukasi & teknologi.
- **Dedicated Agent Persona:** *YouTube Viral Scriptwriter* (Model: `gemini-1.5-pro`).
- **Pre-seeded Discussion Room:** *Channel Ideation & Scripting Studio*.
- **Standard Pipeline Tasks & DoD:**
  1. *Video Ideation, 5 Viral Hooks & Click-Worthy Titles:* Eksplorasi 5 variasi hook pembuka 15 detik pertama dan formula judul berkonversi tinggi.
  2. *Full 10-Minute Narrative Scriptwriting:* Penulisan naskah video komprehensif lengkap dengan *spoken dialogue*, *visual cues* (b-roll, animasi, sound effect), dan CTA sponsor.
  3. *SEO Video Description, Chapters & Tagging:* Penyusunan deskripsi video dengan timestamp bab, link afiliasi, dan tag kata kunci.

### 3. Online Shop / Digital Product Blueprint (`online_shop`)
- **Target Persona:** Pemilik toko online, kreator kursus online, penjual ebook dan template digital.
- **Dedicated Agent Persona:** *Direct Response Copywriter* (Model: `gemini-1.5-pro`).
- **Pre-seeded Discussion Room:** *Product Launch & Funnel Optimization*.
- **Standard Pipeline Tasks & DoD:**
  1. *High-Converting Sales Page Copy:* Penyusunan struktur penawaran (*Hook, Problem Agitation, Solution, Testimonial, Risk-Reversal Guarantee, Call-To-Action*).
  2. *3-Part Email Launch Nurture Sequence:* Tiga naskah email berurutan (Email 1: *The Secret Hook*, Email 2: *Case Study & Proof*, Email 3: *Last Chance Urgency*).
  3. *FAQ & Objection Handling Matrix:* Penanganan 7 keberatan utama calon pembeli sebelum melakukan checkout.

### 4. Affiliate Site Blueprint (`affiliate_site`)
- **Target Persona:** Pengelola situs ulasan teknologi, blogger afiliasi SaaS dan hosting.
- **Dedicated Agent Persona:** *Honest Affiliate Review Specialist* (Model: `gemini-1.5-pro`).
- **Pre-seeded Discussion Room:** *Affiliate Strategy & Product Reviews*.
- **Standard Pipeline Tasks & DoD:**
  1. *Feature Comparison Matrix (Product A vs Product B):* Analisis komparasi objektif kelebihan, kekurangan, dan pricing.
  2. *In-Depth Honest Product Review Article:* Ulasan mendalam berdasarkan pengujian langsung dengan panduan pembelian yang persuasif.

---

## 5. Alur Kerja In-Task Agentic Execution & Auto-DoD

Proses eksekusi konten dari tombol *"Run with Agent"* pada Kanban board bekerja secara deterministik sebagai berikut:

1. **Pemanggilan REST Endpoint:**
   Klien mengirimkan request `POST /api/v1/tasks/{id}/execute-agent` atau alias `POST /api/tasks/{id}/execute-agent`.
2. **Pengambilan Konteks Tugas:**
   Engine mengambil data task, judul, deskripsi instruksi, checklist DoD, dan konteks room diskusi.
3. **Penyusunan Prompt Terarah:**
   Berdasarkan template blueprint dan kebutuhan task, engine merakit system prompt yang mewajibkan LLM menghasilkan konten Markdown lengkap yang siap publikasi.
4. **Eksekusi LLM via Gemini Pro Key Vault:**
   Kredensial Gemini Pro diambil dari KeyVault terenkripsi AES-256-GCM. Jika koneksi atau API key belum disetel, engine mengembalikan instruksi ramah pengguna untuk menghubungkan API key di Agent Hub.
5. **Penyimpanan Deliverable:**
   Draf konten lengkap disimpan ke atribut `agent_output` pada tabel `project_tasks`, dan status eksekusi diperbarui menjadi `completed`.
6. **Auto-Update Checklist Definition of Done:**
   Engine secara otomatis memeriksa item DoD yang belum dicentang dan memperbarui statusnya menjadi `completed: true` apabila konten draf mencakup kriteria tersebut.
7. **Notifikasi Room Diskusi:**
   Pesan otomatis diposting ke room diskusi proyek yang memberitahukan bahwa agen telah menyelesaikan draf konten beserta cuplikan deliverable.
8. **Render UI & One-Click Copy:**
   Pengguna dapat langsung membuka Task Detail drawer, membaca hasil konten pada tab *Agent Output*, dan menyalin draf sekali klik dengan tombol *Copy to Clipboard*.

---

## 6. Prosedur Rilis & Verifikasi Produksi di cuan.dojo.cc

Instance produksi Cuan berjalan pada host `DOJO_TERMAPI_VPS` di bawah domain publik `cuan.dojo.cc`:

- **Port Internal Server:** `8080` (HTTP)
- **Reverse Proxy:** Nginx dengan konfigurasi SSL Let's Encrypt aktif
- **On-Demand TLS:** Gatekeeper Caddy mengotorisasi domain melalui endpoint `/api/caddy/ask?domain=cuan.dojo.cc`
- **Database:** PostgreSQL lokal pada port `5432` dengan skema migrasi `000015_creator_blueprints_execution`

### Langkah-Langkah Operasional Rilis:
1. **Kompilasi Biner & Aset:**
   ```bash
   ./scripts/build.sh --backend-only
   ./scripts/build.sh --frontend-only
   ```
2. **Sinkronisasi Berkas ke Server Produksi:**
   Biner `cuan-server`, `cuan-runner`, `cuan-migrate` disinkronkan ke `/opt/cuan/bin/`, berkas SQL migrasi ke `/opt/cuan/migrations/`, dan bundel frontend ke `/opt/cuan/web/dist/`.
3. **Eksekusi Migrasi 000015 di Server Produksi:**
   ```bash
   /opt/cuan/bin/cuan-migrate --direction up
   ```
4. **Restart Layanan Systemd:**
   ```bash
   sudo systemctl restart cuan.service
   ```
5. **Verifikasi Smoke Test Publik:**
   - `curl -s https://cuan.dojo.cc/healthz` -> `{"app":"cuan","status":"healthy",...}` (HTTP 200)
   - `curl -s https://cuan.dojo.cc/readyz` -> `{"database":"connected","migrations":"applied","migrations_count":15,"status":"ready"}` (HTTP 200)
   - `curl -s https://cuan.dojo.cc/api/v1/creator/templates` -> HTTP 200 dengan 4 blueprint katalog.
