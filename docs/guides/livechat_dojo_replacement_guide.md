# Panduan Operasional: Direct Messenger Hub & Dojo Live Chat Replacement (Sprint 18)

Panduan operasional dan referensi arsitektur untuk sistem percakapan terpadu di Cuan, mencakup Direct Messenger Hub (antarmuka internal WhatsApp/Telegram-style), Drop-In Live Chat Widget untuk ekosistem Dojo (berbasis Shadow DOM), serta backward-compatibility shims (`window.$crisp` dan `window.$dojoChat`).

---

## 1. Ikhtisar & Mental Model Arsitektur

Sprint 18 mentransformasi paradigma perpesanan Cuan dari model "forum thread" terisolasi menjadi **Direct Messenger Hub** yang real-time, responsif, dan terintegrasi penuh dengan seluruh ekosistem Dojo:

1. **Clean Neutral Theme (shadcn/ui zinc & slate)**:
   - Menghapus 100% hardcoded hex Tokyo Night (`#1a1b26`, `#eaeef6`, dll.).
   - Mengadopsi semantic tokens berbasis CSS variables (`hsl(var(--background))`, `hsl(var(--primary))`, dll.) dengan rasio kontras WCAG AAA pada light dan dark mode.
2. **Direct Messenger Hub**:
   - Menghapus keharusan membuat thread dengan judul/topik (`title` bersifat opsional).
   - Pengguna langsung mengklik kontak di sub-panel (Scrum Master, Antigravity Agent, atau Visitor Customer) dan langsung bercakap-cakap.
   - Kartu eksekusi 1-klik (*1-click action card*) untuk agen dengan pratinjau Definition of Done (DoD) dan tombol "Mulai Kerjakan".
3. **Drop-In Live Chat Widget (`/chat/widget.js`)**:
   - Skrip Vanilla JS mandiri berbasis **Shadow DOM** (`attachShadow({ mode: 'open' })`) yang menjamin isolasi gaya CSS dari dan ke host website.
   - Kompatibel penuh dengan kontrak API Dojo Live Chat (`POST /api/chat/session`, `GET /api/chat/messages`, `POST /api/chat/messages`, `POST /api/chat/sessions/{uuid}/page`).
   - Dilengkapi drop-in shims untuk `window.$crisp` (array push queue) dan `window.$dojoChat` (`open`, `close`, `toggle`, `setMessage`, `startTopic`).
   - Open CORS (`Access-Control-Allow-Origin: *`) untuk kemudahan integrasi lintas domain di ekosistem Dojo.

---

## 2. Diagram Alur Kerja & Arsitektur (D2 TALA)

Diagram berikut mengilustrasikan interaksi antara pengunjung ekosistem Dojo, widget live chat, server Cuan, dan operator tim internal melalui Direct Messenger Hub:

```d2
direction: right
vars: {
  d2-config: {
    layout-engine: tala
  }
}

title: Arsitektur Direct Messenger Hub & Dojo Live Chat {
  near: top-center
  shape: text
  style.font-size: 20
  style.bold: true
}

visitor: Pengunjung Website (Dojo Ecosystem) {
  shape: person
}

widget: Dojo Drop-In Widget (/chat/widget.js) {
  shape: package
  style.fill: "#f4f4f5"
  style.stroke: "#18181b"
  
  shadow_dom: Shadow DOM Host {
    shape: rectangle
    launcher: Floating Launcher Button
    window: Chat Window & Bubble Stream
    composer: Multiline Composer
  }
  
  shims: Shims Layer {
    crisp: window.$crisp (Array Push)
    dojo: window.$dojoChat (API Object)
  }
}

cuan_server: Cuan Server (Go Backend) {
  shape: rectangle
  style.fill: "#18181b"
  style.font-color: "#fafafa"

  cors_gate: Open CORS Handler
  chat_api: Live Chat API (/api/chat/*)
  msg_hub: Direct Messenger Engine
  sm_triage: Autonomous Scrum Master
}

database: PostgreSQL Database {
  shape: cylinder
  style.fill: "#e4e4e7"
  project_rooms: Tabel project_rooms (Sessions)
  project_messages: Tabel project_messages (History)
}

operator: Operator / Developer (Cuan Web App) {
  shape: person
}

inbox_ui: Direct Messenger Hub (web/src/features/inbox) {
  shape: package
  subpanel: Contact List Sub-Panel (All / Agents / Customers)
  view: Bubble Stream View & 1-Click Execution Cards
}

# Alur Interaksi
visitor -> widget.shadow_dom.launcher: 1. Klik launcher / Buka chat
widget.shadow_dom.composer -> cuan_server.chat_api: 2. Kirim pesan pengunjung (POST /api/chat/messages)
widget.shims.crisp -> widget.shadow_dom.window: Integrasi kode host eksisting
cuan_server.chat_api -> database.project_rooms: 3. Simpan sesi & pesan
database.project_messages -> cuan_server.msg_hub: Sinkronisasi pesan real-time
cuan_server.msg_hub -> inbox_ui.view: 4. Muncul di tab Customers
operator -> inbox_ui.view: 5. Balas pesan pengunjung
inbox_ui.view -> cuan_server.msg_hub: 6. Kirim respon operator
cuan_server.chat_api -> widget.shadow_dom.window: 7. Polling ambil respon (GET /api/chat/messages)
operator -> cuan_server.sm_triage: 8. Chat internal Scrum Master
cuan_server.sm_triage -> inbox_ui.view: 9. Rekomendasi DoD & 1-Click Run
```

---

## 3. Direct Messenger Hub (Internal UI)

Direct Messenger Hub diakses melalui tab **Inbox** pada sidebar utama Cuan. Antarmuka terbagi menjadi dua sub-komponen utama:

### 3.1 Sub-Panel Kontak (`InboxSubPanel`)
- **Filter Tabs**:
  - `All`: Menampilkan seluruh kontak aktif (agen AI dan pengunjung customer).
  - `Agents`: Kontak tim AI (`Scrum Master`, `Antigravity Flash`).
  - `Customers`: Sesi pengunjung live chat yang aktif dari ekosistem Dojo.
- **Pencarian Real-Time**: Input pencarian memfilter kontak berdasarkan nama, potongan pesan terakhir, atau nama produk.
- **Indikator & Badge**:
  - Avatar lingkaran dengan indikator hijau status *Online*.
  - Badge counter pesan belum dibaca (*Unread count*).
  - Badge identitas produk untuk customer (contoh: `TermAPI Cloud`, `Dojo CLI`).

### 3.2 Tampilan Percakapan (`InboxView`)
- **Bubble Stream Real-Time**:
  - Pesan pengguna terkirim di sisi kanan dengan warna *primary neutral* (`#18181b` pada light mode).
  - Pesan masuk (dari agen atau customer) di sisi kiri dengan latar abu-abu netral (`#e4e4e7` / `#27272a`).
  - Indikator centang ganda (*double checkmark*) dan timestamp format lokal.
- **Kartu Eksekusi 1-Klik (*Agent 1-Click Action Card*)**:
  - Ditampilkan otomatis pada respon Scrum Master yang memiliki rekomendasi tugas.
  - Menampilkan judul task, agen yang direkomendasikan, checklist Definition of Done (DoD), dan tombol **"Mulai Kerjakan"**.
  - Mengklik tombol langsung menjalankan agen worker (`tasksApi.runAgent`) tanpa modal tambahan.
- **Composer Multi-Baris**:
  - Menekan **Enter** langsung mengirim pesan.
  - Menekan **Shift + Enter** membuat baris baru.
  - Tinggi area input membesar secara otomatis hingga batas maksimum 160px.

---

## 4. Drop-In Live Chat Widget (`/chat/widget.js`)

Widget live chat dirancang agar dapat disematkan pada situs eksternal atau aplikasi web mana pun hanya dengan satu tag `<script>`.

### 4.1 Cara Penyematan Cepat (*Quick Embed*)

Tambahkan tag script berikut tepat sebelum tag penutup `</body>`:

```html
<script 
  src="https://cuan.dojo.cc/chat/widget.js" 
  data-product="TermAPI Cloud" 
  data-api-base="https://cuan.dojo.cc" 
  async>
</script>
```

Atribut yang didukung:
- `data-product` (opsional, default: `cuan`): Nama produk/layanan tempat widget dipasang.
- `data-api-base` (opsional, default: origin script URL): URL root backend Cuan.

### 4.2 Kompatibilitas Shims

#### A. Shim `$dojoChat`
Aplikasi host dapat mengontrol widget secara programatik:
```javascript
// Buka jendela chat
window.$dojoChat.open();

// Tutup jendela chat
window.$dojoChat.close();

// Toggle jendela chat
window.$dojoChat.toggle();

// Isi teks ke input composer
window.$dojoChat.setMessage("Saya butuh bantuan aktivasi API key");

// Buka dan langsung mulai topik percakapan
window.$dojoChat.startTopic("Pertanyaan seputar kuota worker");
```

#### B. Shim `$crisp` (Drop-In Crisp Replacement)
Jika website Anda sebelumnya menggunakan Crisp chat, widget Cuan mendukung format antrean array push Crisp tanpa perlu mengubah kode frontend host:
```javascript
window.$crisp = window.$crisp || [];

// Membuka chat
window.$crisp.push(["do", "chat:open"]);

// Menutup chat
window.$crisp.push(["do", "chat:close"]);

// Toggle chat
window.$crisp.push(["do", "chat:toggle"]);

// Menyiapkan teks di composer
window.$crisp.push(["set", "message:text", ["Pertanyaan via Crisp shim"]]);

// Mengirim pesan langsung
window.$crisp.push(["do", "message:send", ["text", "Halo support!"]]);
```

---

## 5. Referensi API & Kontrak Data

### 5.1 Endpoint Pengunjung (Visitor API - Open CORS)
Semua endpoint berikut mendukung `Access-Control-Allow-Origin: *` dan tidak memerlukan header JWT auth.

| Method | Endpoint | Deskripsi |
| :--- | :--- | :--- |
| `POST` | `/api/chat/session` | Inisialisasi atau pemulihan sesi pengunjung berdasarkan `guest_uuid` |
| `GET` | `/api/chat/messages` | Mengambil pesan percakapan berdasarkan `session_uuid` dan `after_id` |
| `POST` | `/api/chat/messages` | Mengirim pesan baru dari pengunjung |
| `POST` | `/api/chat/sessions/{uuid}/page` | Mengirim telemetri URL dan judul halaman yang sedang dikunjungi |
| `POST` | `/api/chat/receipts` | Mengirim tanda terima pesan telah dibaca |
| `GET` | `/chat/widget.js` | Mengambil bundle Vanilla JS widget Shadow DOM |

### 5.2 Endpoint Internal Operator (Internal API - Bearer JWT)
Endpoint berikut digunakan oleh SPA Cuan dan dilindungi oleh otentikasi JWT:

| Method | Endpoint | Deskripsi |
| :--- | :--- | :--- |
| `GET` | `/api/v1/chat/contacts` | Mengambil daftar kontak (filter: `all`, `agents`, `customers`) |
| `GET` | `/api/v1/chat/conversations/{id}/messages` | Mengambil riwayat percakapan kontak |
| `POST` | `/api/v1/chat/conversations/{id}/messages` | Mengirim balasan operator ke pengunjung atau agen |

---

## 6. Prosedur Verifikasi & Runbook Deployment

### 6.1 Targeted Test Suite
Selalu jalankan targeted test suite untuk memvalidasi fungsi chat tanpa memicu probe test warisan:
```bash
go test -v ./internal/server/ -run "TestChatHandlers|TestVerify_S18_4"
```

### 6.2 Build Bundle
Kompilasi binary server Go dan build bundle frontend:
```bash
# Build Go server
go build -o bin/cuan-server ./cmd/cuan-server
go build -o bin/cuan-migrate ./cmd/cuan-migrate

# Build Frontend
cd web && npm run build && cd ..
```

### 6.3 Deployment ke Production VPS (`cuan.dojo.cc`)
```bash
# 1. Upload Binary
sshpass -p "$DOJO_TERMAPI_VPS_PASSWORD" scp -o StrictHostKeyChecking=no \
  bin/cuan-server bin/cuan-migrate \
  $DOJO_TERMAPI_VPS_USERNAME@$DOJO_TERMAPI_VPS_PUBLIC_IP:/opt/cuan/bin/

# 2. Upload Web Assets
sshpass -p "$DOJO_TERMAPI_VPS_PASSWORD" rsync -avz --delete -e "ssh -o StrictHostKeyChecking=no" \
  web/dist/ \
  $DOJO_TERMAPI_VPS_USERNAME@$DOJO_TERMAPI_VPS_PUBLIC_IP:/opt/cuan/web/dist/

# 3. Restart Service
sshpass -p "$DOJO_TERMAPI_VPS_PASSWORD" ssh -o StrictHostKeyChecking=no \
  $DOJO_TERMAPI_VPS_USERNAME@$DOJO_TERMAPI_VPS_PUBLIC_IP \
  "sudo systemctl restart cuan && sudo systemctl is-active cuan"

# 4. Smoke Test
curl -fsS https://cuan.dojo.cc/healthz
curl -fsS https://cuan.dojo.cc/chat/widget.js | head -n 5
```

---
*Dokumen ini merupakan bagian dari Definition of Done (DoD) Sprint 18.*
