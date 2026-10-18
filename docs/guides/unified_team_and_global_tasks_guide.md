# Panduan Operasional Mandiri: Unified Team, Global Tasks Navigation & Chat Scrum Master Persistence

Dokumen ini merupakan panduan operasional mandiri (*self-contained operational guide*) berstandar §11 bagi pengguna mandiri (*Solo Developer*), operator agensi digital, pengelola multi-brand studio, dan administrator sistem mengenai arsitektur sistem Sprint 21 Cuan:
1. **Hotfix PostgreSQL Room/Message Foreign Key & Seed Default Project:** Memastikan persistensi room agen Scrum Master dan riwayat pesan chat di database PostgreSQL tanpa foreign key violation.
2. **Deploy Fresh Frontend Bundle ke VPS Produksi:** Prosedur sinkronisasi build terbaru ke `/opt/cuan/web/dist` dan binary `cuan-server` di VPS `https://cuan.dojo.cc` sehingga auto-hide Bottom Nav di HP aktif dan input composer lega.
3. **Restrukturisasi 4 Navigasi Utama:** Desktop Sidebar Rail & Mobile Bottom Nav Bar yang konsisten: Inbox, Tasks, Projects, Team.
4. **Unified Team Directory:** Menampilkan Humans (Product Owner) dan AI Agents (Scrum Master & Workers) berdampingan dalam satu direktori lengkap dengan onboarding model Fural tanpa API key raw.
5. **Global Task Board View:** Antarmuka papan Kanban Unified Backlog lintas seluruh proyek dengan filter status, pencarian instan, dan pembuatan task cepat.

---

## 1. Arsitektur Komponen & Relasi Database (D2 TALA)

```d2
direction: right

database: "PostgreSQL Database Engine" {
  projects: "projects" {
    id: "id ('default' / UUID)"
    name: "name"
    repo_url: "repo_url"
  }

  rooms: "project_rooms" {
    id: "id (UUID)"
    project_id: "project_id (FK -> projects.id)"
    title: "title (Scrum Master Direct)"
  }

  messages: "project_messages" {
    id: "id (UUID)"
    room_id: "room_id (FK -> project_rooms.id)"
    author_type: "author_type ('user' | 'agent')"
    content: "content (Text)"
  }
}

database.projects -> database.rooms: "FK Constraint (Must Seed 'default')"
database.rooms -> database.messages: "FK Constraint (Must Have Valid Room)"
```

---

## 2. Arsitektur 4 Menu Utama & Unified Team (D2 TALA)

```d2
direction: right

app: "Cuan Responsive Application (AppShell)" {
  rail: "Navigation Bar (Desktop Rail & Mobile Bottom Nav)" {
    inbox: "1. Inbox (Direct Messenger & Chat)"
    tasks: "2. Tasks (Global Kanban Backlog)"
    projects: "3. Projects (Workspace Repositories)"
    team: "4. Team (Unified Directory)"
  }
}

team_view: "Unified Team Directory (TeamView.tsx)" {
  po: "Product Owner: Buchin (Human Owner)"
  sm: "Scrum Master: @sm (AI Manager Autonomous)"
  dev: "Dev Team: Antigravity Flash & Pro Workers"
  modal: "Add Agent Modal (Model Fural — No API Key)"
}

tasks_view: "Global Task Board (TaskBoardView.tsx)" {
  search: "Instant Search Input"
  filters: "Status Pills & Project Filters"
  board: "5-Column Kanban (Backlog, Todo, Doing, Review, Done)"
  create: "+ Task Quick Create Button"
}

app.rail.team -> team_view: "Select Team Tab"
app.rail.tasks -> tasks_view: "Select Tasks Tab"
```

---

## 3. Hotfix PostgreSQL Room/Message Foreign Key & Seed Default Project (S21.1)

### 3.1. Akar Permasalahan
Di versi sebelumnya, tabel `project_rooms` memiliki foreign key constraint `project_rooms_project_id_fkey` yang merujuk ke tabel `projects(id)`. Ketika aplikasi dijalankan di lingkungan produksi baru, tabel `projects` dalam keadaan kosong (0 rows).

Saat fungsi `ensureDefaultAgentRooms` dieksekusi dengan `ProjectID: "default"`, PostgreSQL menolak penyimpanan room default Scrum Master dan Antigravity Flash karena row ber-ID `"default"` tidak ditemukan di tabel `projects`. Akibatnya:
1. Room tidak tersimpan di tabel `project_rooms`.
2. Setiap pesan baru yang dikirim user atau agen gagal disimpan ke tabel `project_messages` karena foreign key constraint `project_messages_room_id_fkey` dilanggar.
3. Riwayat chat selalu mengembalikan array kosong (`[]`) dan hilang saat browser di-refresh.

### 3.2. Solusi Permanen & Idempoten
1. **Boot Seeding (`cmd/server/main.go`):**
   Saat server Go dijalankan dan koneksi database aktif, fungsi `ensureDefaultProjectSeed(dbConn)` secara otomatis meng-ensure row `'default'` di tabel `projects` menggunakan query aman:
   ```sql
   INSERT INTO projects (id, name, repo_url, default_branch, clone_status, project_type, created_at, updated_at)
   VALUES ('default', 'Default Project', 'https://github.com/buchin/cuan', 'main', 'connected', 'custom', NOW(), NOW())
   ON CONFLICT (id) DO NOTHING;
   ```
2. **Handler Auto-Ensure (`internal/server/chat_handlers.go`):**
   Pada awal `ensureDefaultAgentRooms` dan `handleInternalSendConversationMessage`, sistem memeriksa dan meng-insert project `'default'` serta memastikan room tujuan sudah terdaftar sebelum mengeksekusi `CreateMessage`.
3. **Graceful Retry:**
   Jika `CreateMessage` mengalami kendala, sistem secara otomatis mere-ensure room default dan mengulang penyimpanan pesan sekali lagi.

### 3.3. Conversational vs Technical Task Intelligence
Sistem chat Scrum Master kini mampu membedakan jenis pesan:
1. **Percakapan Biasa & Sapaan Kasual:**
   - Pertanyaan seperti *"Apa kabar"*, *"Halo"*, *"Pagi"*, *"Bagaimana sprint?"*, atau *"Siapa kamu?"* direspons secara hangat, natural, dan kontekstual sebagai asisten Scrum Master cerdas.
   - Tidak menampilkan Action Card atau template kaku Problem Statement/DoD agar percakapan terasa manusiawi dan nyaman.
2. **Instruksi Tugas & Pekerjaan Teknis:**
   - Pesan yang mengandung kata kunci tugas seperti *buat*, *perbaiki*, *fix*, *bug*, *deploy*, *fitur*, atau *sprint backlog* dianalisis secara cerdas.
   - Sistem merumuskan Problem Statement yang terarah, menyusun checklist Definition of Done (DoD) dinamis (investigasi root cause, implementasi kode, testing terarah), dan menyajikan kartu aksi interaktif (*Action Card*) dengan tombol **Mulai Kerjakan**.

---

## 4. Restrukturisasi Navigasi 4 Menu Utama & Mobile Drawer (S21.3)

Navigasi aplikasi pada desktop sidebar rail dan mobile kini disederhanakan menjadi 4 menu utama:
1. **Inbox:** Pusat komunikasi langsung antara Product Owner (manusia), Scrum Master (AI), worker agents, serta live chat visitor Dojo.
2. **Tasks:** Global Task Board (Unified Backlog) yang merangkum seluruh tugas dari seluruh workspace proyek dalam satu antarmuka Kanban terpadu.
3. **Projects:** Ruang kerja proyek, repositori GitHub terhubung, dan pengaturan workspace.
4. **Team:** Unified Team Directory yang menggabungkan direktori manusia dan agen AI dalam satu tempat yang transparan.

### 4.1. Mobile Drawer / Slide-Over Sidebar di Layar Ponsel (< md)
Pada smartphone:
- **Hamburger Button (☰):** Tersedia di header atas `AppShell` dan langsung di dalam header chat `InboxView`.
- **Slide-Over Sidebar:** Membuka laci navigasi dari kiri yang menampilkan 4 menu utama (Inbox, Tasks dengan badge Global Board, Projects, Team), subpanel kontekstual, serta tombol Dark/Light mode.
- **Transisi Fleksibel saat Chat Aktif:** Ketika pengguna sedang mengobrol dengan Scrum Master atau agen di ponsel, pengguna dapat kembali via tombol Back (`<`) atau membuka Mobile Drawer (`☰`) untuk langsung melompat ke tab Tasks kapan saja tanpa hambatan.

---

## 5. Unified Team Directory (Humans + Agents) (S21.4)

Tab Team kini menjadi direktori terpadu yang menampilkan:
- **Human Owner:** Buchin (Product Owner) dengan tanggung jawab sprint planning, approval 1-klik, dan evaluasi hasil produksi.
- **AI Scrum Master:** `@sm` (Agent Manager) yang bertugas melakukan triage pesan Inbox otomatis, merumuskan Problem Statement & checklist Definition of Done (DoD), serta mendelegasikan tugas ke worker agents.
- **Development Team (Workers):** Antigravity Flash (`@pin`), Antigravity Pro (`@pin2`), dan agen worker tambahan yang terhubung.
- **Add Agent Modal (Model Fural — No API Key):** Pengguna dapat mendaftarkan agen baru menggunakan handle preset (`@dev`, `@sm`, `@assistant`, `@tester`), runtime provider (`Antigravity CLI`, `Gemini CLI`, `Claude CLI`), dan mekanisme otorisasi sesi CLI tanpa meminta API key mentah berbayar.

---

## 6. Global Task Board View (S21.5)

Tab Tasks menyediakan antarmuka Kanban terpadu lintas seluruh proyek:
- **Pencarian Instan:** Filter judul, deskripsi, atau penugasan task secara real-time.
- **Filter Status Cepat:** Tombol pills status (All, Backlog, To Do, In Progress, In Review, Completed).
- **Badge Proyek:** Setiap kartu task secara otomatis menampilkan nama proyek asalnya saat dilihat di Global View.
- **Checklist DoD & Eksekusi Agen:** Indikator visual progres checklist Definition of Done (`2/3 DoD`) dan tombol `Play` untuk langsung memicu runtime worker agent pada task tersebut.

---

## 7. Prosedur Rilis Produksi di VPS `cuan.dojo.cc` (S21.2 & S21.6)

### 7.1. Build Frontend dan Kompilasi Binary
```bash
# 1. Build frontend bundle
cd web && npm run build && cd ..

# 2. Kompilasi binary Go
go build -o bin/cuan-server ./cmd/server
```

### 7.2. Sinkronisasi ke VPS Produksi
```bash
# 1. Sinkronisasi binary Go
sshpass -p "$DOJO_TERMAPI_VPS_PASSWORD" scp -o StrictHostKeyChecking=no \
  bin/cuan-server $DOJO_TERMAPI_VPS_USERNAME@$DOJO_TERMAPI_VPS_PUBLIC_IP:/opt/cuan/bin/

# 2. Sinkronisasi aset frontend dist/ (dengan --delete agar aset usang terhapus)
sshpass -p "$DOJO_TERMAPI_VPS_PASSWORD" rsync -avz --delete -e "ssh -o StrictHostKeyChecking=no" \
  web/dist/ $DOJO_TERMAPI_VPS_USERNAME@$DOJO_TERMAPI_VPS_PUBLIC_IP:/opt/cuan/web/dist/

# 3. Restart layanan systemd
sshpass -p "$DOJO_TERMAPI_VPS_PASSWORD" ssh -o StrictHostKeyChecking=no \
  $DOJO_TERMAPI_VPS_USERNAME@$DOJO_TERMAPI_VPS_PUBLIC_IP \
  "sudo systemctl restart cuan.service"
```

### 7.3. Smoke Test Produksi
```bash
# Verifikasi liveness & readiness
curl -fsSL https://cuan.dojo.cc/healthz
curl -fsSL https://cuan.dojo.cc/readyz

# Verifikasi live chat widget JS
curl -I https://cuan.dojo.cc/chat/widget.js

# Verifikasi database PostgreSQL produksi
sshpass -p "$DOJO_TERMAPI_VPS_PASSWORD" ssh -o StrictHostKeyChecking=no \
  $DOJO_TERMAPI_VPS_USERNAME@$DOJO_TERMAPI_VPS_PUBLIC_IP \
  "sudo -u postgres psql -d cuan -c 'SELECT id, name FROM projects WHERE id = '\''default'\'';' -c 'SELECT id, title FROM project_rooms;' -c 'SELECT count(*) FROM project_messages;'"
```
