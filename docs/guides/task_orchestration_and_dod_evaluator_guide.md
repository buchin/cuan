# Panduan Task Orchestration Engine & Autonomous DoD Evaluator (`pkg/taskfsm` & `pkg/dodeval`)

## Ringkasan Eksekutif & Nilai Bisnis bagi Pemilik Usaha

Untuk memastikan seluruh instruksi dan pekerjaan otomatisasi toko online dieksekusi secara dapat diandalkan (*reliable*), Cuan mengekstraksi logika orkestrasi tugas menjadi paket mikro mandiri (*independent micro-packages*) tanpa dependensi eksternal pihak ketiga (*pure Go standard library*):

1. **`pkg/taskfsm` (State Machine Engine)**: Mengelola siklus hidup status tugas (`planning → approved → running → review → completed`, `canceled`, `blocked`) dengan penjaga transisi ilegal (*guardrails*) yang mencegah status melompat tidak sah.
2. **`pkg/dodeval` (Definition of Done Evaluator)**: Parser checklist markdown dan penganalisis celah (*gap analyzer*) cerdas yang mengevaluasi secara objektif kesesuaian antara kriteria penerimaan instruksi vs bukti laporan verifikasi hasil pekerjaan agen.

Kedua micro-packages ini beroperasi dalam waktu komputasi kilat (<10ms) dan bebas dari ketergantungan eksternal (*zero bloat*), menjamin stabilitas daemon worker `cuan-runner` dan backend Cuan.

---

## 1. Modul State Machine Engine (`pkg/taskfsm`)

### Siklus Status Lifecycle (Agentic Scrum)
```
planning ──[PO Approve]──> approved ──[Claim Attempt]──> running ──[Submit Review]──> review ──[DoD 100%]──> completed
   │                            │                            │                            │
[Cancel]                     [Cancel]                     [Cancel]                     [Auto-Requeue]
   ↓                            ↓                            ↓                            ↓
canceled                     canceled                     canceled                     approved / running
                                │                            │                            │
                             [Block]                      [Block]                      [Block]
                                ↓                            ↓                            ↓
                             blocked                      blocked                      blocked
```

### Guardrails Pencegahan Transisi Ilegal
- **Tidak ada jalan pintas**: Tugas berstatus `planning` dilarang langsung meloncat ke `running` atau `completed` tanpa approval PO.
- **Terminal safety**: Status terminal `completed` dan `canceled` dikunci secara baku, mencegah modifikasi ilegal tanpa otorisasi pembukaan ulang (*reopen*).
- **Audit trail bawaan**: Menyimpan catatan riwayat transisi waktu-nyata lengkap dengan stempel waktu, alasan perubahan status, dan metadata operasional.

### Penggunaan Cepat
```go
import "github.com/buchin/cuan/pkg/taskfsm"

// Inisialisasi FSM
fsm, err := taskfsm.New(taskfsm.StatePlanning)

// Transisi status dengan validasi guardrail
err = fsm.Transition(taskfsm.StateApproved, "Disetujui oleh Product Owner")
err = fsm.Transition(taskfsm.StateRunning, "Diambil oleh cuan-runner")
err = fsm.Transition(taskfsm.StateReview, "Eksekusi selesai, menunggu DoD")
err = fsm.Transition(taskfsm.StateCompleted, "Lolos evaluasi DoD 100%")
```

---

## 2. Modul Definition of Done Evaluator (`pkg/dodeval`)

### Parser Markdown Checklist & Acceptance Criteria
Mendukung ekstraksi otomatis berbagai sintaks kriteria dan checklist markdown:
- `- [ ] Kriteria belum selesai`
- `- [x] Kriteria telah selesai`
- `* [ ] Checklist bertanda bintang`
- `1. [ ] Checklist bernomor`
- Deteksi seksi eksplisit: `## Acceptance Criteria`, `### Definition of Done`, `### DoD Checklist`

### Gap Analyzer Cerdas
Mengevaluasi bukti verifikasi (ringkasan, catatan selesai, hasil uji otomatis, artefak) terhadap seluruh kriteria:
- Mendeteksi kriteria yang belum terpenuhi (*unmet criteria*).
- Menghitung tingkat kepatuhan (*compliance rate*) secara persentase (0% - 100%).
- Mendeteksi kontradiksi atau sinyal kegagalan eksplisit dalam laporan agen.

### Penggunaan Cepat
```go
import "github.com/buchin/cuan/pkg/dodeval"

instructions := `
## Acceptance Criteria
- [ ] Phase 1: pkg/taskfsm
- [ ] Phase 2: pkg/dodeval
- [ ] Phase 3: Integrasi & Live Deployment
`

evidence := dodeval.Evidence{
    Summary: "Ekstraksi taskfsm, dodeval, dan integrasi runner selesai",
    CompleteNote: "1. Phase 1 lulus unit test (<10ms)\n2. Phase 2 lulus unit test\n3. Phase 3 live deployment berhasil",
}

res := dodeval.Evaluate(instructions, evidence)
if res.Passed {
    fmt.Printf("DoD Lulus 100%%: %s\n", res.Details)
} else {
    fmt.Printf("DoD Perlu Revisi (%d celah ditemukan)\n", res.UnmetCriteria)
}
```

---

## 3. Integrasi pada Cuan Runner Daemon

Pada daemon `cuan-runner` dan `RunnerService` (`internal/runner`):
1. **`EvaluateDoD`**: Secara otomatis mengurai instruksi tugas menggunakan `dodeval.ParseAcceptanceCriteria`, lalu memvalidasi kelengkapan bukti penyelesaian agen sebelum memutuskan `mark_done` atau `auto_requeue`.
2. **`CompleteAttempt`**: Menegakkan validasi transisi status tugas via `taskfsm.FromCuanTaskStatus` dan `taskfsm.CanTransition`, menjamin integritas alur kerja Agentic Scrum tanpa lonjakan status liar.
