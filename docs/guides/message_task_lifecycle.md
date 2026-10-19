# Pesan dan task di Cuan

## Mengirim dan memantau pesan

Inbox menerima pesan dengan ID permintaan yang disimpan sebelum pengiriman.
Saat respons HTTP hilang, gunakan pemeriksaan pengiriman yang sama. Jangan
mengganti ID untuk mengulang pekerjaan yang hasilnya belum pasti.

Pesan masuk ke antrean FIFO per percakapan. Satu agen membalas satu pesan pada
satu waktu; pesan susulan tetap bisa dikirim. Agen menerima maksimal 20 pesan
konteks terakhir, dengan batas encoded history 24 KiB (pesan target tetap utuh). Pesan susulan yang belum gilirannya
tidak dimasukkan ke turn sebelumnya. Sapaan, pertanyaan substantif, dan kebutuhan
klarifikasi tidak lagi dipaksa menjadi template sapaan 1–2 kalimat.

Status menampilkan menunggu giliran, proses berjalan, pembatalan, selesai, gagal,
atau terputus. Detail aktivitas menyimpan ID pesan dan milestone nyata aplikasi.
Progres tidak berisi stderr, credential, reasoning privat, atau persentase rekaan.
SSE mengirim snapshot tersimpan; reconnect dan refresh membaca ulang hasil tanpa
mengeksekusi request yang sama. Saat stream terputus, UI memeriksa riwayat berkala.

Batalkan menghentikan pekerjaan yang menunggu. Untuk proses aktif, status menjadi
menghentikan sampai child process dihentikan; balasan terlambat tidak diterbitkan.
Efek provider yang sudah terjadi tidak bisa ditarik kembali. Setelah process loss,
request aktif melewati deadline menjadi interrupted, tanpa replay otomatis.

Batas: 20 request aktif/menunggu per room, empat worker per proses; body pesan maksimal 64 KiB; proses chat
maksimal 3 menit dan lease 4 menit. Output CLI maksimal 1 MiB. Jenis CLI: agy,
Claude, Codex, dengan argumen masing-masing. Chat dapat membaca konteks project untuk analisis dan rekomendasi sprint;
implementasi yang perlu dilacak dimulai melalui task eksplisit.

### Akses project dalam chat

Sebut nama project, misalnya **cek repo cuan sarankan sprint**. Jika hanya ada
satu project aktif, chat memakai project itu. Room project terikat pada project
room tersebut. Dalam DM dengan beberapa project, sebut satu nama project; jika
ambigu agent meminta klarifikasi. Tidak perlu membuat task untuk membaca repo
atau menyusun rekomendasi.

Agent menerima snapshot maksimal 24 KiB berisi backlog (40 task terbaru), commit
lokal, daftar file terlacak, README dan cuplikan dokumen. Workspace berasal dari
project terdaftar, bukan path bebas dalam pesan. HOME login agent tetap terpisah
dari direktori kerja repo. Agy memakai mode plan+sandbox, Codex sandbox read-only,
Claude hanya tools Read/Glob/Grep dalam mode plan. Kemampuan pembatasan native
mengikuti harness masing-masing; ini bukan sandbox filesystem lintas owner.
Snapshot tidak membaca environment files atau symlink keluar repo.

Hubungkan/sinkronkan repo melalui **Projects**. Jika workspace kosong/tidak ada,
agent masih dapat membahas backlog dan harus menjelaskan bahwa inspeksi kode
belum tersedia. Tidak ada fetch GitHub otomatis dalam setiap turn; commit adalah
snapshot checkout lokal. Issue/PR remote tidak dianggap sudah diperiksa.
Production Cuan memakai checkout privat di data project persisten, dipasok dari
Git bundle release tanpa credential; pembaruan checkout memakai Git bundle baru
atau sinkronisasi repository dengan credential project milik owner.

## Format balasan agent

Balasan agent di Inbox mendukung Markdown: heading, paragraf, bold/italic,
inline code, daftar bertingkat, kutipan, checklist, tautan web dan tabel.
Code block memiliki tombol **Copy**; jika clipboard ditolak dan fallback gagal,
pilih teks kode lalu salin manual. Code block dan tabel lebar dapat digeser
horizontal tanpa melebarkan halaman pada mobile. Pesan user tetap ditampilkan
sebagai teks literal. Format diterapkan juga pada riwayat pesan yang sudah ada;
isi pesan yang disimpan tidak diubah.

Raw HTML dinonaktifkan. URL berbahaya, URL `file://` dan protocol-relative tidak
menjadi tautan aktif. Referensi file lokal tetap berupa label teks; preview repo
belum termasuk fitur ini. Gambar Markdown ditampilkan sebagai keterangan teks,
bukan dimuat dari host luar. Tautan HTTP(S) dibuka di tab baru dengan perlindungan
`noopener noreferrer`. Konten yang belum lengkap tetap dapat dirender saat
snapshot pesan berubah. Tidak ada eksekusi kode dari code block atau syntax
highlighting pada versi ini.

Verifikasi renderer memakai fixture terisolasi di Chromium dengan komponen
InboxView nyata: `node docs/evidence/chat-markdown/browser-check.cjs`.
Bukti: `docs/evidence/chat-markdown/browser.json`.

## Mengubah pesan menjadi task

Pilih **Buat / lihat task** di bawah pesan, isi judul dan pilih project yang benar,
lalu **Simpan task**. Pesan sumber menjadi instruksi task. Tindakan ini membuat
backlog di board utama dan tidak otomatis memulai agent. Pengulangan dari pesan
sumber yang sama membuka task yang sama, termasuk setelah refresh.

Buka task, pilih agen terdaftar dan mulai sesi dari halaman task. Repository
project harus sudah tersinkron bila project terhubung ke repository. Sesi yang
aktif tidak boleh diduplikasi. Attempt disimpan terpisah dari status task. Exit 0
mengarah ke review, bukan bukti bahwa DoD atau implementasi telah selesai.
Status task tertaut juga terlihat pada pesan sumber setelah reload. Output attempt
disimpan maksimal 256 KiB/1000 baris dengan redaksi credential; buka **Saved attempt
output**. Terminal terputus dapat dibuka lagi melalui **Reconnect terminal**.
Gagal/cancel mengarah ke blocked. Cancel run menghentikan PTY lokal. Lease lokal
11 menit menandai hasil interrupted jika proses tidak lagi melapor; tidak replay.

## Kontrak API

Semua endpoint internal berikut memerlukan session pemilik/Bearer header. Jangan
menaruh token di query URL. Cuan saat ini memakai model single-owner.

- `POST /api/v1/chat/conversations/:id/messages`: `request_id` UUID dan `message`.
  202 untuk queued/running, 409 jika key dipakai untuk payload berbeda, 429 bila
  antrean penuh. Retry dengan key yang sama merekonsiliasi hasil. Klien lama tanpa
  request_id tetap didukung, tetapi tidak boleh mengulang POST yang hasilnya
  belum pasti; periksa history terlebih dahulu.
- `GET /api/v1/chat/conversations/:id/messages`: messages dan request snapshots.
- `GET /api/v1/chat/conversations/:id/events`: fetch SSE dengan event `snapshot`.
  Reconnect saat koneksi ditutup; server membatasi stream sekitar 45 detik. Client
  membatalkan koneksi yang diam 25 detik dan pengiriman yang belum terkonfirmasi
  selama 15 detik; ID tetap dipertahankan.
- `POST /api/v1/chat/conversations/:id/requests/:requestID/cancel`: rekonsiliasi
  pembatalan; gunakan GET/SSE untuk status akhir.
- `POST /api/v1/chat/conversations/:id/messages/:messageID/task`:
  `project_id`, `title`. Hasil `{task, source_message_id, status}` pada board utama.
  GET/SSE percakapan menyertakan daftar task sumber. Approval thread lama sekarang
  juga membuat canonical backlog; jangan mengasumsikan responsnya berarti doing.
- `POST /api/v1/tasks/:id/run-agent`: `agent_id` terdaftar. Mengembalikan sesi dan
  attempt; konflik sesi aktif 409. Pantau detail task dan terminal.
- `POST /api/v1/tasks/:id/cancel`: menyimpan cancel dan menghentikan terminal lokal.

## Migrasi dan operasi

Perubahan ini membutuhkan additive migration 000021 setelah 000020. Jangan edit
migrasi lama atau mengaktifkan auto-migrate. Sebelum release pada lingkungan yang
menyimpan data, ambil backup privat dan jalankan migrator secara eksplisit dengan
DATABASE_URL privat. Jangan menulis URL credential dalam log atau argumen shell.

Quiesce pekerjaan sebelum mengganti versi. Untuk rollback aplikasi, selesaikan
atau batalkan queued/running/cancelling dan pertahankan schema additive; jangan
jalankan down migration untuk rollback aplikasi biasa. Gunakan release native
repo dengan lock, checksum, atomic activation, health/readiness dan rollback.
Production https://cuan.dojo.cc telah memakai schema 21 dan release
`dedf65dc384055c0aa846146ed9b73a4b543b7b5` sejak 2026-10-19.
Backup privat sebelum migrasi dan uji rollback/aktivasi kembali lulus.
Rollback aplikasi tetap mempertahankan schema 21; gunakan:

```sh
sudo python3 /home/ubuntu/.local/share/cuan-s25/activate-release.py --rollback --schema-count 21
# Aktivasi kembali release terverifikasi:
sudo python3 /home/ubuntu/.local/share/cuan-s25/activate-release.py --release dedf65dc384055c0aa846146ed9b73a4b543b7b5 --schema-count 21
```

Pastikan antrean/attempt tidak aktif sebelum rollback. Jangan gunakan instruksi
schema20 lama untuk deployment ini. Bukti: `docs/evidence/message-task/production/`.

## Verifikasi dan checkpoint 2026-10-19

Targeted tests `TestInboxLifecycle*`, `TestMessageTask*`, safe-key timeout dan
request-lifetime terminal lulus. PostgreSQL 16.4 disposable menjalankan migration
serta klaim paralel, cancel/late output, canonical task/attempt, dan regression
OS-process restart: completed tetap satu eksekusi; crash menjadi interrupted
juga satu eksekusi. Semua respons adalah fixture, bukan keberhasilan provider.

TypeScript/Vite dan server/migrator build lulus; warning CSS/chunk bawaan tetap
ada. Chromium membuktikan 12 alur pada preview fixture final, termasuk SSE,
FIFO/cancel, refresh, source-task link, terminal Connected, attempt review dan
output tersimpan. JS/CSS yang dilayani identik dengan build lokal. Viewport
360/390/1280 tanpa overflow dan tanpa error JavaScript.

Bukti: docs/evidence/message-task/verification.json dan browser-results.json.
Jalankan ulang pemeriksaan terarah dengan:

```sh
go test ./internal/db ./internal/task ./internal/server ./internal/terminal -run 'TestInboxLifecycle|TestMessageTask|TestS264TimeoutAfterAcceptanceSafeRetry|TestS25AgentOutlivesRequestAndPromptIsNotShellCode|TestManager_RunTaskAgent|TestInboxHandlers_CompleteFlow|TestS26DedicatedAgentRegistrationAndRuntimeIsolation' -count=1
node web/src/api/chat-stream.test.cjs
```

Test SQL memerlukan CUAN_LIFECYCLE_SQL_DSN menuju database disposable; test membuat
schema unik dan tidak memigrasikan public/staging/production. Browser-check.cjs
memerlukan Playwright/Chromium dan preview fixture yang disebut di atas, tanpa
response mock/asset overlay. Source preview opt-in: TestInboxLifecycleBrowserPreview.

Preview memakai temporary HOME, data memori dan provider palsu. Tidak ada klaim
respons provider nyata, pengujian perangkat fisik atau bukti provider production.
Smoke production terpisah lulus untuk login owner, API, UI Inbox, viewport
360/390/1280, serta kecocokan aset publik; tidak memanggil provider.
Spesifikasi dan bukti final: docs/plans/message-task-adoption.md.

## Verifikasi akses project production

Pada 2026-10-19, satu request nyata ke @pin2 berhasil membaca
`internal/server/inbox_project.go` pada checkout release `ccc45c8`. Balasan
mengidentifikasi `inboxProjectContext`, `truncateProjectText`, `budget := 32000`,
dan pemilihan project mengikuti `room.ProjectID` pada room non-agent, sesuai kode.
Tidak membuat task dan tidak mengubah kode repo. Ini bukti provider nyata yang
terpisah dari pengujian fixture sebelumnya. Request test diberi label verifikasi
dan tetap tersimpan di Inbox; pesan pengguna sebelumnya tidak dikirim ulang.
Bukti: `docs/evidence/chat-project/provider-read.json` dan `release.json`.

## Verifikasi Markdown production

Release `dedf65d` aktif pada 2026-10-19, schema tetap 21. Backup privat tervalidasi
sebelum native activation. Chromium membaca riwayat production tanpa mock dan
membuktikan 3 balasan agent memakai renderer baru, termasuk bold, list bertingkat,
dan inline code. Aset publik identik dengan build lokal. Login owner/API,
viewport 360/390/1280, dan pemeriksaan error JavaScript lulus. Tidak ada pesan
production baru atau panggilan provider untuk pengujian format ini.

Kasus tabel lebar, code copy, penolakan clipboard, raw HTML/URL berbahaya, dan
snapshot belum lengkap diverifikasi terpisah memakai fixture InboxView terisolasi.
Bukti: `docs/evidence/chat-markdown/production-browser.json` dan `release.json`.
