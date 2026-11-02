# Panduan Integrasi Task Orchestration & DoD Evaluator (`pkg/taskfsm` & `pkg/dodeval`)

## Ringkasan Eksekutif
Dalam rangka mendukung performa yang ramping, modular, dan kilat (<10 detik) sesuai kesepakatan Agentic Scrum, modul orkestrasi tugas dan evaluasi Definition of Done (DoD) dipisahkan menjadi dua micro-packages mandiri (*zero external dependencies*):
1. **`pkg/taskfsm`**: State Machine Engine yang mengelola siklus hidup status tugas (`planning → approved → running → review → completed`, `blocked`, `canceled`) dengan guardrails pencegahan lonjakan ilegal.
2. **`pkg/dodeval`**: Definition of Done Evaluator yang mem-parsing kriteria markdown secara otomatis dan membandingkannya terhadap korpus bukti laporan verifikasi.

## 1. Modul `pkg/taskfsm`
### Siklus Status (Lifecycle)
Siklus transisi status tugas terikat pada alur Scrum:
- `planning` (backlog): Inisiasi tugas dan perumusan backlog.
- `approved` (queued): Disetujui oleh PO atau koordinator tugas.
- `running` (doing): Dieksekusi aktif oleh agent runner daemon.
- `review`: Evaluasi hasil kerja terhadap kriteria DoD.
- `completed` (done): Selesai dan terverifikasi secara objektif.
- `blocked`: Ditangguhkan karena adanya kendala konkret.
- `canceled` (failed): Dibatalkan atau dihentikan.

### Guardrail Transisi
Transisi ilegal (seperti melompat langsung dari `planning` ke `running` atau `completed` tanpa persetujuan) ditolak secara otomatis oleh fungsi `CanTransition` dan `ValidateTransition`.

## 2. Modul `pkg/dodeval`
### Parsing Kriteria Markdown
Mendukung checklist bertingkat:
- `- [ ]` / `- [x]`
- `1. [ ]` / `1. [x]`
- `1. Teks Kriteria` (numbered list)

### Analisis Kesenjangan Bukti (Gap Analysis)
Fungsi `AnalyzeGaps` dan `Evaluate` memvalidasi ketersediaan bukti pada ringkasan (`summary`), catatan lengkap (`complete_note`), dan status verifikasi teknis (`clean_working_tree`, `targeted_tests_pass`, `build_zero_error`). Jika seluruh kriteria terpenuhi, evaluasi menghasilkan rekomendasi `mark_done`. Jika terdapat kesenjangan, sistem menghasilkan rekomendasi `auto_requeue` terarah.

## 3. Integrasi ke Cuan Runner (`internal/runner`)
- `internal/runner/service.go`: Memvalidasi setiap transisi status tugas menggunakan `ValidateTaskTransition` dan mencegah lonjakan ilegal ke terminal state.
- `internal/runner/dod_evaluator.go`: Menjalankan evaluasi checklist acceptance criteria otonom sebelum menandai tugas selesai.
