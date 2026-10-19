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
Claude, Codex, dengan argumen masing-masing. Chat diarahkan untuk jawaban dan
klarifikasi; eksekusi pekerjaan dimulai melalui task eksplisit.

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
Belum ada penerapan schema atau release production dari pekerjaan ini.

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
respons provider nyata, pengujian perangkat fisik atau release production.
Spesifikasi dan bukti final: docs/plans/message-task-adoption.md.
