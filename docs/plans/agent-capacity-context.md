# Rencana Arsitektur: Snapshot Kapasitas Akun & Runner untuk SM Inbox

## Otorisasi & Scope Digest
- **Scope Digest**: `d958442680c877a2693c8175b0d7e14dff49d338bda07cdc8c870b453d0760df`
- **Approval Message**: `8a1b76ba-6c90-46bf-bb45-9b383c9edd05`
- **User Authorization**: Approved: Rencana Arsitektur Snapshot Kapasitas Akun & Runner untuk SM Inbox
- **Conversation**: `37025f2f-f469-42e3-af92-57166a6d4dd7`
- **Tujuan**: Menganalisis arsitektur backend saat ini dan merancang mekanisme penyediaan snapshot status kapasitas, kuota provider, session slot, dan readiness task runner yang tersanitasi ke dalam context Scrum Master (SM) di Inbox. Hal ini memampukan SM menjawab pertanyaan pengguna mengenai kesiapan eksekusi secara proaktif dan transparan tanpa menyuruh pengguna memeriksa menu Settings secara manual.

---

## 1. Evaluasi Mekanisme Backend Cuan Saat Ini

Berdasarkan penelusuran kode sumber Cuan pada komponen [`internal/agent`](file:///opt/cuan/releases/9509adad38a4a22e41dde3af1ec5e78168639c6a/data/projects/task-workspaces/8516c6c49ec359c7c8609dd0259aacc9c515603136dd7bc8d492c8b1b64d7285/internal/agent), [`internal/server`](file:///opt/cuan/releases/9509adad38a4a22e41dde3af1ec5e78168639c6a/data/projects/task-workspaces/8516c6c49ec359c7c8609dd0259aacc9c515603136dd7bc8d492c8b1b64d7285/internal/server), [`internal/runner`](file:///opt/cuan/releases/9509adad38a4a22e41dde3af1ec5e78168639c6a/data/projects/task-workspaces/8516c6c49ec359c7c8609dd0259aacc9c515603136dd7bc8d492c8b1b64d7285/internal/runner), dan [`internal/task`](file:///opt/cuan/releases/9509adad38a4a22e41dde3af1ec5e78168639c6a/data/projects/task-workspaces/8516c6c49ec359c7c8609dd0259aacc9c515603136dd7bc8d492c8b1b64d7285/internal/task), berikut evaluasi mekanisme yang berjalan saat ini:

### 1.1 Pelacakan Health & Status Autentikasi Akun
- **Struktur Akun**: Dikelola melalui [`agent.ProviderAccount`](file:///opt/cuan/releases/9509adad38a4a22e41dde3af1ec5e78168639c6a/data/projects/task-workspaces/8516c6c49ec359c7c8609dd0259aacc9c515603136dd7bc8d492c8b1b64d7285/internal/agent/account_profiles.go#L19-L37) dengan penyimpanan metadata JSON pada path privat `$CUAN_CONFIG_HOME/.local/share/cuan/accounts/<account_id>`.
- **Status Akun (`State`)**:
  - `authenticated`: Token tersedia, profil siap pakai.
  - `login_required`: Token kadaluarsa atau tidak valid (HTTP 401/invalid_grant).
  - `quota_limited`: Terkena limit kuota global atau cooldown runtime.
  - `unavailable`: Terjadi error sistem atau kegagalan internal.
- **Siklus Hidup Token**: Token OAuth access dicek masa aktifnya; jika tersisa kurang dari 1 menit atau saat verifikasi mengembalikan 401, backend melakukan refresh otomatis melalui Google OAuth dengan penguncian eksklusif (`.execution.lock`) untuk mencegah race condition penulisan berkas kredensial.

### 1.2 Pelacakan Kuota Provider & Model Catalog
- **RPC Kuota**: Backend memanfaatkan RPC Cloud Code (`/v1internal:fetchAvailableModels`) melalui endpoint `daily-cloudcode-pa.googleapis.com` atau `cloudcode-pa.googleapis.com` (diimplementasikan di [`model_usage.go`](file:///opt/cuan/releases/9509adad38a4a22e41dde3af1ec5e78168639c6a/data/projects/task-workspaces/8516c6c49ec359c7c8609dd0259aacc9c515603136dd7bc8d492c8b1b64d7285/internal/agent/model_usage.go)).
- **Caching**: Hasil query kuota di-cache di memori selama 30 detik (keyed by `owner:account_id:sha256(token)`). Timeout per request adalah 3 detik dengan budget preflight maksimal 8 detik.
- **Penyaringan Model Teks**: Fungsi [`agent.IsTextModel`](file:///opt/cuan/releases/9509adad38a4a22e41dde3af1ec5e78168639c6a/data/projects/task-workspaces/8516c6c49ec359c7c8609dd0259aacc9c515603136dd7bc8d492c8b1b64d7285/internal/agent/model_usage.go#L44-L47) memfilter model Gemini dan Claude, serta mengabaikan model non-teks (image, embedding, computer-use).
- **Validasi Catalog**: [`agent.FetchRunnableAccountModelUsage`](file:///opt/cuan/releases/9509adad38a4a22e41dde3af1ec5e78168639c6a/data/projects/task-workspaces/8516c6c49ec359c7c8609dd0259aacc9c515603136dd7bc8d492c8b1b64d7285/internal/agent/runnable_models.go#L44-L78) menyelaraskan data kuota dari RPC dengan model yang benar-benar didukung oleh instalasi binary CLI native (`agy`).
- **Cooldown Runtime**: Jika provider mengembalikan error kuota (HTTP 429 / resource_exhausted), sistem mencatat cooldown 5 menit pada model tersebut via `RecordProviderModelQuota` dan pembatasan regional via `RecordProviderModelRestriction`.

### 1.3 Alokasi Session Slot (`CUAN_ACCOUNT_MAX_SESSIONS`)
- **Kontrol Konkurensi OS**: Diatur dalam [`internal/agent/account_lease.go`](file:///opt/cuan/releases/9509adad38a4a22e41dde3af1ec5e78168639c6a/data/projects/task-workspaces/8516c6c49ec359c7c8609dd0259aacc9c515603136dd7bc8d492c8b1b64d7285/internal/agent/account_lease.go). Konkurensi lokal dibatasi oleh environment variable `CUAN_ACCOUNT_MAX_SESSIONS` (nilai integer valid 1–16, default 4).
- **Mekanisme Lock**:
  1. **Gate Lock (`.execution.lock`)**: Diambil secara bersamaan (`syscall.LOCK_SH`) oleh sesi eksekusi. Lock eksklusif (`syscall.LOCK_EX`) hanya digunakan saat penulisan refresh token.
  2. **Slot Locks (`.session-0.lock` s/d `.session-(N-1).lock`)**: Sesi mengambil slot pertama yang berhasil dikunci secara eksklusif non-blocking (`syscall.LOCK_EX | syscall.LOCK_NB`).
- **Pelepasan Slot**: Lock berbasis file descriptor kernel OS, sehingga otomatis lepas saat proses berakhir (bahkan jika proses mati mendadak/crash).
- **Pemisahan Konseptual Kapasitas vs Kuota**: Keterbatasan slot sesi adalah batas konkurensi lokal server, bukan kegagalan kuota API provider.

### 1.4 Status Failover Rute Akun
- **Preflight Failover**: [`inbox_failover.go`](file:///opt/cuan/releases/9509adad38a4a22e41dde3af1ec5e78168639c6a/data/projects/task-workspaces/8516c6c49ec359c7c8609dd0259aacc9c515603136dd7bc8d492c8b1b64d7285/internal/server/inbox_failover.go) memeringkat pasangan akun dan model berdasarkan sisa persentase kuota.
- **Kandidat Rute**: Akun utama (`primary`) dievaluasi terlebih dahulu, diikuti hingga 7 akun backup yang valid (`AllowFailover == true`, kredensial tersedia, terikat ke agen aktif).
- **Batas Percobaan**: Inbox menguji hingga 3 rute berbeda untuk aksi read-only sebelum berhenti. **Task eksekusi tidak pernah melakukan failover otomatis di tengah jalan** demi integritas file workspace.

### 1.5 Status & Health Task Runner
- **Runner Daemon**: Dikelola melalui [`runner.RunnerService`](file:///opt/cuan/releases/9509adad38a4a22e41dde3af1ec5e78168639c6a/data/projects/task-workspaces/8516c6c49ec359c7c8609dd0259aacc9c515603136dd7bc8d492c8b1b64d7285/internal/runner/service.go) dan tabel/store runner.
- **Heartbeat & Metrik**: Runner melaporkan status periodik (`idle`, `busy`, `offline`), utilisasi CPU (`CPUPercent`), memori (`MemoryUsedBytes`/`MemoryTotalBytes`), disk, serta `CurrentAttemptID`.
- **Reconciliation**: Lease yang habis masa berlakunya secara berkala direkonsiliasi menjadi status `interrupted`.

### 1.6 Analisis Kesenjangan (Gap Analysis)
Saat ini, context SM di Inbox (`inboxContext` di [`inbox_lifecycle.go`](file:///opt/cuan/releases/9509adad38a4a22e41dde3af1ec5e78168639c6a/data/projects/task-workspaces/8516c6c49ec359c7c8609dd0259aacc9c515603136dd7bc8d492c8b1b64d7285/internal/server/inbox_lifecycle.go#L328-L422)) hanya menerima:
1. Riwayat chat (maksimal 20 pesan terakhir, dipotong bila melebihi 24 KiB).
2. Hasil request sebelumnya yang gagal/denied (`Recorded request outcomes`).
3. Daftar proyek aktif yang terdaftar.
4. Catatan kontak dan informasi lampiran berkas.

**Ketiadaan Snapshot Kapasitas**: SM **sama sekali tidak mengetahui** apakah runner sedang online, berapa slot sesi yang tersisa, model mana yang sedang dalam masa cooldown, atau apakah akun backup aktif. Akibatnya, saat pengguna bertanya *"Apakah sistem siap jalan?"* atau *"Bagaimana kapasitas saat ini?"*, SM tidak memiliki data faktual dan cenderung memberikan jawaban spekulatif atau menyuruh pengguna *"Silakan cek di Settings → Agents"*—sebuah pola yang melanggar prinsip desain Cuan untuk pengguna nondeveloper.

---

## 2. Desain Sanitasi Data & Arsitektur Snapshot Agregat

### 2.1 Prinsip Desain Sanitasi (Zero Leakage Boundary)
Sesuai panduan integrasi Cuan ([`docs/product/integration-principles.md`](file:///opt/cuan/releases/9509adad38a4a22e41dde3af1ec5e78168639c6a/data/projects/task-workspaces/8516c6c49ec359c7c8609dd0259aacc9c515603136dd7bc8d492c8b1b64d7285/docs/product/integration-principles.md)):
1. **Tidak Ada Kredensial Sensitif**: Token OAuth, refresh token, client secret, kunci privat SSH, dan authorization code tidak boleh disuntikkan ke context prompt.
2. **Tidak Ada Path Internal atau ID Akun Mentah**: Path seperti `/opt/cuan/...` atau ID akun teknis seperti `acct_d52a31af2da6aa1ec35bf1fe942f63e5` tidak boleh terekspos. Sebagai gantinya, gunakan label logis agregat (misal: "Primary Account", "Backup Account #1").
3. **Agregasi Read-Only**: Informasi disajikan dalam bentuk status diskrit (`READY`, `DEGRADED`, `BUSY`, `BLOCKED`) dan persentase yang dibulatkan.

### 2.2 Pemetaan Data Mentah vs Data Tersanitasi

| Kategori | Data Internal Backend (Mentah) | Data Tersanitasi untuk Context SM | Alasan Keamanan / Privasi |
|---|---|---|---|
| **ID Akun** | `acct_d52a31af...`, `conn_profile_...` | Tipe peran: `"primary"` atau `"backup_pool"` | Mencegah leakage referensi internal database & storage |
| **Kredensial** | `antigravity_token`, Bearer JWT, Refresh Token | Dikeluarkan sepenuhnya (`omitted`) | Perlindungan kredensial absolut |
| **Path Sistem** | `/home/ubuntu/.local/share/cuan/accounts/...` | Dikeluarkan sepenuhnya (`omitted`) | Mencegah eksploitasi filesystem |
| **Kuota Provider** | Respon JSON RPC Cloud Code mentah, header HTTP | Sisa kuota per model teks: `~85%`, status: `healthy` / `cooldown` | Menyajikan fakta operasional tanpa ekspos API provider |
| **Slot Sesi** | Descriptor lock file `.session-*.lock` | Ringkasan agregat: `used: 1, total: 4, available: 3` | Transparansi konkurensi tanpa ekspos mekanisme kernel |
| **Task Runner** | Hostname server, IP address, Runner Token | Status runner: `online`, `status: idle`, beban: `normal` | Perlindungan identitas infrastruktur server |
| **Antrean Task** | ID tabel database `task_launches` | Jumlah task dalam antrean: `queued: 0` | Memberikan kepastian kesiapan tanpa kebocoran ID internal |

### 2.3 Kontrak DTO Snapshot Kapasitas (`AggregateCapacitySnapshot`)

Struktur data baru didefinisikan pada internal backend (misal di paket `internal/agent` atau `internal/server`):

```go
// AggregateCapacitySnapshot merepresentasikan status kesiapan dan kapasitas
// sistem yang aman untuk disuntikkan ke context SM Inbox.
type AggregateCapacitySnapshot struct {
    Timestamp           string              `json:"timestamp"`
    ReadinessStatus     string              `json:"readiness_status"` // "READY", "DEGRADED", "BUSY", "BLOCKED"
    SummaryNote         string              `json:"summary_note"`
    SessionSlots        SlotCapacityDTO     `json:"session_slots"`
    AccountsOverview    AccountOverviewDTO  `json:"accounts_overview"`
    ActiveModels        []ModelCapacityDTO  `json:"active_models"`
    RunnerStatus        RunnerCapacityDTO   `json:"runner_status"`
}

type SlotCapacityDTO struct {
    MaxConcurrent   int `json:"max_concurrent"`
    ActiveSessions  int `json:"active_sessions"`
    AvailableSlots  int `json:"available_slots"`
}

type AccountOverviewDTO struct {
    PrimaryConnected     bool `json:"primary_connected"`
    BackupAccountsCount  int  `json:"backup_accounts_count"`
    FailoverPoolActive   bool `json:"failover_pool_active"`
}

type ModelCapacityDTO struct {
    ModelName           string `json:"model_name"`
    Status              string `json:"status"` // "healthy", "cooldown", "unverified"
    RemainingPercent    int    `json:"remaining_percent,omitempty"` // -1 jika tidak terverifikasi
    CooldownRemainingSec int   `json:"cooldown_remaining_sec,omitempty"`
    IsPrimaryDefault    bool   `json:"is_primary_default"`
}

type RunnerCapacityDTO struct {
    OnlineRunnersCount  int    `json:"online_runners_count"`
    RunnerState         string `json:"runner_state"` // "idle", "busy", "offline"
    QueuedTasksCount    int    `json:"queued_tasks_count"`
    SystemLoadStatus    string `json:"system_load_status"` // "normal", "high", "exhausted"
}
```

### 2.4 Mekanisme Pengumpulan Non-Blocking

Agar pengumpulan data kapasitas tidak memperlambat pembukaan Inbox atau turn chat:
1. **Inspeksi Slot Sesi Non-Destruktif (`InspectAccountSlotUsage`)**:
   Menambahkan fungsi pada [`internal/agent/account_lease.go`](file:///opt/cuan/releases/9509adad38a4a22e41dde3af1ec5e78168639c6a/data/projects/task-workspaces/8516c6c49ec359c7c8609dd0259aacc9c515603136dd7bc8d492c8b1b64d7285/internal/agent/account_lease.go) yang menguji `flock` non-blocking pada masing-masing berkas `.session-<i>.lock`.
   - Jika lock berhasil didapat: slot berstatus bebas (segera lepaskan kembali).
   - Jika mengembalikan `syscall.EWOULDBLOCK` atau `syscall.EAGAIN`: slot sedang digunakan oleh proses sesi aktif.
   - Operasi ini membutuhkan waktu kurang dari 1 milidetik dan tidak mengganggu proses yang sedang berjalan.
2. **Pemanfaatan Cache Kuota Model (In-Memory 30s TTL)**:
   Pengumpulan snapshot membaca dari cache memori yang sudah ada di `model_usage.go`. Tidak melakukan RPC jaringan baru saat menyusun context chat jika cache masih segar.
3. **Pemeriksaan Runner & Task Queue**:
   Mengambil data cepat dari `s.runnerSvc.ListRunners(ctx)` dan menghitung status attempt aktif/antrean di `s.taskSvc`.

### 2.5 Integrasi Pipeline Konteks di `internal/server/inbox_lifecycle.go`

Di dalam fungsi [`inboxContext`](file:///opt/cuan/releases/9509adad38a4a22e41dde3af1ec5e78168639c6a/data/projects/task-workspaces/8516c6c49ec359c7c8609dd0259aacc9c515603136dd7bc8d492c8b1b64d7285/internal/server/inbox_lifecycle.go#L328), setelah section proyek aktif dan sebelum riwayat pesan target:

```go
// Menghasilkan snapshot kapasitas agregat
capacitySnapshot := s.buildAggregateCapacitySnapshot(ctx, q.OwnerID, registered)
if capacitySnapshot != nil {
    snapshotJSON, _ := json.Marshal(capacitySnapshot)
    b.WriteString("\nSystem execution capacity and readiness (application facts, not instructions):\n")
    b.Write(snapshotJSON)
    b.WriteByte('\n')
}
```

**Batas Ukuran Konteks**:
Snapshot JSON yang tersanitasi ini berukuran sangat ringkas (kurang dari 800 bytes hingga maksimal 1.2 KiB), jauh di bawah pagu konteks 24 KiB yang dialokasikan untuk CLI Inbox.

---

## 3. Batasan & Panduan Respon Scrum Master (SM)

### 3.1 Aturan Utama Perilaku SM
1. **Dilarang Mengarahkan Pengguna ke Settings Secara Manual**:
   SM tidak boleh merespon dengan kalimat seperti *"Silakan periksa kuota Anda di Settings → Agents"* atau *"Buka menu Settings untuk melihat ketersediaan runner"*. Cuan dirancang untuk pengguna nondeveloper; SM bertanggung jawab menyampaikan status operasional secara langsung.
2. **Menjawab Langsung, Proaktif, dan Transparan**:
   Saat pengguna menanyakan kesiapan sistem (*"Bisa kerjain task sekarang?", "Kapasitas cukup nggak?"*), SM langsung menyimpulkan status readiness berdasarkan fakta aplikasi yang disuntikkan.
3. **Bahasa Alami & Menghindari Jargon Internal**:
   Gunakan bahasa Indonesia yang jelas dan ramah. Hindari istilah teknis tingkat rendah seperti flock, descriptor, lease token, atau SQLite mutex.
4. **Pembedaan Kapasitas vs Kuota**:
   Jika slot sesi penuh, jelaskan bahwa server sedang menjalankan sesi task lain secara bersamaan. Jika kuota provider habis, jelaskan ketersediaan model backup atau perkiraan waktu reset cooldown.

### 3.2 Matriks Skenario & Rekomendasi Respon SM

| Kondisi Sistem | Data Snapshot | Rekomendasi Tindakan & Respon SM |
|---|---|---|
| **Skenario 1: Normal / Siap Penuh** | `readiness_status: "READY"`, `available_slots: 3`, `runner_state: "idle"` | **Respon Langsung**: Mengonfirmasi sistem siap menjalankan task segera. Sebutkan model default yang aktif dan ketersediaan slot sesi. Tawarkan eksekusi atau formulasi task langsung jika user memiliki instruksi. |
| **Skenario 2: Slot Sesi Penuh (Konkurensi Maksimal)** | `readiness_status: "BUSY"`, `available_slots: 0`, `active_sessions: 4` | **Respon Transparan**: Jelaskan bahwa seluruh slot eksekusi (4 slot) sedang bekerja menangani task aktif. Jelaskan bahwa task baru tetap dapat dibuat dan akan masuk antrean otomatis segera setelah salah satu sesi selesai. |
| **Skenario 3: Model Utama Cooldown, Failover Siap** | `readiness_status: "DEGRADED"`, model default `cooldown: 180s`, model backup `healthy (~70%)` | **Respon Proaktif**: Jelaskan bahwa model utama sedang beristirahat sementara (cooldown ~3 menit), namun rute cadangan (misal Claude Sonnet / backup pool) aktif dan siap menangani tugas tanpa hambatan. |
| **Skenario 4: Seluruh Rute Kuota Habis** | `readiness_status: "BLOCKED"`, seluruh model `status: "cooldown"` / `quota_limited` | **Respon Solutif**: Informasikan dengan jujur bahwa kuota provider AI sedang mencapai batas pada seluruh rute yang diizinkan dan sebutkan perkiraan waktu reset cooldown terdekat. Sampaikan bahwa ide/instruksi pengguna tetap aman tersimpan di chat. |
| **Skenario 5: Runner Offline** | `readiness_status: "BLOCKED"`, `online_runners_count: 0` | **Respon Faktual**: Jelaskan bahwa task runner lokal sedang tidak aktif sehingga eksekusi belum dapat dimulai secara fisik. SM dapat membantu menyusun draft rencana kerja terlebih dahulu di chat sambil menunggu layanan runner kembali online. |

### 3.3 Pembaruan Kontrak & Instruksi Koordinator
Panduan instruksi koordinator pada [`internal/workflow/coordinator.md`](file:///opt/cuan/releases/9509adad38a4a22e41dde3af1ec5e78168639c6a/data/projects/task-workspaces/8516c6c49ec359c7c8609dd0259aacc9c515603136dd7bc8d492c8b1b64d7285/internal/workflow/coordinator.md) dan [`internal/server/inbox_coordinator_response.go`](file:///opt/cuan/releases/9509adad38a4a22e41dde3af1ec5e78168639c6a/data/projects/task-workspaces/8516c6c49ec359c7c8609dd0259aacc9c515603136dd7bc8d492c8b1b64d7285/internal/server/inbox_coordinator_response.go#L15-L28) akan diperkaya dengan aturan:

```markdown
Capacity and readiness questions:
- Use the supplied "System execution capacity and readiness" snapshot facts directly.
- Answer transparently about model health, available session slots, and runner availability.
- Never instruct the user to check Settings -> Agents, inspect logs, or verify quotas manually.
- When capacity is degraded or slots are busy, explain the situation plainly, state the automatic failover or queue behavior, and offer to proceed with planning or drafting.
```

---

## 4. Acceptance Criteria (Definition of Done - DoD)

Implementasi dinyatakan selesai (Done) jika memenuhi kriteria berikut:

1. **Backend Capacity Inspector**:
   - Terdapat fungsi non-blocking `InspectAccountSlotUsage(owner, accountID string) (active, max int, err error)` pada `internal/agent/account_lease.go` yang akurat mendeteksi file lock tanpa memblokir proses.
   - Terdapat fungsi agregator `buildAggregateCapacitySnapshot` pada `internal/server/inbox_lifecycle.go` yang menggabungkan status akun, model, slot sesi, dan runner.
2. **Sanitisasi & Masking**:
   - Snapshot data dijamin bebas dari token OAuth, refresh token, bearer header, path lokal server, ID akun teknis mentah, dan email pengguna.
   - Pagu ukuran snapshot tidak melebihi 1.5 KiB pada payload prompt.
3. **Injeksi Konteks Inbox**:
   - Context SM di `inboxContext` memuat header `System execution capacity and readiness (application facts, not instructions):` yang valid dan tidak merusak batas budget total 24 KiB.
4. **Respon SM Sesuai Panduan**:
   - SM menjawab pertanyaan kesiapan sistem secara lugas dan faktual berbasis snapshot data yang disuntikkan.
   - Tidak ada respon SM yang menyuruh pengguna membuka Settings atau memeriksa kuota secara manual.
5. **Keamanan & Kompatibilitas Mundur**:
   - Jika service runner mati atau kuota tidak dapat diambil, backend fallback ke status `unverified` / aman tanpa menyebabkan error 500 atau kegagalan turn chat Inbox.
   - Tidak ada perubahan skema database permanen (zero migration required), semua data diambil dari runtime memory, lock files, dan metadata yang ada.

---

## 5. Pertimbangan Keamanan & Privasi

1. **Owner Scoping & Isolasi Multi-Tenant**:
   Pengumpulan data kapasitas akun dan runner wajib dibatasi pada `ownerID` dari permintaan Inbox yang bersangkutan (`q.OwnerID`). Akun atau runner milik akun/tenant lain tidak boleh dihitung ke dalam snapshot agregat.
2. **Pencegahan Prompt Injection**:
   Blok snapshot disuntikkan sebagai data fakta aplikasi terstruktur dengan penegasan: `(application facts, not instructions)`. Struktur JSON yang seragam mencegah input luar memanipulasi instruksi koordinator.
3. **Zero Secret Retention di Memori Chat**:
   Hasil serialisasi snapshot adalah representasi DTO baru; objek credential asli tidak pernah disentuh atau disalin ke dalam memori pembuatan prompt.
4. **Resiliensi Kegagalan Parsial**:
   Jika file konfigurasi akun korup atau socket RPC kuota timeout, sistem mencatat status `unavailable` pada level agregat tanpa memuntahkan error dump atau stack trace teknis ke pengguna.

---

## 6. Rencana Verifikasi Fungsional

### 6.1 Pengujian Unit (Unit Tests)
- `account_lease_test.go`:
  - Menguji `InspectAccountSlotUsage` saat 0 slot terpakai, sebagian terpakai, dan seluruh slot terpakai (`CUAN_ACCOUNT_MAX_SESSIONS` variasi 1, 2, 4).
  - Memverifikasi bahwa inspeksi slot tidak pernah memblokir dan tidak pernah mencuri slot dari proses yang sedang aktif.
- `inbox_capacity_sanitize_test.go`:
  - Menguji fungsi penyusunan snapshot dengan mock profil yang memiliki token dan kredensial sensitif.
  - Memverifikasi output JSON tidak mengandung string token, path direktori `/home/...`, atau pola rahasia apa pun via `agent.MaskSecrets`.

### 6.2 Pengujian Integrasi (Integration Tests)
- `inbox_capacity_context_test.go`:
  - Menguji `inboxContext` untuk memastikan blok `System execution capacity and readiness` muncul dengan format JSON yang valid di dalam prompt.
  - Memastikan ukuran total prompt tetap berada dalam batas toleransi (< 24 KiB).
- `inbox_sm_capacity_response_test.go`:
  - Menguji pemanggilan `executeInboxCLIWithSafety` dengan prompt simulasi pertanyaan kesiapan eksekusi (*"Apakah sistem siap mengeksekusi tugas?"*).
  - Memverifikasi bahwa respon SM menyebutkan kesiapan slot dan model tanpa kata kunci *"Settings"* atau instruksi cek manual.

### 6.3 Pengujian Skenario Failover & Ekstrem
- Simulasi kondisi seluruh slot terisi: Verifikasi snapshot melaporkan `available_slots: 0` dan SM menginformasikan sistem sedang sibuk.
- Simulasi kuota model utama habis: Verifikasi snapshot melaporkan model utama dalam `cooldown` dan model backup siap, serta SM mengonfirmasi delegasi dapat dialihkan ke model cadangan secara transparan.

---

## 7. Rangkuman & Rekomendasi Eksekusi
Rencana arsitektur ini memberikan solusi elegan, ringan, dan aman tanpa memerlukan migrasi database maupun penambahan dependensi eksternal. Dengan menyuntikkan fakta kapasitas agregat langsung ke context SM Inbox, pengalaman pengguna nondeveloper di Cuan meningkat secara signifikan sesuai visi produk: sistem bekerja secara otonom dan transparan, sementara detail teknis ditangani oleh agen di balik layar.
