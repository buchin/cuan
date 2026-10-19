# Adopsi pesan dan task Fural di Cuan

## Otorisasi dan tujuan

Pengguna meminta adopsi siklus pesan Fural, kemudian memperluas tujuan menjadi
“implementasi pesan dan task fural ke cuan”. Instruksi eksplisit 2026-10-19:
“kali ini ga pake scrum dulu, gas disini.” Pekerjaan langsung pada repository Cuan,
bukan sprint baru dan tidak mengubah status/review S26.

Outcome: pengguna dapat mengirim beragam pesan, mengirim susulan saat agen bekerja,
melihat progres nyata, membatalkan, pulih setelah refresh/reconnect, dan mengubah
pesan menjadi task pada board utama. Task memiliki sumber percakapan, attempt,
output tersimpan, hasil review, dan pembatalan yang dapat diamati tanpa sesi internal.

## Scope dan bukti

| Area | Implementasi | Bukti |
| --- | --- | --- |
| Penerimaan | Atomic message/request, UUID/hash reconciliation, kapasitas 20 | TestInboxLifecycleQueueCapacityAndIdempotency, SQL disposable, S264 timeout regression |
| Eksekusi | FIFO claim lintas proses; satu turn per room; maksimal empat room per proses | TestInboxLifecycleFIFOAndCancellation, TestInboxLifecycleContextFIFOStreamAndTask, SQL parallel claims |
| Konteks | History 20 pesan; history encoded dibatasi 24 KiB; pesan target utuh; tidak memasukkan follow-up masa depan | HTTP/child fixture context assertions |
| Lifecycle | queued/running/cancelling/completed/failed/interrupted/cancelled; state terminal tidak hidup kembali | Memory + SQL cancellation/late output and OS-process restart |
| Progres | Milestone aplikasi; SSE snapshot dengan Bearer header; catch-up dan fallback | HTTP SSE auth/snapshot, Chromium live stream; transport chunk/UTF-8/CRLF/idle-abort tests |
| UI | Status ringkas, elapsed, detail terlipat, cancel, follow-up, scroll pengguna, source-task link | Chromium 360/390/1280; browser-results.json |
| Task | Canonical board, deterministic capture per message, project wajib, source room/message persisted | TestMessageTask*, HTTP source-scope negative tests, SQL source task |
| Attempt | Durable reservation, duplicate launch rejection, numbered local attempts, lease recovery, immutable terminal state | Memory + SQL tests; real PTY fixture browser flow |
| Hasil | Exit 0 menuju review; gagal/cancel blocked; callback lama tidak menimpa run baru | TestMessageTaskDelayedExitDoesNotOverwriteNewAttempt; SQL reconciliation; Chromium |
| Output | Complete lines, credential redaction, 256 KiB/1000 lines, retained output panel | Split-credential test, SQL store reload, Chromium saved-output check |
| Operasi | Additive migration 21; manual schema gate; documented native release/rollback | Disposable PostgreSQL 16.4 migration + server/migrator build; guide |

Konversi approval Inbox lama diarahkan ke canonical capture. Tidak lagi memulai
job Gemini lain, mengumumkan sprint berjalan, atau menandai DoD lulus tanpa bukti.
Membaca daftar task kosong tidak lagi membuat task contoh otomatis.

## Batas dan gate

CLI fixture bukan bukti keberhasilan provider nyata. Tidak ada consent baru,
perpindahan credential, rotasi akun atau bypass kuota. Chat meminta jawaban dan
klarifikasi; pekerjaan yang memerlukan eksekusi ditangani melalui task eksplisit.
Attachment/voice baru, roundtable multi-agent, tool broker baru, dan exactly-once
untuk efek eksternal berada di luar scope. Hasil provider yang tidak diketahui
menjadi interrupted tanpa replay otomatis.

Migration 21 menambah lifecycle serta metadata sumber task. Migrasi lama tetap
utuh. Tidak ada schema/deploy production pada pekerjaan ini. Sebelum penerapan
berdata: backup privat, staging, quiesce pekerjaan, explicit migration dengan
auto-migrate off, native atomic release, health/readiness, dan rollback aplikasi
sambil mempertahankan schema additive. Lihat panduan penggunaan/operasi.

## Verifikasi final — 2026-10-19

Targeted Go checks, PostgreSQL 16.4 disposable dengan proses terpisah, transport
SSE, TypeScript/Vite serta server/migrator build lulus. Hash sumber yang diuji
tercatat di docs/evidence/message-task/verification.json; tidak ada drift kode
setelah pemeriksaan. Bukan full suite.

Publikasi native preview fixture pada 12:28 sempat mengembalikan origin 504,
tetapi pemeriksaan proses membuktikan boot final tetap berjalan. Tidak ada boot
ulang duplikat. Chromium kemudian membuktikan JS/CSS yang dilayani identik byte
per byte dengan build final, SSE snapshot pertama 453 ms, terminal Connected,
serta 12 alur end-to-end. Viewport 360/390/1280 tidak overflow dan tidak ada error
JavaScript. Hasil dan checksum aset ada di browser-results.json; screenshot
running-360.png, inbox-360.png, inbox-390.png, inbox-1280.png, task-review-1280.png.

Preview: https://cuan-message-lifecycle-dojo.fural.space. Ini hanya data fixture,
HOME sementara dan provider palsu; tidak ada credential/history production.
Provider nyata tidak dipanggil. Pengujian viewport bukan sertifikasi perangkat
fisik/keyboard. Warning CSS/chunk bawaan frontend tetap tercatat.

Implementasi, panduan dan bukti siap dikomit pada branch kerja yang ada. Jangan
merge main atau menghapus branch S26 yang belum merged dari pekerjaan ini.
Production tetap tidak diubah; penerapan berdata memerlukan prosedur backup,
explicit migration 21 dan native release/rollback pada panduan.
