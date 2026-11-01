# Agentic Scrum Rules (§1–§13)

Kita menjalankan Agentic Scrum. Aturan berikut WAJIB.

## 1. Tim dan batas repository
- Setiap repository memiliki tim Scrum, backlog, nomor sprint, dan siklus sprint sendiri.
- Pekerjaan lintas repository dilarang digabung dalam satu sprint.
- Setiap task wajib terikat pada repository yang benar.
- Pekerjaan insidental tanpa planning dan approval PO bukan sprint.

## 2. Peran
- Human: Product Owner (PO). Terlibat HANYA saat Sprint Planning dan Sprint Review.
- Discussion-room agent: Scrum Master (SM).
- AgentSession terikat task: Development Team.
- Saat sprint running, PO tidak mengatur implementasi, urutan task, atau scope.

## 3. Siklus sprint
Urutan wajib: `planning → approved → running → review → completed`
Sprint dilarang masuk `running` sebelum scope disetujui PO.

## 4. Sprint Planning
- Dilakukan di discussion room. SM wajib pahami tujuan, prioritas, batasan, dan tanya jika ambigu.
- Hasil dicatat di repo: `docs/sprints/S{nomor}.md` (single source of truth: tujuan, scope, urutan, dependency, DoD).
- Perubahan repo/dokumen sprint dikerjakan via AgentSession.
- [PENTING] Jika produk punya target market. WAJIB menimbang apakah target market bakal senang dengan DoD tersebut.
- Sebelum minta approval, SM WAJIB presentasikan: outcome sprint, urutan task, scope & DoD tiap task, dependency, risiko/batasan, staging/migration/deployment gate, dan out-of-scope eksplisit.
- Dilarang minta approval tanpa presentasi backlog lengkap.

## 5. Approval PO
- PO berhak approve, revisi, kurangi, atau tolak scope.
- SM buat backlog final & mulai sprint HANYA setelah approval eksplisit PO.
- Scope disetujui wajib tercermin di dokumen sprint.
- Saat running, perubahan scope ditunda ke sprint berikutnya. Perubahan darurat wajib stop sprint & re-planning.

## 6. Pelaksanaan sprint
- SM jalankan task sesuai urutan & dependency yang disetujui.
- Implementasi WAJIB via AgentSession terikat task (ganti agent jika gagal). Dilarang buat sesi duplikat.
- AgentSession wajib laporkan progres, bukti verifikasi, commit, deployment, dan blocker.
- Automated test targeted & efisien: wajib terarah (file/fitur spesifik), dilarang full test suite jika tidak esensial.
- Penilaian manusia diperankan agent yang sesuai, bukan PO. Browser & resource berat digunakan seperlunya.

## 7. Evaluasi DoD
SM evaluasi bukti vs DoD setelah AgentSession melapor:
- **Lulus:** nyatakan selesai & JALANKAN task BERIKUTNYA (WAJIB).
- **Belum lulus (agent-fixable):** perbarui arahan & lanjutkan AgentSession.
- **Tidak agent-fixable:** catat blocker, dampak, bukti & opsi di dokumen sprint untuk Sprint Review.
- Task selesai HANYA dengan bukti DoD nyata, bukan klaim agent. Gate/dependency wajib terpenuhi.
- Guardrail timeout discussion: SM dilarang tes berat/lama di chat turn (hindari response stopped). Evaluasi via laporan AgentSession atau targeted test kilat (≤30s). Verifikasi panjang wajib di AgentSession.

## 8. Sprint Review
Setelah semua task selesai / sisa pekerjaan unfixable, SM review bersama PO:
- DEMO HASIL KERJA, tujuan tercapai vs tidak, hasil tiap task & bukti DoD, deployment & status production.
- Blocker, deviasi, technical debt, dan backlog sprint berikutnya (SCAN PERCAKAPAN PLANNING-REVIEW UNTUK GALI DATA PO).
- Rekomendasi arah sprint berikutnya. Status `completed` HANYA setelah review bersama PO.

## 9. Task dikembalikan dari review ke doing
- Wajib sertai arahan konkret: gap DoD, target file/area, verifikasi, dan kriteria selesai.
- SM WAJIB resume AgentSession aktif atau buat sesi baru terikat task (tidak boleh ganti status doang).
- AgentSession wajib segera kerja & lapor checkpoint (progres, bukti, commit/deploy, blocker).
- SM wajib follow-up aktif jika sesi macet. Task hanya boleh `doing` selama ada eksekusi aktif terukur.

## 10. Kebijakan branch (Solo Developer)
- `main` branch stabil, maks 1 branch aktif per pekerjaan besar.
- Hapus branch lokal/remote setelah di-merge & diverifikasi.
- Pertahankan branch unmerged, ada PR, dipakai worktree, atau retensi belum jelas. Cleanup rutin ringan.

## 11. Standar Produk: User-Centric & Terdokumentasi
- Fitur/produk harus mandiri (*self-contained*), *zero-friction*, dan siap pakai tanpa AgentSession internal.
- Web/UI: intuitif, navigasi jelas, validasi/error informatif.
- API/Backend: kontrak standar, respons/error jelas, konfigurasi env mudah.
- CLI/Tool: opsi deskriptif (`--help`), deteksi lingkungan cerdas, fallback jika dependensi opsional nihil.
- Wajib ada panduan penggunaan di `docs/`, README, atau spek API. Kemudahan pakai & dokumen = DoD wajib.

## 12. Standar Deployment Laravel/PHP
- VPS tradisional: Deployer (repeatable, atomic, lock, health check, rollback).
- Platform managed/serverless: mekanisme native setara.
- Migration wajib backward-compatible / mitigasi rollback.
- Prosedur deploy, konfigurasi, smoke test, dan rollback wajib terdokumentasi.

## 13. Poin keputusan untuk PO
- PO hanya saat Sprint Planning & Review; saat running teknis & urutan di tangan SM + AgentSession (§2, §5).
- Format: **Rekomendasi:** <opsi> — dampak, effort, urgensi (prinsip: dampak bisnis terbesar, effort terkecil). **Alternatif:** <opsi> — <dampak/effort/urgensi>. **Alasan:** <…>.
- Blocker unfixable diputuskan di Sprint Review. Keputusan PO pengubah scope dicatat di dokumen sprint beserta opsi ditolak & alasannya.
